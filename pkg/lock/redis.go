package lock

import (
	"context"
	"fmt"
	"time"

	"astro-scheduler/pkg/utils"

	"github.com/redis/go-redis/v9"
)

const (
	lockScript = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
			return 1
		else
			return redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2])
		end
	`

	unlockScript = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`

	extendScript = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
)

type RedisLock struct {
	client *redis.Client
	prefix string
	holder string
}

func NewRedisLock(cfg LockConfig) (*RedisLock, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "astro:lock:"
	}

	return &RedisLock{
		client: client,
		prefix: prefix,
		holder: utils.GenerateID(),
	}, nil
}

func (r *RedisLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := r.prefix + key
	ttlMs := ttl.Milliseconds()

	result, err := r.client.Eval(ctx, lockScript, []string{fullKey}, r.holder, ttlMs).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis lock error: %w", err)
	}

	if result == "OK" || result == int64(1) {
		utils.Sugar.Debugf("Redis lock acquired: %s", fullKey)
		return true, nil
	}

	return false, nil
}

func (r *RedisLock) Lock(ctx context.Context, key string, ttl time.Duration) error {
	for {
		ok, err := r.TryLock(ctx, key, ttl)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (r *RedisLock) Unlock(ctx context.Context, key string) error {
	fullKey := r.prefix + key

	result, err := r.client.Eval(ctx, unlockScript, []string{fullKey}, r.holder).Result()
	if err != nil {
		return fmt.Errorf("redis unlock error: %w", err)
	}

	if result.(int64) == 0 {
		utils.Sugar.Warnf("Redis lock not held or expired: %s", fullKey)
		return ErrLockNotHeld
	}

	utils.Sugar.Debugf("Redis lock released: %s", fullKey)
	return nil
}

func (r *RedisLock) Extend(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := r.prefix + key
	ttlMs := ttl.Milliseconds()

	result, err := r.client.Eval(ctx, extendScript, []string{fullKey}, r.holder, ttlMs).Result()
	if err != nil {
		return fmt.Errorf("redis extend lock error: %w", err)
	}

	if result.(int64) == 0 {
		return ErrLockNotHeld
	}

	return nil
}

func (r *RedisLock) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
