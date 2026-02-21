package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/taskflow/backend/internal/audit"
	"github.com/taskflow/backend/internal/auth"
	"github.com/taskflow/backend/internal/cache"
	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/database"
	"github.com/taskflow/backend/internal/handlers"
	"github.com/taskflow/backend/internal/keepalive"
	"github.com/taskflow/backend/internal/mail"
	"github.com/taskflow/backend/internal/repository"
	"github.com/taskflow/backend/internal/websocket"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connection established")

	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "../migrations"
	}

	if err := database.RunMigrations(db.DB, migrationsDir); err != nil {
		log.Printf("Warning: Failed to run migrations: %v", err)
	}

	if err := database.ConnectRedis(cfg); err != nil {
		log.Printf("Warning: Failed to connect to Redis for Pub/Sub: %v", err)
	} else {
		log.Println("Redis connection established for Pub/Sub")
	}

	var redisClient *cache.RedisClient
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		client, err := cache.NewRedisClient(redisAddr, os.Getenv("REDIS_PASSWORD"))
		if err != nil {
			log.Printf("Warning: Failed to connect to Redis Cache: %v", err)
		} else {
			log.Println("Redis Cache connection established")
			redisClient = client
			defer client.Client.Close()
		}
	}

	userRepo := repository.NewUserRepository(db.DB, redisClient)
	taskRepo := repository.NewTaskRepository(db.DB)
	deadlineRepo := repository.NewDeadlineRepository(db.DB)
	teamRepo := repository.NewTeamRepository(db.DB, redisClient)
	customRoleRepo := repository.NewCustomRoleRepository(db.DB)
	auditLogger := audit.NewLogger(db.DB)
	notifRepo := repository.NewNotificationRepository(db.DB)

	jwtManager := auth.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenExpiration,
		cfg.JWT.RefreshTokenExpiration,
	)

	emailService := mail.NewEmailService(cfg.SMTP)

	authHandler := handlers.NewAuthHandler(userRepo, jwtManager, emailService)
	userHandler := handlers.NewUserHandler(userRepo, teamRepo, auditLogger, notifRepo, emailService)
	taskHandler := handlers.NewTaskHandler(taskRepo, teamRepo, userRepo, auditLogger)
	deadlineHandler := handlers.NewDeadlineHandler(deadlineRepo, taskRepo, auditLogger)
	teamHandler := handlers.NewTeamHandler(teamRepo, userRepo, auditLogger)
	customRoleHandler := handlers.NewCustomRoleHandler(customRoleRepo, taskRepo)
	adminHandler := handlers.NewAdminHandler(userRepo, teamRepo, taskRepo)

	hub := websocket.NewHub()
	go hub.Run()

	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		if err := db.HealthCheck(); err != nil {
			c.JSON(500, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "healthy"})
	})

	if cfg.Server.AppURL != "" {
		keepalive.Start(cfg.Server.AppURL+"/health", 10*time.Minute)
	}

	router.GET("/api/public/teams", teamHandler.ListTeams)

	api := router.Group("/api")
	{
		api.GET("/load-test", taskHandler.RunLoadTest)
		authRoutes := api.Group("/auth")
		{
			authRoutes.POST("/signup", authHandler.SignUp)
			authRoutes.GET("/teams", teamHandler.ListTeams)
			authRoutes.POST("/login", authHandler.Login)
			authRoutes.POST("/refresh", authHandler.RefreshToken)
			authRoutes.POST("/logout", authHandler.Logout)
			authRoutes.POST("/forgot-password", authHandler.ForgotPassword)
			authRoutes.POST("/reset-password", authHandler.ResetPassword)
		}

		protected := api.Group("")
		protected.Use(auth.AuthMiddleware(jwtManager))
		{
			protected.GET("/auth/me", authHandler.GetMe)
			protected.POST("/auth/change-password", authHandler.ChangePassword)

			userRoutes := protected.Group("/users")
			{
				userRoutes.GET("", userHandler.ListUsers)
				userRoutes.GET("/:id", userHandler.GetUser)
				userRoutes.PUT("/:id", userHandler.UpdateProfile)
				userRoutes.DELETE("/:id", auth.AdminOnly(), userHandler.DeleteUser)
			}

			// Admin only routes
			admin := protected.Group("")
			admin.Use(auth.AdminOnly())
			{
				admin.GET("/admin/stats", adminHandler.GetStats)
				admin.POST("/admin/users", userHandler.AdminCreateUser)
				admin.POST("/users/:id/approve", userHandler.ApproveUser)
				admin.POST("/users/:id/promote", userHandler.PromoteToAdmin)
				admin.POST("/users/:id/demote", userHandler.DemoteToUser)
			}

			taskRoutes := protected.Group("/tasks")
			{
				taskRoutes.GET("", taskHandler.ListTasks)
				taskRoutes.GET("/:id", taskHandler.GetTask)
				taskRoutes.POST("", taskHandler.CreateTask)
				taskRoutes.PUT("/:id", taskHandler.UpdateTask)
				taskRoutes.PATCH("/:id/status", taskHandler.UpdateTaskStatus)
				taskRoutes.DELETE("/:id", taskHandler.DeleteTask)
			}

			deadlineRoutes := protected.Group("/deadline-requests")
			{
				deadlineRoutes.GET("", deadlineHandler.ListRequests)
				deadlineRoutes.GET("/:id", deadlineHandler.GetRequest)
				deadlineRoutes.POST("", deadlineHandler.CreateRequest)
				deadlineRoutes.POST("/:id/approve", auth.AdminOnly(), deadlineHandler.ApproveRequest)
				deadlineRoutes.POST("/:id/reject", auth.AdminOnly(), deadlineHandler.RejectRequest)
			}

			teamRoutes := protected.Group("/teams")
			{
				teamRoutes.GET("", teamHandler.ListTeams)
				teamRoutes.GET("/my-teams", teamHandler.GetUserTeams)
				teamRoutes.GET("/:id", teamHandler.GetTeam)
				teamRoutes.POST("", auth.AdminOnly(), teamHandler.CreateTeam)
				teamRoutes.PUT("/:id", teamHandler.UpdateTeam)
				teamRoutes.DELETE("/:id", auth.AdminOnly(), teamHandler.DeleteTeam)
				teamRoutes.POST("/:id/members", teamHandler.AddMember)
				teamRoutes.DELETE("/:id/members/:userId", teamHandler.RemoveMember)
				teamRoutes.PUT("/:id/members/:userId/role", teamHandler.UpdateMemberRole)
				teamRoutes.GET("/:id/roles", teamHandler.GetTeamRoles)
				teamRoutes.POST("/:id/roles", teamHandler.CreateRole)
				teamRoutes.GET("/:id/assignable-users", teamHandler.GetAssignableUsers)

				teamRoutes.GET("/:id/custom-roles", customRoleHandler.GetTeamRoles)
				teamRoutes.POST("/:id/custom-roles", customRoleHandler.CreateCustomRole)
				teamRoutes.PUT("/:id/custom-roles/:roleId", customRoleHandler.UpdateCustomRole)
				teamRoutes.DELETE("/:id/custom-roles/:roleId", customRoleHandler.DeleteCustomRole)
				teamRoutes.POST("/:id/members/:userId/custom-role", customRoleHandler.AssignRoleToMember)
				teamRoutes.PUT("/:id/members/:userId/position", customRoleHandler.UpdateMemberPosition)
			}

			protected.GET("/ws", func(c *gin.Context) {
				websocket.ServeWs(hub, c, jwtManager)
			})
		}
	}

	address := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("Starting server on %s", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
