package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

var keepAlive interface{}

func main() {
	_ = flag.Int("iter", 100000, "unused in this mode")
	cap := flag.Int("cap", 25000, "capacity")
	valSize := flag.Int("valSize", 1024, "value size")
	flag.Parse()

	runtime.GC()
	debug.FreeOSMemory()

	cache, _ := lru.New[string, []byte](*cap)

	// Run 3 passes to ensure admission policies accept the items
	for pass := 0; pass < 3; pass++ {
		for i := range *cap {
			key := "key-" + strconv.Itoa(i)
			val := make([]byte, *valSize)
			cache.Add(key, val)
		}
	}

	keepAlive = cache

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	debug.FreeOSMemory()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	fmt.Printf(`{"name":"lru", "items":%d, "bytes":%d}`, cache.Len(), mem.Alloc)
}
