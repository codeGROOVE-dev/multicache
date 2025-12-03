# bdcache - Big Dumb Cache

<img src="media/logo-small.png" alt="bdcache logo" width="256">

[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/bdcache.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/bdcache)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/bdcache)](https://goreportcard.com/report/github.com/codeGROOVE-dev/bdcache)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

<br clear="right">

Stupid fast in-memory Go cache with optional L2 persistence layer.

Designed originally for persistently caching HTTP fetches in unreliable environments like Google Cloud Run, this cache has something for everyone.

## Features

- **Faster than a bat out of hell** - Best-in-class latency and throughput
- **S3-FIFO eviction** - Better hit-rates than LRU ([learn more](https://s3fifo.com/))
- **Pluggable persistence** - Bring your own database or use built-in backends:
  - [`persist/localfs`](persist/localfs) - Local files (gob encoding, zero dependencies)
  - [`persist/datastore`](persist/datastore) - Google Cloud Datastore
  - [`persist/valkey`](persist/valkey) - Valkey/Redis
  - [`persist/cloudrun`](persist/cloudrun) - Auto-detect Cloud Run
- **Per-item TTL** - Optional expiration
- **Graceful degradation** - Cache works even if persistence fails
- **Zero allocation reads** - minimal GC thrashing
- **Type safe** - Go generics

## Usage

As a stupid-fast in-memory cache:

```go
import "github.com/codeGROOVE-dev/bdcache"

// strings as keys, ints as values
cache := bdcache.Memory[string, int]()
cache.Set("answer", 42, 0)
val, found := cache.Get("answer")
```

or with local file persistence to survive restarts:

```go
import (
  "github.com/codeGROOVE-dev/bdcache"
  "github.com/codeGROOVE-dev/bdcache/persist/localfs"
)

p, _ := localfs.New[string, User]("myapp", "")
cache, _ := bdcache.Persistent[string, User](ctx, p)

cache.SetAsync(ctx, "user:123", user, 0) // Don't wait for the key to persist
cache.Store.Len(ctx)                      // Access persistence layer directly
```

A persistent cache suitable for Cloud Run or local development; uses Cloud Datastore if available

```go
p, _ := cloudrun.New[string, User](ctx, "myapp")
cache, _ := bdcache.Persistent[string, User](ctx, p)
```

## Performance against the Competition

bdcache prioritizes high hit-rates and low read latency, but it performs quite well all around.

Here's the results from an M4 MacBook Pro - run `make bench` to see the results for yourself:
### Hit Rate (Zipf α=0.99, 1M ops, 1M keyspace)

| Cache         | Size=1% | Size=2.5% | Size=5% |
|---------------|---------|-----------|---------|
| bdcache 🟡    |  94.45% |    94.91% |  95.09% |
| otter 🦦      |  94.28% |    94.69% |  95.09% |
| ristretto ☕  |  91.63% |    92.44% |  93.02% |
| tinylfu 🔬    |  94.31% |    94.87% |  95.09% |
| freecache 🆓  |  94.03% |    94.15% |  94.75% |
| lru 📚        |  94.10% |    94.84% |  95.09% |

🏆 Hit rate: +0.1% better than 2nd best (tinylfu)

### Single-Threaded Latency (sorted by Get)

| Cache         | Get ns/op | Get B/op | Get allocs | Set ns/op | Set B/op | Set allocs |
|---------------|-----------|----------|------------|-----------|----------|------------|
| bdcache 🟡    |       7.0 |        0 |          0 |      12.0 |        0 |          0 |
| lru 📚        |      24.0 |        0 |          0 |      22.0 |        0 |          0 |
| ristretto ☕  |      30.0 |       13 |          0 |      69.0 |      119 |          3 |
| otter 🦦      |      32.0 |        0 |          0 |     145.0 |       51 |          1 |
| freecache 🆓  |      72.0 |       15 |          1 |      57.0 |        4 |          0 |
| tinylfu 🔬    |      89.0 |        3 |          0 |     106.0 |      175 |          3 |

🏆 Get latency: +243% faster than 2nd best (lru)
🏆 Set latency: +83% faster than 2nd best (lru)

### Single-Threaded Throughput (mixed read/write)

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   77.36M   |   61.54M   |
| lru 📚        |   34.69M   |   35.25M   |
| ristretto ☕  |   29.44M   |   13.61M   |
| otter 🦦      |   25.63M   |    7.10M   |
| freecache 🆓  |   12.92M   |   15.65M   |
| tinylfu 🔬    |   10.87M   |    8.93M   |

🏆 Get throughput: +123% faster than 2nd best (lru)
🏆 Set throughput: +75% faster than 2nd best (lru)

### Concurrent Throughput (mixed read/write): 4 threads

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   45.67M   |   38.65M   |
| otter 🦦      |   28.11M   |    4.06M   |
| ristretto ☕  |   27.06M   |   13.41M   |
| freecache 🆓  |   24.67M   |   20.84M   |
| lru 📚        |    9.29M   |    9.56M   |
| tinylfu 🔬    |    5.72M   |    4.94M   |

🏆 Get throughput: +62% faster than 2nd best (otter)
🏆 Set throughput: +85% faster than 2nd best (freecache)

### Concurrent Throughput (mixed read/write): 8 threads

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   22.31M   |   22.84M   |
| otter 🦦      |   19.49M   |    3.30M   |
| ristretto ☕  |   18.67M   |   11.46M   |
| freecache 🆓  |   17.34M   |   16.36M   |
| lru 📚        |    7.66M   |    7.75M   |
| tinylfu 🔬    |    4.81M   |    4.11M   |

🏆 Get throughput: +14% faster than 2nd best (otter)
🏆 Set throughput: +40% faster than 2nd best (freecache)

### Concurrent Throughput (mixed read/write): 12 threads

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   26.25M   |   24.04M   |
| ristretto ☕  |   21.71M   |   11.49M   |
| otter 🦦      |   19.78M   |    2.93M   |
| freecache 🆓  |   15.84M   |   16.10M   |
| lru 📚        |    7.50M   |    8.92M   |
| tinylfu 🔬    |    4.08M   |    3.37M   |

🏆 Get throughput: +21% faster than 2nd best (ristretto)
🏆 Set throughput: +49% faster than 2nd best (freecache)

### Concurrent Throughput (mixed read/write): 16 threads

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   16.92M   |   16.00M   |
| ristretto ☕  |   15.73M   |   11.97M   |
| otter 🦦      |   15.70M   |    2.89M   |
| freecache 🆓  |   14.67M   |   14.42M   |
| lru 📚        |    7.53M   |    8.07M   |
| tinylfu 🔬    |    4.75M   |    3.41M   |

🏆 Get throughput: +7.6% faster than 2nd best (ristretto)
🏆 Set throughput: +11% faster than 2nd best (freecache)

### Concurrent Throughput (mixed read/write): 24 threads

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   20.08M   |   16.56M   |
| ristretto ☕  |   16.76M   |   12.81M   |
| otter 🦦      |   15.71M   |    2.93M   |
| freecache 🆓  |   14.43M   |   14.59M   |
| lru 📚        |    7.71M   |    7.75M   |
| tinylfu 🔬    |    4.80M   |    3.09M   |

🏆 Get throughput: +20% faster than 2nd best (ristretto)
🏆 Set throughput: +14% faster than 2nd best (freecache)

### Concurrent Throughput (mixed read/write): 32 threads

| Cache         | Get QPS    | Set QPS    |
|---------------|------------|------------|
| bdcache 🟡    |   15.84M   |   15.29M   |
| ristretto ☕  |   15.36M   |   13.49M   |
| otter 🦦      |   15.04M   |    2.91M   |
| freecache 🆓  |   14.87M   |   13.95M   |
| lru 📚        |    7.79M   |    8.23M   |
| tinylfu 🔬    |    5.34M   |    3.09M   |

🏆 Get throughput: +3.1% faster than 2nd best (ristretto)
🏆 Set throughput: +9.6% faster than 2nd best (freecache)

NOTE: Performance characteristics often have trade-offs. There are almost certainly workloads where other cache implementations are faster, but nobody blends speed and persistence the way that bdcache does.

## License

Apache 2.0
