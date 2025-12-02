package bdcache

import (
	"container/list"
	"hash/maphash"
	"sync"
	"sync/atomic"
	"time"
)

const (
	numShards = 32
	shardMask = numShards - 1 // For fast modulo via bitwise AND
)

// s3fifo implements the S3-FIFO eviction algorithm from SOSP'23 paper
// "FIFO queues are all you need for cache eviction"
//
// This implementation uses 32-way sharding for improved concurrent performance.
// Each shard is an independent S3-FIFO instance with its own queues and lock.
//
// Algorithm per shard:
// - Small queue (S): 10% of shard capacity, for new objects
// - Main queue (M): 90% of shard capacity, for promoted objects
// - Ghost queue (G): Tracks evicted keys (no data)
//
// On cache miss:
//   - If object not in ghost → insert into Small
//   - If object in ghost → insert into Main (was accessed before)
//
// On eviction from Small:
//   - If freq == 0 → evict and add to ghost
//   - If freq > 0 → promote to Main and reset freq to 0
//
// On eviction from Main:
//   - If freq == 0 → evict (don't add to ghost, already there)
//   - If freq > 0 → reinsert to back of Main and decrement freq (lazy promotion)
type s3fifo[K comparable, V any] struct {
	shards [numShards]*shard[K, V]
	seed   maphash.Seed
}

// shard is an independent S3-FIFO cache partition.
// Uses lock-free reads via atomic pointer to items map.
// Writes use mutex + copy-on-write for new keys.
//
//nolint:govet // fieldalignment: padding is intentional to prevent false sharing
type shard[K comparable, V any] struct {
	mu        sync.Mutex                         // Only for writes
	_         [48]byte                           // Padding to cache line boundary
	items     atomic.Pointer[map[K]*entry[K, V]] // Lock-free read access
	small     *list.List
	main      *list.List
	ghost     *list.List
	ghostKeys map[K]*list.Element
	capacity  int
	smallCap  int
	ghostCap  int
}

// entry represents a cached item with metadata.
type entry[K comparable, V any] struct {
	key        K
	value      V
	element    *list.Element
	expiryNano int64       // Unix nanoseconds; 0 means no expiry
	accessed   atomic.Bool // Fast "was accessed" flag for S3-FIFO promotion
	inSmall    bool        // True if in Small queue, false if in Main
}

// newS3FIFO creates a new sharded S3-FIFO cache with the given total capacity.
func newS3FIFO[K comparable, V any](capacity int) *s3fifo[K, V] {
	if capacity <= 0 {
		capacity = 16384 // 2^14, divides evenly by 16 shards
	}

	// Divide capacity across shards (round up to avoid zero-capacity shards)
	shardCap := (capacity + numShards - 1) / numShards
	if shardCap < 1 {
		shardCap = 1
	}

	c := &s3fifo[K, V]{
		seed: maphash.MakeSeed(),
	}

	for i := range numShards {
		c.shards[i] = newShard[K, V](shardCap)
	}

	return c
}

// newShard creates a new S3-FIFO shard with the given capacity.
// Queue sizes per S3-FIFO paper: Small=10%, Main=90%, Ghost=90% (matching Main).
func newShard[K comparable, V any](capacity int) *shard[K, V] {
	smallCap := capacity / 10
	if smallCap < 1 {
		smallCap = 1
	}
	// Ghost tracks evicted keys from Main, so size matches Main (90% of capacity)
	ghostCap := capacity - smallCap
	if ghostCap < 1 {
		ghostCap = 1
	}

	s := &shard[K, V]{
		capacity:  capacity,
		smallCap:  smallCap,
		ghostCap:  ghostCap,
		small:     list.New(),
		main:      list.New(),
		ghost:     list.New(),
		ghostKeys: make(map[K]*list.Element, ghostCap),
	}
	items := make(map[K]*entry[K, V], capacity)
	s.items.Store(&items)
	return s
}

// getShard returns the shard for a given key using type-optimized hashing.
// Uses bitwise AND with shardMask for fast modulo (numShards must be power of 2).
func (c *s3fifo[K, V]) getShard(key K) *shard[K, V] {
	switch k := any(key).(type) {
	case int:
		if k < 0 {
			k = -k
		}
		return c.shards[k&shardMask]
	case int64:
		if k < 0 {
			k = -k
		}
		return c.shards[k&shardMask]
	case uint:
		return c.shards[k&shardMask]
	case uint64:
		return c.shards[k&shardMask]
	case string:
		return c.shards[maphash.String(c.seed, k)&shardMask]
	default:
		// Fallback for other types: convert to string and hash
		//nolint:errcheck,forcetypeassert // Only called for string-convertible types
		return c.shards[maphash.String(c.seed, any(key).(string))&shardMask]
	}
}

