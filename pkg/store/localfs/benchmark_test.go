//nolint:errcheck,thelper,unparam // benchmark code - errors not critical for performance measurement
package localfs

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	benchCacheSize  = 1000
	smallValueSize  = 64
	mediumValueSize = 1024
	largeValueSize  = 4096
)

// TestLocalFSBenchmarkSuite runs the full benchmark suite for localfs.
func TestLocalFSBenchmarkSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping benchmark suite in short mode")
	}

	fmt.Println()
	fmt.Println("localfs benchmark suite")
	fmt.Println()

	// Sequential benchmarks
	fmt.Println("### Sequential Operations (single thread)")
	fmt.Println()
	runSequentialBenchmarks(t)

	// Concurrent benchmarks
	fmt.Println()
	fmt.Println("### Concurrent Operations")
	fmt.Println()
	runConcurrentBenchmarks(t)

	// Value size impact
	fmt.Println()
	fmt.Println("### Value Size Impact")
	fmt.Println()
	runValueSizeBenchmarks(t)
}

func runSequentialBenchmarks(t *testing.T) {
	fmt.Println("| Operation     | ns/op       | B/op     | allocs/op |")
	fmt.Println("|---------------|-------------|----------|-----------|")

	ops := []struct {
		name string
		fn   func(*testing.B)
	}{
		{"Set (cold)", benchSetCold},
		{"Set (warm)", benchSetWarm},
		{"Get (hit)", benchGetHit},
		{"Get (miss)", benchGetMiss},
		{"Delete", benchDelete},
	}

	for _, op := range ops {
		result := testing.Benchmark(op.fn)
		fmt.Printf("| %-13s | %11.0f | %8d | %9d |\n",
			op.name,
			float64(result.NsPerOp()),
			result.AllocedBytesPerOp(),
			result.AllocsPerOp())
	}
}

func runConcurrentBenchmarks(t *testing.T) {
	fmt.Println("| Threads | Read QPS    | Write QPS   | Mixed QPS   |")
	fmt.Println("|---------|-------------|-------------|-------------|")

	for _, threads := range []int{1, 2, 4, 8} {
		readQPS := measureConcurrentRead(threads)
		writeQPS := measureConcurrentWrite(threads)
		mixedQPS := measureConcurrentMixed(threads)
		fmt.Printf("| %7d | %9.0f   | %9.0f   | %9.0f   |\n",
			threads, readQPS, writeQPS, mixedQPS)
	}
}

func runValueSizeBenchmarks(t *testing.T) {
	fmt.Println("| Value Size | Set ns/op   | Get ns/op   |")
	fmt.Println("|------------|-------------|-------------|")

	for _, size := range []int{64, 256, 1024, 4096, 16384} {
		setResult := testing.Benchmark(benchSetValueSizeFactory(size))
		getResult := testing.Benchmark(benchGetValueSizeFactory(size))
		fmt.Printf("| %10d | %11.0f | %11.0f |\n",
			size,
			float64(setResult.NsPerOp()),
			float64(getResult.NsPerOp()))
	}
}

// Exported benchmarks for go test -bench=.
func BenchmarkLocalFSSetCold(b *testing.B)       { benchSetCold(b) }
func BenchmarkLocalFSSetWarm(b *testing.B)       { benchSetWarm(b) }
func BenchmarkLocalFSGetHit(b *testing.B)        { benchGetHit(b) }
func BenchmarkLocalFSGetMiss(b *testing.B)       { benchGetMiss(b) }
func BenchmarkLocalFSDelete(b *testing.B)        { benchDelete(b) }
func BenchmarkLocalFSConcurrent(b *testing.B)    { benchConcurrent(b) }
func BenchmarkLocalFSSetSmall(b *testing.B)      { benchSetValueSizeFactory(smallValueSize)(b) }
func BenchmarkLocalFSSetMedium(b *testing.B)     { benchSetValueSizeFactory(mediumValueSize)(b) }
func BenchmarkLocalFSSetLarge(b *testing.B)      { benchSetValueSizeFactory(largeValueSize)(b) }
func BenchmarkLocalFSGetSmall(b *testing.B)      { benchGetValueSizeFactory(smallValueSize)(b) }
func BenchmarkLocalFSGetMedium(b *testing.B)     { benchGetValueSizeFactory(mediumValueSize)(b) }
func BenchmarkLocalFSGetLarge(b *testing.B)      { benchGetValueSizeFactory(largeValueSize)(b) }
func BenchmarkLocalFSKeyToFilename(b *testing.B) { benchKeyToFilename(b) }

func createTestStore(b *testing.B) *Store[string, []byte] {
	b.Helper()
	dir := b.TempDir()
	store, err := New[string, []byte]("bench", dir)
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	return store
}

func benchSetCold(b *testing.B) {
	store := createTestStore(b)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	b.ResetTimer()
	for i := range b.N {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}
}

func benchSetWarm(b *testing.B) {
	store := createTestStore(b)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	// Pre-populate
	for i := range benchCacheSize {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}

	b.ResetTimer()
	for i := range b.N {
		key := "key-" + strconv.Itoa(i%benchCacheSize)
		store.Set(ctx, key, value, expiry)
	}
}

