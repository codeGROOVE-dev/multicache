package bdcache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCache_MemoryOnly(t *testing.T) {
	ctx := context.Background()
	cache, err := New[string, int](ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Test Set and Get
	if err := cache.Set(ctx, "key1", 42, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, found, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("key1 not found")
	}
	if val != 42 {
		t.Errorf("Get value = %d; want 42", val)
	}

	// Test miss
	_, found, err = cache.Get(ctx, "missing")
	if err != nil {
		t.Fatalf("Get missing: %v", err)
	}
	if found {
		t.Error("missing key should not be found")
	}

	// Test delete
	cache.Delete(ctx, "key1")

	_, found, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if found {
		t.Error("deleted key should not be found")
	}
}

func TestCache_WithTTL(t *testing.T) {
	ctx := context.Background()
	cache, err := New[string, string](ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Set with short TTL
	if err := cache.Set(ctx, "temp", "value", 50*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Should be available immediately
	val, found, _ := cache.Get(ctx, "temp")
	if !found || val != "value" {
		t.Error("temp should be found immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, found, _ = cache.Get(ctx, "temp")
	if found {
		t.Error("temp should be expired")
	}
}

func TestCache_DefaultTTL(t *testing.T) {
	ctx := context.Background()
	cache, err := New[string, int](ctx, WithDefaultTTL(50*time.Millisecond))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Set without explicit TTL (ttl=0 uses default)
	if err := cache.Set(ctx, "key", 100, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Should be available immediately
	_, found, _ := cache.Get(ctx, "key")
	if !found {
		t.Error("key should be found immediately")
	}

	// Wait for default TTL expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, found, _ = cache.Get(ctx, "key")
	if found {
		t.Error("key should be expired after default TTL")
	}
}

func TestCache_Cleanup(t *testing.T) {
	ctx := context.Background()
	cache, err := New[string, int](ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Add expired and valid entries
	if err := cache.Set(ctx, "expired1", 1, 1*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cache.Set(ctx, "expired2", 2, 1*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cache.Set(ctx, "valid", 3, 1*time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Run cleanup
	removed := cache.Cleanup()
	if removed != 2 {
		t.Errorf("Cleanup removed %d items; want 2", removed)
	}

	// Valid entry should still exist
	_, found, _ := cache.Get(ctx, "valid")
	if !found {
		t.Error("valid entry should still exist")
	}
}

func TestCache_Concurrent(t *testing.T) {
	ctx := context.Background()
	cache, err := New[int, int](ctx, WithMemorySize(1000))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	var wg sync.WaitGroup

	// Concurrent writers
	for i := range 10 {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := range 100 {
				if err := cache.Set(ctx, offset*100+j, j, 0); err != nil {
					t.Errorf("Set: %v", err)
				}
			}
		}(i)
	}

	// Concurrent readers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 100 {
				cache.Get(ctx, j)
			}
		}()
	}

	wg.Wait()

	// Cache should be at or near capacity
	if cache.Len() > 1000 {
		t.Errorf("cache length = %d; should not exceed capacity", cache.Len())
	}
}

func TestCache_WithFilePersistence(t *testing.T) {
	ctx := context.Background()
	cacheID := "test-file-cache-" + time.Now().Format("20060102150405")

	cache, err := New[string, string](ctx, WithLocalStore(cacheID))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Store values
	if err := cache.Set(ctx, "key1", "value1", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cache.Set(ctx, "key2", "value2", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	cache.Close()

	// Create new cache instance - should load from files
	cache2, err := New[string, string](ctx, WithLocalStore(cacheID))
	if err != nil {
		t.Fatalf("New cache2: %v", err)
	}
	defer cache2.Close()

	// Give warmup time to complete
	time.Sleep(100 * time.Millisecond)

	// Values should be available
	val, found, err := cache2.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || val != "value1" {
		t.Errorf("Get key1 = %v, %v; want value1, true", val, found)
	}
}

func TestCache_Len(t *testing.T) {
	ctx := context.Background()
	cache, err := New[string, int](ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	if cache.Len() != 0 {
		t.Errorf("initial length = %d; want 0", cache.Len())
	}

	if err := cache.Set(ctx, "a", 1, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cache.Set(ctx, "b", 2, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cache.Set(ctx, "c", 3, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if cache.Len() != 3 {
		t.Errorf("length = %d; want 3", cache.Len())
	}

	cache.Delete(ctx, "b")

	if cache.Len() != 2 {
		t.Errorf("length after delete = %d; want 2", cache.Len())
	}
}

func BenchmarkCache_Set(b *testing.B) {
	ctx := context.Background()
	cache, _ := New[int, int](ctx)
	defer cache.Close()

	b.ResetTimer()
	for i := range b.N {
		if err := cache.Set(ctx, i%10000, i, 0); err != nil {
			b.Fatalf("Set: %v", err)
		}
	}
}

func BenchmarkCache_Get_Hit(b *testing.B) {
	ctx := context.Background()
	cache, _ := New[int, int](ctx)
	defer cache.Close()

	// Populate cache
	for i := range 10000 {
		if err := cache.Set(ctx, i, i, 0); err != nil {
			b.Fatalf("Set: %v", err)
		}
	}

	b.ResetTimer()
	for i := range b.N {
		cache.Get(ctx, i%10000)
	}
}

func BenchmarkCache_Get_Miss(b *testing.B) {
	ctx := context.Background()
	cache, _ := New[int, int](ctx)
	defer cache.Close()

	b.ResetTimer()
	for i := range b.N {
		cache.Get(ctx, i)
	}
}

func BenchmarkCache_Mixed(b *testing.B) {
	ctx := context.Background()
	cache, _ := New[int, int](ctx)
	defer cache.Close()

	b.ResetTimer()
	for i := range b.N {
		if i%3 == 0 {
			if err := cache.Set(ctx, i%10000, i, 0); err != nil {
				b.Fatalf("Set: %v", err)
			}
		} else {
			cache.Get(ctx, i%10000)
		}
	}
}

func TestCache_Close_PersistenceError(t *testing.T) {
	cache := &Cache[string, int]{
		memory:  newS3FIFO[string, int](100),
		persist: &closeErrorPersist[string, int]{},
		opts:    &Options{MemorySize: 100},
	}

	// Close should return the persistence error
	if err := cache.Close(); err == nil {
		t.Error("Close should return error when persistence.Close() fails")
	}
}

func TestCache_Close_WithNilPersist(t *testing.T) {
	ctx := context.Background()

	cache, err := New[string, int](ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Close with nil persistence should not error
	if err := cache.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestCache_Delete_PersistenceError(t *testing.T) {
	ctx := context.Background()

	cache := &Cache[string, int]{
		memory:  newS3FIFO[string, int](100),
		persist: &errorPersist[string, int]{},
		opts:    &Options{MemorySize: 100},
	}
	defer cache.Close()

	// Set directly in memory (bypass persistence which would fail)
	cache.memory.set("key1", 42, time.Time{})

	// Delete should handle persistence error gracefully
	cache.Delete(ctx, "key1")

	// Should be deleted from memory
	_, found, _ := cache.Get(ctx, "key1")
	if found {
		t.Error("key1 should be deleted from memory")
	}
}

func TestCache_Get_PersistenceError(t *testing.T) {
	ctx := context.Background()

	// Create cache with mock that returns errors
	cache := &Cache[string, int]{
		memory:  newS3FIFO[string, int](100),
		persist: &errorPersist[string, int]{},
		opts:    &Options{MemorySize: 100},
	}
	defer cache.Close()

	// Get should handle persistence error gracefully
	_, found, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get should not return error on persistence failure: %v", err)
	}
	if found {
		t.Error("key1 should not be found")
	}
}

func TestCache_New_DatastorePath(t *testing.T) {
	ctx := context.Background()

	// This will fail to connect but exercises the datastore path in New()
	cache, err := New[string, int](ctx,
		WithCloudDatastore("test-datastore-project"),
		WithMemorySize(50),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Should work in memory even if datastore failed
	if err := cache.Set(ctx, "key1", 42, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, found, _ := cache.Get(ctx, "key1")
	if !found || val != 42 {
		t.Errorf("Get = %v, %v; want 42, true", val, found)
	}
}

func TestCache_New_DefaultOptions(t *testing.T) {
	ctx := context.Background()

	cache, err := New[string, int](ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	if cache.opts.MemorySize != 10000 {
		t.Errorf("default memory size = %d; want 10000", cache.opts.MemorySize)
	}

	if cache.opts.DefaultTTL != 0 {
		t.Errorf("default TTL = %v; want 0", cache.opts.DefaultTTL)
	}

	if cache.persist != nil {
		t.Error("persist should be nil with default options")
	}
}

func TestCache_New_FilePersistenceSuccess(t *testing.T) {
	ctx := context.Background()
	cacheID := "test-success-" + t.Name()

	cache, err := New[string, int](ctx, WithLocalStore(cacheID))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	if cache.persist == nil {
		t.Error("persist should not be nil with valid local store")
	}

	// Test on-demand loading from disk
	if err := cache.Set(ctx, "warmup-test", 42, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	cache.Close()

	// Create new cache - should load on-demand from disk (no warmup)
	cache2, err := New[string, int](ctx, WithLocalStore(cacheID))
	if err != nil {
		t.Fatalf("New cache2: %v", err)
	}
	defer cache2.Close()

	// Should load on-demand from disk
	val, found, _ := cache2.Get(ctx, "warmup-test")
	if !found || val != 42 {
		t.Errorf("Get on-demand = %v, %v; want 42, true", val, found)
	}
}

func TestCache_New_WithDatastoreOption(t *testing.T) {
	ctx := context.Background()

	// This will fail to connect but should gracefully degrade
	cache, err := New[string, int](ctx, WithCloudDatastore("invalid-project"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Should work in memory-only mode after persistence fails
	if err := cache.Set(ctx, "key1", 42, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, found, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || val != 42 {
		t.Errorf("Get = %v, %v; want 42, true", val, found)
	}
}

func TestCache_SetDefaultWithExplicitTTL(t *testing.T) {
	ctx := context.Background()

	cache, err := New[string, int](ctx, WithDefaultTTL(1*time.Hour))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Set with ttl=0 should use the default TTL
	if err := cache.Set(ctx, "key1", 42, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify it's set
	val, found, _ := cache.Get(ctx, "key1")
	if !found || val != 42 {
		t.Errorf("Get = %v, %v; want 42, true", val, found)
	}
}

func TestCache_SetExplicitTTLOverridesDefault(t *testing.T) {
	ctx := context.Background()

	cache, err := New[string, int](ctx, WithDefaultTTL(1*time.Hour))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cache.Close()

	// Set with explicit short TTL (overrides default)
	if err := cache.Set(ctx, "key1", 42, 50*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Should exist immediately
	_, found, _ := cache.Get(ctx, "key1")
	if !found {
		t.Error("key1 should exist immediately")
	}

	// Wait for explicit TTL to expire (not default)
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, found, _ = cache.Get(ctx, "key1")
	if found {
		t.Error("key1 should be expired after explicit TTL")
	}
}

func TestCache_Set_WithPersistenceStoreError(t *testing.T) {
	ctx := context.Background()

	cache := &Cache[string, int]{
		memory:  newS3FIFO[string, int](100),
		persist: &errorPersist[string, int]{},
		opts:    &Options{MemorySize: 100, DefaultTTL: 0},
	}
	defer cache.Close()

	// Set should return error when persistence fails
	err := cache.Set(ctx, "key1", 42, 0)
	if err == nil {
		t.Fatal("Set should return error when persistence fails")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Set error = %v; want context.DeadlineExceeded", err)
	}

	// BUT value should still be in memory (reliability guarantee)
	val, found, _ := cache.Get(ctx, "key1")
	if !found || val != 42 {
		t.Errorf("Get = %v, %v; want 42, true (value should be cached despite persistence failure)", val, found)
	}
}

// closeErrorPersist is a mock that fails on Close.
type closeErrorPersist[K comparable, V any] struct {
	errorPersist[K, V]
}

func (c *closeErrorPersist[K, V]) Close() error {
	return errors.New("close failed")
}

// errorPersist is a mock persistence layer that always returns errors.
type errorPersist[K comparable, V any] struct{}

func (e *errorPersist[K, V]) Close() error {
	return nil
}

func (e *errorPersist[K, V]) Delete(ctx context.Context, key K) error {
	return context.DeadlineExceeded
}

func (e *errorPersist[K, V]) Load(ctx context.Context, key K) (V, time.Time, bool, error) {
	var zero V
	return zero, time.Time{}, false, context.DeadlineExceeded
}

func (e *errorPersist[K, V]) LoadAll(ctx context.Context) (<-chan Entry[K, V], <-chan error) {
	return e.LoadRecent(ctx, 0)
}

func (e *errorPersist[K, V]) LoadRecent(ctx context.Context, limit int) (<-chan Entry[K, V], <-chan error) {
	entryCh := make(chan Entry[K, V])
	errCh := make(chan error, 1)
	close(entryCh)
	errCh <- context.DeadlineExceeded
	return entryCh, errCh
}

func (e *errorPersist[K, V]) Store(ctx context.Context, key K, value V, expiry time.Time) error {
	return context.DeadlineExceeded
}

func (e *errorPersist[K, V]) ValidateKey(key K) error {
	return nil // Allow all keys for test
}
