// Package multicache provides a high-performance cache with optional persistence.
package multicache

import (
	"sync"
	"time"
)

const numFlightShards = 256

// Cache is an in-memory cache. All operations are synchronous and infallible.
type Cache[K comparable, V any] struct {
	flights    [numFlightShards]flightGroup[K, V]
	memory     *s3fifo[K, V]
	defaultTTL time.Duration
	noExpiry   bool
}

// flightGroup deduplicates concurrent calls for the same key.
//
//nolint:govet // fieldalignment: semantic grouping preferred
type flightGroup[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*flightCall[V]
	pool  sync.Pool
}

//nolint:govet // fieldalignment: semantic grouping preferred
type flightCall[V any] struct {
	wg     sync.WaitGroup
	val    V
	err    error
	shared bool
}

// do executes fn once per key; concurrent callers share the result.
func (g *flightGroup[K, V]) do(key K, fn func() (V, error)) (V, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*flightCall[V])
	}
	if c, ok := g.calls[key]; ok {
		c.shared = true
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	var c *flightCall[V]
	if pooled, ok := g.pool.Get().(*flightCall[V]); ok && pooled != nil {
		c = pooled
	} else {
		c = &flightCall[V]{}
	}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()

	g.mu.Lock()
	delete(g.calls, key)
	shared := c.shared
	g.mu.Unlock()
	c.wg.Done()

	val, err := c.val, c.err

	if !shared {
		var zero V
		c.val = zero
		c.err = nil
		c.shared = false
		g.pool.Put(c)
	}

	return val, err
}

// New creates an in-memory cache.
func New[K comparable, V any](opts ...Option) *Cache[K, V] {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &Cache[K, V]{
		memory:     newS3FIFO[K, V](cfg),
		defaultTTL: cfg.defaultTTL,
		noExpiry:   cfg.defaultTTL == 0,
	}
}

// Get returns the value for key, or zero and false if not found.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.memory.get(key)
}

// Set stores a value. Uses default TTL if none provided.
func (c *Cache[K, V]) Set(key K, value V, ttl ...time.Duration) {
	if c.noExpiry && len(ttl) == 0 {
		c.memory.set(key, value, 0)
		return
	}
	var t time.Duration
	if len(ttl) > 0 {
		t = ttl[0]
	}
	c.memory.set(key, value, timeToNano(c.expiry(t)))
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.memory.del(key)
}

// SetIfAbsent stores value only if key is missing. No deduplication.
func (c *Cache[K, V]) SetIfAbsent(key K, value V, ttl ...time.Duration) (V, bool) {
	if val, ok := c.memory.get(key); ok {
		return val, true
	}
	var t time.Duration
	if len(ttl) > 0 {
		t = ttl[0]
	}
	c.memory.set(key, value, timeToNano(c.expiry(t)))
	return value, false
}

// GetSet returns cached value or calls loader to compute it.
// Concurrent calls for the same key share one loader invocation.
func (c *Cache[K, V]) GetSet(key K, loader func() (V, error), ttl ...time.Duration) (V, error) {
	if val, ok := c.memory.get(key); ok {
		return val, nil
	}

	idx := flightShard(key)
	return c.flights[idx].do(key, func() (V, error) {
		if val, ok := c.memory.get(key); ok {
			return val, nil
		}

		val, err := loader()
		if err != nil {
			return val, err
		}

		var t time.Duration
		if len(ttl) > 0 {
			t = ttl[0]
		}
		c.memory.set(key, val, timeToNano(c.expiry(t)))

		return val, nil
	})
}

func flightShard[K comparable](key K) int {
	switch k := any(key).(type) {
	case int:
		return k & (numFlightShards - 1)
	case int64:
		return int(k) & (numFlightShards - 1)
	case uint64:
		return int(k) & (numFlightShards - 1) //nolint:gosec // result is always 0-255
	case string:
		return int(hashString(k)) & (numFlightShards - 1) //nolint:gosec // result is always 0-255
	default:
		return 0
	}
}

// Len returns the number of entries.
func (c *Cache[K, V]) Len() int {
	return c.memory.len()
}

// Flush removes all entries. Returns count removed.
func (c *Cache[K, V]) Flush() int {
	return c.memory.flush()
}

// Close is a no-op for Cache (provided for API symmetry with TieredCache).
func (*Cache[K, V]) Close() {}

func (c *Cache[K, V]) expiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

type config struct {
	size       int
	defaultTTL time.Duration
}

func defaultConfig() *config {
	return &config{size: 16384}
}

// Option configures a Cache.
type Option func(*config)

// Size sets maximum entries. Default 16384.
func Size(n int) Option {
	return func(c *config) { c.size = n }
}

// TTL sets default expiration. Default 0 (none).
func TTL(d time.Duration) Option {
	return func(c *config) { c.defaultTTL = d }
}
