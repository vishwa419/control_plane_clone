package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with our schema operations
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client
func NewClient(host string, port string, password string) (*Client, error) {
	addr := fmt.Sprintf("%s:%s", host, port)
	
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// GetClient returns the underlying Redis client (for advanced operations)
func (c *Client) GetClient() *redis.Client {
	return c.rdb
}

// AcquireLock acquires a distributed lock using SET NX
// Returns true if lock was acquired, false if already locked
func (c *Client) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	result := c.rdb.SetNX(ctx, key, "locked", ttl)
	if result.Err() != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", result.Err())
	}
	return result.Val(), nil
}

// ReleaseLock releases a distributed lock
func (c *Client) ReleaseLock(ctx context.Context, key string) error {
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}
