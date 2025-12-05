package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/coocood/freecache"
)

var keepAlive interface{}

func main() {
	_ = flag.Int("iter", 100000, "unused in this mode")
	cap := flag.Int("cap", 25000, "capacity")
	valSize := flag.Int("valSize", 1024, "value size")
	flag.Parse()

	runtime.GC()
	debug.FreeOSMemory()

	// Freecache size in bytes
	overhead := 256 // per entry overhead estimate
	size := *cap * (*valSize + overhead)
	cache := freecache.NewCache(size)

	// Run 3 passes to ensure admission policies accept the items
	for pass := 0; pass < 3; pass++ {
		for i := range *cap {
			key := "key-" + strconv.Itoa(i)
			val := make([]byte, *valSize)
			cache.Set([]byte(key), val, 0)
		}
	}

	keepAlive = cache

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	debug.FreeOSMemory()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	fmt.Printf(`{"name":"freecache", "items":%d, "bytes":%d}`, cache.EntryCount(), mem.Alloc)
}
