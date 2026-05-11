package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisService provides Redis connection and operations
type RedisService struct {
	client *redis.Client
	mu     sync.RWMutex
}

var (
	redisInstance *RedisService
	redisOnce     sync.Once
)

// NewRedisService creates a new Redis service instance
func NewRedisService(redisURL string) (*RedisService, error) {
	var initErr error

	redisOnce.Do(func() {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			initErr = fmt.Errorf("failed to parse Redis URL: %w", err)
			return
		}

		// Configure connection pool
		opts.PoolSize = 10
		opts.MinIdleConns = 2
		opts.MaxRetries = 3
		opts.DialTimeout = 5 * time.Second
		opts.ReadTimeout = 3 * time.Second
		opts.WriteTimeout = 3 * time.Second

		client := redis.NewClient(opts)

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("failed to connect to Redis: %w", err)
			return
		}

		redisInstance = &RedisService{
			client: client,
		}

		log.Println("✅ Redis connection established")
	})

	if initErr != nil {
		return nil, initErr
	}

	return redisInstance, nil
}

// GetRedisService returns the singleton Redis service instance
func GetRedisService() *RedisService {
	return redisInstance
}

// Client returns the underlying Redis client
func (r *RedisService) Client() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

// Close closes the Redis connection
func (r *RedisService) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Ping checks if Redis is healthy
func (r *RedisService) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Set sets a key-value pair with optional expiration
func (r *RedisService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key
func (r *RedisService) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Delete removes a key
func (r *RedisService) Delete(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// SetNX sets a key only if it doesn't exist (for distributed locking)
func (r *RedisService) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

// Publish publishes a message to a channel
func (r *RedisService) Publish(ctx context.Context, channel string, message interface{}) error {
	return r.client.Publish(ctx, channel, message).Err()
}

// Subscribe subscribes to one or more channels
func (r *RedisService) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return r.client.Subscribe(ctx, channels...)
}

// Incr increments a counter (for rate limiting)
func (r *RedisService) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Expire sets expiration on a key
func (r *RedisService) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL gets the remaining time to live for a key
func (r *RedisService) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// AcquireLock attempts to acquire a distributed lock
// Returns true if lock was acquired, false otherwise
func (r *RedisService) AcquireLock(ctx context.Context, lockKey string, lockValue string, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, lockKey, lockValue, expiration).Result()
}

// ReleaseLock releases a distributed lock if it's still held by the given value
func (r *RedisService) ReleaseLock(ctx context.Context, lockKey string, lockValue string) (bool, error) {
	// Lua script to atomically check and delete
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, r.client, []string{lockKey}, lockValue).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

// SAdd adds one or more members to a Redis SET
func (r *RedisService) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return r.client.SAdd(ctx, key, members...).Result()
}

// SMembers returns all members of a Redis SET
func (r *RedisService) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// Exists checks if a key exists in Redis
func (r *RedisService) Exists(ctx context.Context, key string) (int64, error) {
	return r.client.Exists(ctx, key).Result()
}

// CheckRateLimit checks if a rate limit has been exceeded
// Returns remaining requests and whether the limit was exceeded
func (r *RedisService) CheckRateLimit(ctx context.Context, key string, limit int64, window time.Duration) (remaining int64, exceeded bool, err error) {
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, false, err
	}

	// Set expiry on first request
	if count == 1 {
		r.client.Expire(ctx, key, window)
	}

	remaining = limit - count
	if remaining < 0 {
		remaining = 0
	}

	return remaining, count > limit, nil
}