// getFromMemory retrieves a value from the in-memory cache.
// On hit, increments frequency counter (used during eviction).
func (c *s3fifo[K, V]) getFromMemory(key K) (V, bool) {
	return c.getShard(key).get(key)
}

func (s *shard[K, V]) get(key K) (V, bool) {
	// Lock-free read via atomic pointer
	items := s.items.Load()
	ent, ok := (*items)[key]
	if !ok {
		var zero V
		return zero, false
	}

	// Check expiration (lazy - actual cleanup happens in background)
	// Single int64 comparison; time.Now().UnixNano() only called if item has expiry
	if ent.expiryNano != 0 && time.Now().UnixNano() > ent.expiryNano {
		var zero V
		return zero, false
	}

	// S3-FIFO: Mark as accessed for lazy promotion.
	// Fast path: skip atomic store if already marked (avoids cache line invalidation).
	if !ent.accessed.Load() {
		ent.accessed.Store(true)
	}

	return ent.value, true
}

// timeToNano converts time.Time to Unix nanoseconds; zero time becomes 0.
func timeToNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// setToMemory adds or updates a value in the in-memory cache.
func (c *s3fifo[K, V]) setToMemory(key K, value V, expiry time.Time) {
	c.getShard(key).set(key, value, timeToNano(expiry))
}

func (s *shard[K, V]) set(key K, value V, expiryNano int64) {
	s.mu.Lock()

	oldItems := s.items.Load()

	// Update existing entry (in-place, no map copy needed)
	if ent, ok := (*oldItems)[key]; ok {
		ent.value = value
		ent.expiryNano = expiryNano
		s.mu.Unlock()
		return
	}

	// Check if key is in ghost queue
	inGhost := false
	if ghostElem, ok := s.ghostKeys[key]; ok {
		inGhost = true
		s.ghost.Remove(ghostElem)
		delete(s.ghostKeys, key)
	}

	// Create new entry
	ent := &entry[K, V]{
		key:        key,
		value:      value,
		expiryNano: expiryNano,
		inSmall:    !inGhost,
	}

	// Make room if at capacity (evict modifies map via s.items pointer)
	for len(*s.items.Load()) >= s.capacity {
		s.evict()
	}

	// Add to appropriate queue
	if ent.inSmall {
		ent.element = s.small.PushBack(ent)
	} else {
		ent.element = s.main.PushBack(ent)
	}

	// Copy-on-write for new key
	currentItems := s.items.Load()
	newItems := make(map[K]*entry[K, V], len(*currentItems)+1)
	for k, v := range *currentItems {
		newItems[k] = v
	}
	newItems[key] = ent
	s.items.Store(&newItems)
	s.mu.Unlock()
}

// deleteFromMemory removes a value from the in-memory cache.
func (c *s3fifo[K, V]) deleteFromMemory(key K) {
	c.getShard(key).delete(key)
}

func (s *shard[K, V]) delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := s.items.Load()
	ent, ok := (*items)[key]
	if !ok {
		return
	}

	if ent.inSmall {
		s.small.Remove(ent.element)
	} else {
		s.main.Remove(ent.element)
	}

	s.deleteFromMap(key)
}

// evict removes one item according to S3-FIFO algorithm.
func (s *shard[K, V]) evict() {
	if s.small.Len() > 0 {
		s.evictFromSmall()
		return
	}
	s.evictFromMain()
}

// evictFromSmall evicts an item from the small queue.
func (s *shard[K, V]) evictFromSmall() {
	for s.small.Len() > 0 {
		elem := s.small.Front()
		ent, ok := elem.Value.(*entry[K, V])
		if !ok {
			s.small.Remove(elem)
			continue
		}

		s.small.Remove(elem)

		// Check if accessed since last eviction attempt
		if !ent.accessed.Swap(false) {
			// Not accessed - evict and track in ghost
			s.deleteFromMap(ent.key)
			s.addToGhost(ent.key)
			return
		}

		// Accessed - promote to Main queue
		ent.inSmall = false
		ent.element = s.main.PushBack(ent)
	}
}

