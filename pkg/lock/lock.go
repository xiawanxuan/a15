package lock

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLockAcquired = errors.New("lock already acquired")
	ErrLockNotHeld  = errors.New("lock not held")
	ErrLockFailed   = errors.New("failed to acquire lock")
)

type DistributedLock interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Lock(ctx context.Context, key string, ttl time.Duration) error
	Unlock(ctx context.Context, key string) error
	Extend(ctx context.Context, key string, ttl time.Duration) error
	Close() error
}

type LockConfig struct {
	Type     string
	Address  string
	Password string
	DB       int
	Prefix   string
}
