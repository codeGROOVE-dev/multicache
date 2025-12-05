package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/maypok86/otter/v2"
)

var keepAlive interface{}

func main() {
	_ = flag.Int("iter", 100000, "unused in this mode")
	cap := flag.Int("cap", 25000, "capacity")
	valSize := flag.Int("valSize", 1024, "value size")
	flag.Parse()

	runtime.GC()
	debug.FreeOSMemory()

	cache := otter.Must(&otter.Options[string, []byte]{MaximumSize: *cap})

	// Run 3 passes to ensure admission policies accept the items
	for pass := 0; pass < 3; pass++ {
		for i := range *cap {
			key := "key-" + strconv.Itoa(i)
			val := make([]byte, *valSize)
			cache.Set(key, val)
		}
	}

	keepAlive = cache

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	debug.FreeOSMemory()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// Count items manually
	count := 0
	for i := range *cap {
		if _, ok := cache.GetIfPresent("key-" + strconv.Itoa(i)); ok {
			count++
		}
	}

	fmt.Printf(`{"name":"otter", "items":%d, "bytes":%d}`, count, mem.Alloc)
}
