package multicache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const asyncTimeout = 5 * time.Second

// TieredCache combines an in-memory cache with persistent storage.
type TieredCache[K comparable, V any] struct {
	Store      Store[K, V] // direct access to persistence layer
	flights    [numFlightShards]flightGroup[K, V]
	memory     *s3fifo[K, V]
	defaultTTL time.Duration
}

// NewTiered creates a cache backed by the given store.
func NewTiered[K comparable, V any](store Store[K, V], opts ...Option) (*TieredCache[K, V], error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if store == nil {
		return nil, errors.New("store cannot be nil")
	}

	cache := &TieredCache[K, V]{
		Store:      store,
		memory:     newS3FIFO[K, V](cfg),
		defaultTTL: cfg.defaultTTL,
	}

	return cache, nil
}

// Get checks memory, then persistence. Found values are cached in memory.
//
//nolint:gocritic // unnamedResult: public API signature is intentionally clear
func (c *TieredCache[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	if val, ok := c.memory.get(key); ok {
		return val, true, nil
	}

	var zero V
	if err := c.Store.ValidateKey(key); err != nil {
		return zero, false, fmt.Errorf("invalid key: %w", err)
	}

	val, expiry, found, err := c.Store.Get(ctx, key)
	if err != nil {
		return zero, false, fmt.Errorf("persistence load: %w", err)
	}
	if !found {
		return zero, false, nil
	}

	c.memory.set(key, val, timeToNano(expiry))
	return val, true, nil
}

func (c *TieredCache[K, V]) expiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

// Set stores to memory first (always), then persistence.
func (c *TieredCache[K, V]) Set(ctx context.Context, key K, value V, ttl ...time.Duration) error {
	var t time.Duration
	if len(ttl) > 0 {
		t = ttl[0]
	}
	expiry := c.expiry(t)

	if err := c.Store.ValidateKey(key); err != nil {
		return err
	}

	c.memory.set(key, value, timeToNano(expiry))

	if err := c.Store.Set(ctx, key, value, expiry); err != nil {
		return fmt.Errorf("persistence store failed: %w", err)
	}
	return nil
}

// SetAsync stores to memory synchronously, persistence asynchronously.
// Persistence errors are logged, not returned.
func (c *TieredCache[K, V]) SetAsync(ctx context.Context, key K, value V, ttl ...time.Duration) error {
	var t time.Duration
	if len(ttl) > 0 {
		t = ttl[0]
	}
	expiry := c.expiry(t)

	if err := c.Store.ValidateKey(key); err != nil {
		return err
	}

	c.memory.set(key, value, timeToNano(expiry))

	go func() {
		storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), asyncTimeout)
		defer cancel()
		if err := c.Store.Set(storeCtx, key, value, expiry); err != nil {
			slog.Error("async persistence failed", "key", key, "error", err)
		}
	}()

	return nil
}

// GetSet returns cached value or calls loader. Concurrent calls share one loader.
func (c *TieredCache[K, V]) GetSet(ctx context.Context, key K, loader func(context.Context) (V, error), ttl ...time.Duration) (V, error) {
	var zero V

	if val, ok := c.memory.get(key); ok {
		return val, nil
	}

	if err := c.Store.ValidateKey(key); err != nil {
		return zero, fmt.Errorf("invalid key: %w", err)
	}

	val, expiry, found, err := c.Store.Get(ctx, key)
	if err != nil {
		return zero, fmt.Errorf("persistence load: %w", err)
	}
	if found {
		c.memory.set(key, val, timeToNano(expiry))
		return val, nil
	}

	idx := flightShard(key)
	return c.flights[idx].do(key, func() (V, error) {
		if val, ok := c.memory.get(key); ok {
			return val, nil
		}

		val, expiry, found, err := c.Store.Get(ctx, key)
		if err != nil {
			var zero V
			return zero, fmt.Errorf("persistence load: %w", err)
		}
		if found {
			c.memory.set(key, val, timeToNano(expiry))
			return val, nil
		}

		val, err = loader(ctx)
		if err != nil {
			var zero V
			return zero, err
		}

		var t time.Duration
		if len(ttl) > 0 {
			t = ttl[0]
		}
		exp := c.expiry(t)
		c.memory.set(key, val, timeToNano(exp))

		if err := c.Store.Set(ctx, key, val, exp); err != nil {
			slog.Warn("GetSet persistence failed", "key", key, "error", err)
		}

		return val, nil
	})
}

// Delete removes from memory and persistence.
func (c *TieredCache[K, V]) Delete(ctx context.Context, key K) error {
	c.memory.del(key)

	if err := c.Store.ValidateKey(key); err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}
	if err := c.Store.Delete(ctx, key); err != nil {
		return fmt.Errorf("persistence delete: %w", err)
	}
	return nil
}

// Flush clears memory and persistence. Returns total entries removed.
func (c *TieredCache[K, V]) Flush(ctx context.Context) (int, error) {
	memoryRemoved := c.memory.flush()
	persistRemoved, err := c.Store.Flush(ctx)
	if err != nil {
		return memoryRemoved, fmt.Errorf("persistence flush: %w", err)
	}
	return memoryRemoved + persistRemoved, nil
}

// Len returns the memory cache size. Use Store.Len for persistence count.
func (c *TieredCache[K, V]) Len() int {
	return c.memory.len()
}

// Close releases store resources.
func (c *TieredCache[K, V]) Close() error {
	if err := c.Store.Close(); err != nil {
		return fmt.Errorf("close persistence: %w", err)
	}
	return nil
}
