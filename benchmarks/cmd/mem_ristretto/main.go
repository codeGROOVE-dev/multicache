package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/dgraph-io/ristretto"
)

var keepAlive interface{}

func main() {
	_ = flag.Int("iter", 100000, "unused in this mode")
	cap := flag.Int("cap", 25000, "capacity")
	valSize := flag.Int("valSize", 1024, "value size")
	flag.Parse()

	runtime.GC()
	debug.FreeOSMemory()

	// Ristretto config: NumCounters should be 10x MaxCost for best performance
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters:        int64(*cap * 10),
		MaxCost:            int64(*cap),
		BufferItems:        64 * 1024, // Increase buffer to avoid drops during ingestion
		IgnoreInternalCost: true,
	})

	// Run 3 passes to ensure admission policies accept the items
	for pass := 0; pass < 3; pass++ {
		for i := range *cap {
			key := "key-" + strconv.Itoa(i)
			val := make([]byte, *valSize)
			cache.Set(key, val, 1) // Cost 1 per item
		}
	}
	cache.Wait()

	keepAlive = cache

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	debug.FreeOSMemory()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	count := 0
	for i := range *cap {
		if _, ok := cache.Get("key-" + strconv.Itoa(i)); ok {
			count++
		}
	}

	fmt.Printf(`{"name":"ristretto", "items":%d, "bytes":%d}`, count, mem.Alloc)
}
