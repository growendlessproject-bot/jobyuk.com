package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a new Redis connection.
func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	opts.PoolSize = 10
	opts.MinIdleConns = 3
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	// Verify connection with brief retry for Windows socket readiness
	var pingErr error
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pingErr = client.Ping(ctx).Err()
		cancel()
		if pingErr == nil {
			return client, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("failed to connect to Redis: %w", pingErr)
}

// CacheService provides Redis caching operations.
type CacheService struct {
	client *redis.Client
}

// NewCacheService creates a new cache service.
func NewCacheService(client *redis.Client) *CacheService {
	return &CacheService{client: client}
}

// Get retrieves a cached value.
func (c *CacheService) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(val, dest)
}

// Set stores a value in cache with TTL.
func (c *CacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a cached value.
func (c *CacheService) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// ClearAll flushes all cached data.
func (c *CacheService) ClearAll(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

// ClearPattern deletes all keys matching a pattern.
func (c *CacheService) ClearPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, newCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			c.client.Del(ctx, keys...)
		}
		cursor = newCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// ClearSearchCache clears all cached search results from Reed API and local search.
func (c *CacheService) ClearSearchCache(ctx context.Context) error {
	_ = c.ClearPattern(ctx, "reed:search:*")
	return c.ClearPattern(ctx, "jobs:search:*")
}

