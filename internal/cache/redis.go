package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/config"
)

// New builds the application cache. When no Redis address is configured a
// Noop cache is returned and the server keeps working.
func New(cfg *config.Config, logger *zerolog.Logger) Cache {
	if cfg.Cache.Redis.Addr == "" {
		logger.Info().Msg("redis cache disabled")
		return Noop{}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Cache.Redis.Addr,
		Password: cfg.Cache.Redis.Password,
		DB:       cfg.Cache.Redis.DB,
	})

	logger.Info().Str("addr", cfg.Cache.Redis.Addr).Msg("redis cache enabled")
	return &redisCache{client: client}
}

type redisCache struct {
	client *redis.Client
}

func (c *redisCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get %q: %w", key, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("cache decode %q: %w", key, err)
	}
	return true, nil
}

func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache encode %q: %w", key, err)
	}
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set %q: %w", key, err)
	}
	return nil
}

func (c *redisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache delete %q: %w", keys, err)
	}
	return nil
}
