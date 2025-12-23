# multicache - Adaptive Multi-Tier Cache

<img src="media/logo-small.png" alt="multicache logo" width="256">

[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/multicache.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/multicache)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/multicache)](https://goreportcard.com/report/github.com/codeGROOVE-dev/multicache)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

<br clear="right">

multicache is a high-performance cache for Go that automatically adapts to your workload. It combines **multiple eviction strategies** that switch based on access patterns, with an optional **multi-tier architecture** for persistence.

## Why "multi"?

### Multiple Adaptive Strategies

multicache monitors ghost hit rates (how often evicted keys return) and automatically selects the optimal eviction strategy:

| Mode | Trigger | Strategy | Workload |
|------|---------|----------|----------|
| 0 | Ghost rate <1% | Pure recency | Scan-heavy (unique keys) |
| 1 | Ghost rate 1-22% | Balanced S3-FIFO | Mixed access patterns |
| 2 | Ghost rate 7-12% | Frequency-biased | Repeated hot keys |
| 3 | Ghost rate ≥23% | Clock-like second-chance | High temporal locality |

No tuning required - the cache learns your workload and adapts.

### Multi-Tier Architecture

Stack fast in-memory caching with durable persistence:

```
┌─────────────────────────────────────┐
│         Your Application            │
└─────────────────┬───────────────────┘
                  │
┌─────────────────▼───────────────────┐
│    Memory Cache (microseconds)      │  ← L1: S3-FIFO with adaptive modes
└─────────────────┬───────────────────┘
                  │ async write / sync read
┌─────────────────▼───────────────────┐
│   Persistence Store (milliseconds)  │  ← L2: localfs, Valkey, Datastore
└─────────────────────────────────────┘
```

Persistence backends:
- [`pkg/store/localfs`](pkg/store/localfs) - Local files (JSON, zero dependencies)
- [`pkg/store/valkey`](pkg/store/valkey) - Valkey/Redis
- [`pkg/store/datastore`](pkg/store/datastore) - Google Cloud Datastore
- [`pkg/store/cloudrun`](pkg/store/cloudrun) - Auto-selects Datastore or localfs
- [`pkg/store/null`](pkg/store/null) - No-op for testing

All backends support optional S2 or Zstd compression via [`pkg/store/compress`](pkg/store/compress).

## Features

- **Best-in-class performance** - 7ns reads, 100M+ QPS single-threaded
- **Adaptive S3-FIFO eviction** - Better hit-rates than LRU ([learn more](https://s3fifo.com/))
- **Thundering herd prevention** - `GetSet` deduplicates concurrent loads
- **Per-item TTL** - Optional expiration
- **Graceful degradation** - Cache works even if persistence fails
- **Zero allocation updates** - Minimal GC pressure

## Usage

As a stupid-fast in-memory cache:

```go
import "github.com/codeGROOVE-dev/multicache"

// strings as keys, ints as values
cache := multicache.New[string, int]()
cache.Set("answer", 42)
val, found := cache.Get("answer")
```

Or as a multi-tier cache with local persistence to survive restarts:

```go
import (
  "github.com/codeGROOVE-dev/multicache"
  "github.com/codeGROOVE-dev/multicache/pkg/store/localfs"
)

p, _ := localfs.New[string, User]("myapp", "")
cache, _ := multicache.NewTiered(p)

cache.SetAsync(ctx, "user:123", user) // Don't wait for the key to persist
cache.Store.Len(ctx)                  // Access persistence layer directly
```

With S2 compression (fast, good ratio):

```go
import "github.com/codeGROOVE-dev/multicache/pkg/store/compress"

p, _ := localfs.New[string, User]("myapp", "", compress.S2())
```

How about a persistent cache suitable for Cloud Run or local development? This uses Cloud DataStore if available, local files if not:

```go
import "github.com/codeGROOVE-dev/multicache/pkg/store/cloudrun"

p, _ := cloudrun.New[string, User](ctx, "myapp")
cache, _ := multicache.NewTiered(p)
```

## Performance against the Competition

multicache prioritizes high hit-rates and low read latency. We have our own built in `make bench` that asserts cache dominance:

```
>>> TestLatencyNoEviction: Latency - No Evictions (Set cycles within cache size) (go test -run=TestLatencyNoEviction -v)
| Cache         | Get ns/op | Get B/op | Get allocs | Set ns/op | Set B/op | Set allocs |
|---------------|-----------|----------|------------|-----------|----------|------------|
| multicache       |       7.0 |        0 |          0 |      23.0 |        0 |          0 |
| lru           |      23.0 |        0 |          0 |      23.0 |        0 |          0 |
| ristretto     |      28.0 |       13 |          0 |      77.0 |      118 |          3 |
| otter         |      34.0 |        0 |          0 |     160.0 |       51 |          1 |
| freecache     |      74.0 |        8 |          1 |      53.0 |        0 |          0 |
| tinylfu       |      80.0 |        0 |          0 |     110.0 |      168 |          3 |

- 🔥 Get: 229% better than next best (lru)
- 🔥 Set: 0.000% better than next best (lru)

>>> TestLatencyWithEviction: Latency - With Evictions (Set uses 20x unique keys) (go test -run=TestLatencyWithEviction -v)
| Cache         | Get ns/op | Get B/op | Get allocs | Set ns/op | Set B/op | Set allocs |
|---------------|-----------|----------|------------|-----------|----------|------------|
| multicache       |       7.0 |        0 |          0 |      94.0 |        0 |          0 |
| lru           |      24.0 |        0 |          0 |      83.0 |       80 |          1 |
| ristretto     |      31.0 |       14 |          0 |      73.0 |      119 |          3 |
| otter         |      34.0 |        0 |          0 |     176.0 |       61 |          1 |
| freecache     |      69.0 |        8 |          1 |     102.0 |        1 |          0 |
| tinylfu       |      79.0 |        0 |          0 |     115.0 |      168 |          3 |

- 🔥 Get: 243% better than next best (lru)
- 💧 Set: 29% worse than best (ristretto)

>>> TestZipfThroughput1: Zipf Throughput (1 thread) (go test -run=TestZipfThroughput1 -v)

### Zipf Throughput (alpha=0.99, 75% read / 25% write): 1 threads

| Cache         | QPS        |
|---------------|------------|
| multicache       |  100.26M   |
| lru           |   44.58M   |
| tinylfu       |   18.42M   |
| freecache     |   14.07M   |
| otter         |   13.52M   |
| ristretto     |   11.32M   |

- 🔥 Throughput: 125% faster than next best (lru)

>>> TestZipfThroughput16: Zipf Throughput (16 threads) (go test -run=TestZipfThroughput16 -v)

### Zipf Throughput (alpha=0.99, 75% read / 25% write): 16 threads

| Cache         | QPS        |
|---------------|------------|
| multicache       |   36.46M   |
| freecache     |   15.00M   |
| ristretto     |   13.47M   |
| otter         |   10.75M   |
| lru           |    5.87M   |
| tinylfu       |    4.19M   |

- 🔥 Throughput: 143% faster than next best (freecache)

>>> TestMetaTrace: Meta Trace Hit Rate (10M ops) (go test -run=TestMetaTrace -v)

### Meta Trace Hit Rate (10M ops from Meta KVCache)

| Cache         | 50K cache | 100K cache |
|---------------|-----------|------------|
| multicache       |   71.16%  |   78.30%   |
| otter         |   41.12%  |   56.34%   |
| ristretto     |   40.35%  |   48.99%   |
| tinylfu       |   53.70%  |   54.79%   |
| freecache     |   56.86%  |   65.52%   |
| lru           |   65.21%  |   74.22%   |

- 🔥 Meta trace: 5.5% better than next best (lru)

>>> TestHitRate: Zipf Hit Rate (go test -run=TestHitRate -v)

### Hit Rate (Zipf alpha=0.99, 1M ops, 1M keyspace)

| Cache         | Size=1% | Size=2.5% | Size=5% |
|---------------|---------|-----------|---------|
| multicache       |  63.80% |    68.71% |  71.84% |
| otter         |  61.77% |    67.67% |  71.33% |
| ristretto     |  34.91% |    41.23% |  46.58% |
| tinylfu       |  63.83% |    68.25% |  71.56% |
| freecache     |  56.65% |    57.84% |  63.39% |
| lru           |  57.33% |    64.55% |  69.92% |

- 🔥 Hit rate: 0.34% better than next best (tinylfu)
```

Want even more comprehensive benchmarks? See https://github.com/tstromberg/gocachemark where we win the top score.

## Implementation Notes

### S3-FIFO Enhancements

multicache implements the S3-FIFO algorithm from SOSP'23 with these optimizations:

1. **Dynamic Sharding** - 1-2048 independent shards for concurrent workloads
2. **Bloom Filter Ghosts** - Two rotating Bloom filters (vs storing keys), 10-100x less memory
3. **Lazy Ghost Checks** - Only check ghosts at capacity, saving 5-9% latency during warmup
4. **Intrusive Lists** - Zero-allocation queue operations
5. **Fast-path Hashing** - Specialized `int`/`string` hashing via wyhash

### Adaptive Mode Details

Mode switching uses **hysteresis** to prevent oscillation. Mode 2 (frequency-biased) requires 7-12% ghost rate to enter, but stays active while rate is 5-22%.

Additional tuning beyond the paper:
- **Adaptive queue sizing** - Small queue is 20% for caches ≤32K, 15% for ≤128K, 10% for larger
- **Ghost frequency boost** - Returning items start with freq=1 instead of 0
- **Higher frequency cap** - Max freq=7 (vs 3) for better hot/warm discrimination

## License

Apache 2.0