// evictFromMain evicts an item from the main queue.
func (s *shard[K, V]) evictFromMain() {
	for s.main.Len() > 0 {
		elem := s.main.Front()
		ent, ok := elem.Value.(*entry[K, V])
		if !ok {
			s.main.Remove(elem)
			continue
		}

		s.main.Remove(elem)

		// Check if accessed since last eviction attempt
		if !ent.accessed.Swap(false) {
			// Not accessed - evict
			s.deleteFromMap(ent.key)
			return
		}

		// Accessed - give second chance (FIFO-Reinsertion)
		ent.element = s.main.PushBack(ent)
	}
}

// deleteFromMap removes a key from the items map using copy-on-write.
// Must be called with s.mu held.
func (s *shard[K, V]) deleteFromMap(key K) {
	oldItems := s.items.Load()
	newItems := make(map[K]*entry[K, V], len(*oldItems))
	for k, v := range *oldItems {
		if k != key {
			newItems[k] = v
		}
	}
	s.items.Store(&newItems)
}

// addToGhost adds a key to the ghost queue.
func (s *shard[K, V]) addToGhost(key K) {
	if s.ghost.Len() >= s.ghostCap {
		elem := s.ghost.Front()
		if ghostKey, ok := elem.Value.(K); ok {
			delete(s.ghostKeys, ghostKey)
		}
		s.ghost.Remove(elem)
	}

	elem := s.ghost.PushBack(key)
	s.ghostKeys[key] = elem
}

// memoryLen returns the total number of items across all shards.
func (c *s3fifo[K, V]) memoryLen() int {
	total := 0
	for _, s := range c.shards {
		items := s.items.Load()
		total += len(*items)
	}
	return total
}

// cleanupMemory removes expired entries from all shards.
func (c *s3fifo[K, V]) cleanupMemory() int {
	total := 0
	for _, s := range c.shards {
		total += s.cleanup()
	}
	return total
}

func (s *shard[K, V]) cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := s.items.Load()
	nowNano := time.Now().UnixNano()
	var expired []K

	for key, ent := range *items {
		if ent.expiryNano != 0 && nowNano > ent.expiryNano {
			expired = append(expired, key)
		}
	}

	if len(expired) == 0 {
		return 0
	}

	// Remove from queues
	for _, key := range expired {
		ent := (*items)[key]
		if ent.inSmall {
			s.small.Remove(ent.element)
		} else {
			s.main.Remove(ent.element)
		}
	}

	// Batch copy-on-write for efficiency
	newItems := make(map[K]*entry[K, V], len(*items)-len(expired))
	for k, v := range *items {
		keep := true
		for _, expKey := range expired {
			if k == expKey {
				keep = false
				break
			}
		}
		if keep {
			newItems[k] = v
		}
	}
	s.items.Store(&newItems)

	return len(expired)
}

// flushMemory removes all entries from all shards.
func (c *s3fifo[K, V]) flushMemory() int {
	total := 0
	for _, s := range c.shards {
		total += s.flush()
	}
	return total
}

func (s *shard[K, V]) flush() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := s.items.Load()
	n := len(*items)
	newItems := make(map[K]*entry[K, V], s.capacity)
	s.items.Store(&newItems)
	s.small.Init()
	s.main.Init()
	s.ghost.Init()
	s.ghostKeys = make(map[K]*list.Element, s.ghostCap)
	return n
}

// totalCapacity returns the total capacity across all shards.
func (c *s3fifo[K, V]) totalCapacity() int {
	return c.shards[0].capacity * numShards
}

// queueLens returns total small and main queue lengths across all shards (for testing/debugging).
func (c *s3fifo[K, V]) queueLens() (small, main int) {
	for _, s := range c.shards {
		s.mu.Lock()
		small += s.small.Len()
		main += s.main.Len()
		s.mu.Unlock()
	}
	return small, main
}

// isInSmall returns whether a key is in the small queue (for testing).
func (c *s3fifo[K, V]) isInSmall(key K) bool {
	s := c.getShard(key)
	items := s.items.Load()
	if ent, ok := (*items)[key]; ok {
		return ent.inSmall
	}
	return false
}

// setExpiry sets the expiry time for a key (for testing).
func (c *s3fifo[K, V]) setExpiry(key K, expiry time.Time) {
	s := c.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.items.Load()
	if ent, ok := (*items)[key]; ok {
		ent.expiryNano = timeToNano(expiry)
	}
}
