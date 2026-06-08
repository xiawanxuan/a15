package lock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"astro-scheduler/pkg/utils"
)

type MemoryLock struct {
	mu     sync.Mutex
	locks  map[string]*memoryLockEntry
	prefix string
}

type memoryLockEntry struct {
	holder    string
	expiresAt time.Time
}

func NewMemoryLock(prefix string) *MemoryLock {
	if prefix == "" {
		prefix = "astro:"
	}
	return &MemoryLock{
		locks:  make(map[string]*memoryLockEntry),
		prefix: prefix,
	}
}

func (m *MemoryLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fullKey := m.prefix + key
	holder := utils.GenerateID()

	entry, exists := m.locks[fullKey]
	if exists && time.Now().Before(entry.expiresAt) {
		return false, nil
	}

	m.locks[fullKey] = &memoryLockEntry{
		holder:    holder,
		expiresAt: time.Now().Add(ttl),
	}

	utils.Sugar.Debugf("Memory lock acquired: %s", fullKey)
	return true, nil
}

func (m *MemoryLock) Lock(ctx context.Context, key string, ttl time.Duration) error {
	for {
		ok, err := m.TryLock(ctx, key, ttl)
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

func (m *MemoryLock) Unlock(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fullKey := m.prefix + key
	delete(m.locks, fullKey)

	utils.Sugar.Debugf("Memory lock released: %s", fullKey)
	return nil
}

func (m *MemoryLock) Extend(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fullKey := m.prefix + key
	entry, exists := m.locks[fullKey]
	if !exists || time.Now().After(entry.expiresAt) {
		return ErrLockNotHeld
	}

	entry.expiresAt = time.Now().Add(ttl)
	return nil
}

func (m *MemoryLock) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.locks = make(map[string]*memoryLockEntry)
	return nil
}

func NewDistributedLock(cfg LockConfig) (DistributedLock, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryLock(cfg.Prefix), nil
	case "redis":
		return NewRedisLock(cfg)
	default:
		return nil, fmt.Errorf("unsupported lock type: %s", cfg.Type)
	}
}