func benchGetHit(b *testing.B) {
	store := createTestStore(b)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	// Pre-populate
	for i := range benchCacheSize {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}

	b.ResetTimer()
	for i := range b.N {
		key := "key-" + strconv.Itoa(i%benchCacheSize)
		store.Get(ctx, key)
	}
}

func benchGetMiss(b *testing.B) {
	store := createTestStore(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		key := "miss-" + strconv.Itoa(i)
		store.Get(ctx, key)
	}
}

func benchDelete(b *testing.B) {
	store := createTestStore(b)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	// Pre-populate
	for i := range b.N {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}

	b.ResetTimer()
	for i := range b.N {
		key := "key-" + strconv.Itoa(i)
		store.Delete(ctx, key)
	}
}

func benchConcurrent(b *testing.B) {
	store := createTestStore(b)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	// Pre-populate
	for i := range benchCacheSize {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key-" + strconv.Itoa(i%benchCacheSize)
			if i%4 == 0 {
				store.Set(ctx, key, value, expiry)
			} else {
				store.Get(ctx, key)
			}
			i++
		}
	})
}

func benchSetValueSizeFactory(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		store := createTestStore(b)
		ctx := context.Background()
		value := make([]byte, valueSize)
		expiry := time.Now().Add(time.Hour)

		b.ResetTimer()
		for i := range b.N {
			key := "key-" + strconv.Itoa(i%benchCacheSize)
			store.Set(ctx, key, value, expiry)
		}
	}
}

func benchGetValueSizeFactory(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		store := createTestStore(b)
		ctx := context.Background()
		value := make([]byte, valueSize)
		expiry := time.Now().Add(time.Hour)

		// Pre-populate
		for i := range benchCacheSize {
			key := "key-" + strconv.Itoa(i)
			store.Set(ctx, key, value, expiry)
		}

		b.ResetTimer()
		for i := range b.N {
			key := "key-" + strconv.Itoa(i%benchCacheSize)
			store.Get(ctx, key)
		}
	}
}

func benchKeyToFilename(b *testing.B) {
	store := &Store[string, []byte]{}
	keys := make([]string, benchCacheSize)
	for i := range benchCacheSize {
		keys[i] = "key-" + strconv.Itoa(i)
	}

	b.ResetTimer()
	for i := range b.N {
		store.keyToFilename(keys[i%benchCacheSize])
	}
}

// Concurrent measurement helpers
const concurrentBenchDuration = 2 * time.Second

func measureConcurrentRead(threads int) float64 {
	dir, _ := createTempDir()
	store, _ := New[string, []byte]("bench", dir)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	// Pre-populate
	for i := range benchCacheSize {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}

	var ops atomic.Int64
	var stop atomic.Bool
	var wg sync.WaitGroup

	for range threads {
		wg.Go(func() {
			for i := 0; !stop.Load(); i++ {
				key := "key-" + strconv.Itoa(i%benchCacheSize)
				store.Get(ctx, key)
				ops.Add(1)
			}
		})
	}

	time.Sleep(concurrentBenchDuration)
	stop.Store(true)
	wg.Wait()

	return float64(ops.Load()) / concurrentBenchDuration.Seconds()
}

func measureConcurrentWrite(threads int) float64 {
	dir, _ := createTempDir()
	store, _ := New[string, []byte]("bench", dir)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	var ops atomic.Int64
	var stop atomic.Bool
	var wg sync.WaitGroup

	for range threads {
		wg.Go(func() {
			for i := 0; !stop.Load(); i++ {
				key := "key-" + strconv.Itoa(i%benchCacheSize)
				store.Set(ctx, key, value, expiry)
				ops.Add(1)
			}
		})
	}

	time.Sleep(concurrentBenchDuration)
	stop.Store(true)
	wg.Wait()

	return float64(ops.Load()) / concurrentBenchDuration.Seconds()
}

func measureConcurrentMixed(threads int) float64 {
	dir, _ := createTempDir()
	store, _ := New[string, []byte]("bench", dir)
	ctx := context.Background()
	value := make([]byte, mediumValueSize)
	expiry := time.Now().Add(time.Hour)

	// Pre-populate
	for i := range benchCacheSize {
		key := "key-" + strconv.Itoa(i)
		store.Set(ctx, key, value, expiry)
	}

	var ops atomic.Int64
	var stop atomic.Bool
	var wg sync.WaitGroup

	for range threads {
		wg.Go(func() {
			for i := 0; !stop.Load(); i++ {
				key := "key-" + strconv.Itoa(i%benchCacheSize)
				if i%4 == 0 { // 25% writes
					store.Set(ctx, key, value, expiry)
				} else { // 75% reads
					store.Get(ctx, key)
				}
				ops.Add(1)
			}
		})
	}

	time.Sleep(concurrentBenchDuration)
	stop.Store(true)
	wg.Wait()

	return float64(ops.Load()) / concurrentBenchDuration.Seconds()
}

func createTempDir() (string, error) {
	return fmt.Sprintf("/tmp/localfs-bench-%d", time.Now().UnixNano()), nil
}
