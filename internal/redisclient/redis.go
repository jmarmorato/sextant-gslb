// internal/redisclient/redis.go
package redisclient

import (
	"context"
	"strconv"

	"gslb/internal/models"

	"github.com/redis/go-redis/v9"
)

// New creates and returns a configured Redis client using values from the provided config.
func New(ctx context.Context, cfg models.Configuration) (*redis.Client, error) {
	db, err := strconv.Atoi(cfg.Sextant.Redis.Database)
	if err != nil {
		return nil, err
	}

	protocol, err := strconv.Atoi(cfg.Sextant.Redis.Protocol)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Sextant.Redis.Host + ":" + cfg.Sextant.Redis.Port,
		Password: cfg.Sextant.Redis.Password,
		DB:       db,
		Protocol: protocol,
	})

	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return client, nil
}
