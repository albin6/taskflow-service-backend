package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Server   ServerConfig
	CORS     CORSConfig
	Google   GoogleOAuthConfig
	SMTP     SMTPConfig
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret                 string
	AccessTokenExpiration  time.Duration
	RefreshTokenExpiration time.Duration
}

type ServerConfig struct {
	Port        string
	Host        string
	Environment string
	AppURL      string
}

type CORSConfig struct {
	AllowedOrigins []string
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", "postgres://taskflow:taskflow_dev_password@localhost:5432/taskflow?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		JWT: JWTConfig{
			Secret:                 getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			AccessTokenExpiration:  parseDuration(getEnv("JWT_EXPIRATION", "24h"), 24*time.Hour),
			RefreshTokenExpiration: parseDuration(getEnv("REFRESH_TOKEN_EXPIRATION", "168h"), 168*time.Hour),
		},
		Server: ServerConfig{
			Port:        getEnv("SERVER_PORT", "8080"),
			Host:        getEnv("SERVER_HOST", "0.0.0.0"),
			Environment: getEnv("ENVIRONMENT", "development"),
			AppURL:      getEnv("APP_URL", ""),
		},
		CORS: CORSConfig{
			AllowedOrigins: parseStringSlice(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:8080")),
		},
		Google: GoogleOAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnv("SMTP_PORT", ""),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", ""),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(value string, defaultDuration time.Duration) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultDuration
	}
	return duration
}

func parseStringSlice(value string) []string {
	if value == "" {
		return []string{}
	}

	var result []string
	current := ""
	for _, char := range value {
		if char == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}

	return result
}

func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWT.Secret == "" || c.JWT.Secret == "your-secret-key-change-in-production" {
		if c.Server.Environment == "production" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
	}
	return nil
}
