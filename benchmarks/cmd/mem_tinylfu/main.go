package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/vmihailenco/go-tinylfu"
)

var keepAlive interface{}

func main() {
	_ = flag.Int("iter", 100000, "unused in this mode")
	cap := flag.Int("cap", 25000, "capacity")
	valSize := flag.Int("valSize", 1024, "value size")
	flag.Parse()

	runtime.GC()
	debug.FreeOSMemory()

	cache := tinylfu.NewSync(*cap, *cap*10)

	// Set and immediately access items to force promotion from Window to Main.
	// TinyLFU is scan-resistant and will reject a pure loop (0..cap) if the loop is larger than the Window size (~1%).
	// By accessing immediately, we prove the item has frequency > 1.
	for i := range *cap {
		key := "key-" + strconv.Itoa(i)
		val := make([]byte, *valSize)
		cache.Set(&tinylfu.Item{Key: key, Value: val})
		
		// Boost frequency
		cache.Get(key)
		cache.Get(key)
		cache.Get(key)
	}

	keepAlive = cache

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	debug.FreeOSMemory()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	count := 0
	for i := range *cap {
		key := "key-" + strconv.Itoa(i)
		if _, ok := cache.Get(key); ok {
			count++
		}
	}

	fmt.Printf(`{"name":"tinylfu", "items":%d, "bytes":%d}`, count, mem.Alloc)
}
