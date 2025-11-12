package redis

import (
	"base-server/internal/config"
	"base-server/internal/observability"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with observability
type Client struct {
	client *redis.Client
	logger *observability.Logger
}

// NewClient creates a new Redis client
func NewClient(cfg config.RedisConfig, logger *observability.Logger) (*Client, error) {
	if !cfg.Enabled {
		logger.Info(context.Background(), "Redis is disabled, skipping client initialization")
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info(ctx, "successfully connected to Redis",
		observability.Field{Key: "host", Value: cfg.Host},
		observability.Field{Key: "port", Value: cfg.Port},
		observability.Field{Key: "db", Value: cfg.DB},
	)

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// GetClient returns the underlying Redis client
func (c *Client) GetClient() *redis.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// ZAdd adds a member with score to a sorted set
func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZAdd(ctx, key, members...).Err()
}

// ZRank returns the rank of a member in a sorted set (ascending order)
func (c *Client) ZRank(ctx context.Context, key, member string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRank(ctx, key, member).Result()
}

// ZRevRank returns the rank of a member in a sorted set (descending order)
func (c *Client) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRevRank(ctx, key, member).Result()
}

// ZScore returns the score of a member in a sorted set
func (c *Client) ZScore(ctx context.Context, key, member string) (float64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZScore(ctx, key, member).Result()
}

// ZRange returns members in a sorted set by index range (ascending)
func (c *Client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRange(ctx, key, start, stop).Result()
}

// ZRevRange returns members in a sorted set by index range (descending)
func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRevRange(ctx, key, start, stop).Result()
}

// ZRangeWithScores returns members with scores in a sorted set (ascending)
func (c *Client) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

// ZRevRangeWithScores returns members with scores in a sorted set (descending)
func (c *Client) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRevRangeWithScores(ctx, key, start, stop).Result()
}

// ZCard returns the number of members in a sorted set
func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZCard(ctx, key).Result()
}

// ZRem removes members from a sorted set
func (c *Client) ZRem(ctx context.Context, key string, members ...interface{}) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZRem(ctx, key, members...).Err()
}

// ZIncrBy increments the score of a member in a sorted set
func (c *Client) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("Redis client not initialized")
	}
	return c.client.ZIncrBy(ctx, key, increment, member).Result()
}

// Exists checks if a key exists
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("Redis client not initialized")
	}
	return c.client.Exists(ctx, keys...).Result()
}

// Del deletes keys
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	return c.client.Del(ctx, keys...).Err()
}

// Expire sets an expiration on a key
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	return c.client.Expire(ctx, key, expiration).Err()
}

// IsEnabled returns whether Redis is enabled
func (c *Client) IsEnabled() bool {
	return c != nil && c.client != nil
}
