package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/taskflow/backend/internal/config"
)

var RedisClient *redis.Client

func ConnectRedis(cfg *config.Config) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	return nil
}
