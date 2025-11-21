# bdcache - Big Dumb Cache

<img src="media/logo-small.png" alt="bdcache logo" width="256">

[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/bdcache.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/bdcache)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/bdcache)](https://goreportcard.com/report/github.com/codeGROOVE-dev/bdcache)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

<br clear="right">

Fast, persistent Go cache with S3-FIFO eviction - better hit rates than LRU, survives restarts with pluggable persistence backends, zero allocations.

## Install

```bash
go get github.com/codeGROOVE-dev/bdcache
```

## Use

```go
import (
    "github.com/codeGROOVE-dev/bdcache"
    "github.com/codeGROOVE-dev/bdcache/persist/localfs"
)

// Memory only
cache, _ := bdcache.New[string, int](ctx)
cache.Set(ctx, "answer", 42, 0)           // Synchronous: returns after persistence completes
cache.SetAsync(ctx, "answer", 42, 0)      // Async: returns immediately, persists in background
val, found, _ := cache.Get(ctx, "answer")

// With local file persistence
p, _ := localfs.New[string, User]("myapp", "")
cache, _ := bdcache.New[string, User](ctx,
    bdcache.WithPersistence(p))

// With Valkey/Redis persistence
p, _ := valkey.New[string, User](ctx, "myapp", "localhost:6379")
cache, _ := bdcache.New[string, User](ctx,
    bdcache.WithPersistence(p))

// Cloud Run auto-detection (datastore in Cloud Run, localfs elsewhere)
p, _ := cloudrun.New[string, User](ctx, "myapp")
cache, _ := bdcache.New[string, User](ctx,
    bdcache.WithPersistence(p))
```

## Features

- **S3-FIFO eviction** - Better than LRU ([learn more](https://s3fifo.com/))
- **Type safe** - Go generics
- **Pluggable persistence** - Bring your own database or use built-in backends:
  - [`persist/localfs`](persist/localfs) - Local files (gob encoding, zero dependencies)
  - [`persist/datastore`](persist/datastore) - Google Cloud Datastore
  - [`persist/valkey`](persist/valkey) - Valkey/Redis
  - [`persist/cloudrun`](persist/cloudrun) - Auto-detect Cloud Run
- **Graceful degradation** - Cache works even if persistence fails
- **Per-item TTL** - Optional expiration

## Performance

### vs Popular Go Caches

Benchmarks on MacBook Pro M4 Max comparing memory-only Get operations:

| Library | Algorithm | ns/op | Persistence |
|---------|-----------|-------|-------------|
| **bdcache** | S3-FIFO | **8.61** | Yes |
| golang-lru | LRU | 13.02 | ❌ None |
| otter | S3-FIFO | 14.58 | ⚠️ Manual (Save/Load entire cache) |
| ristretto | TinyLFU | 30.53 | ❌ None |

> ⚠️ **Benchmark Disclaimer**: These benchmarks are highly cherrypicked to show S3-FIFO's advantages. Different cache implementations excel at different workloads - LRU may outperform S3-FIFO in some scenarios, while TinyLFU shines in others. Performance varies based on access patterns, working set size, and hardware.
>
> **The real differentiator** is bdcache's automatic per-item persistence designed for unreliable environments like Cloud Run and Kubernetes, where shutdowns are unpredictable. See [benchmarks/](benchmarks/) for methodology.

**Key advantage:**
- **Automatic persistence for unreliable environments** - per-item writes to local files or Google Cloud Datastore survive unexpected shutdowns (Cloud Run, Kubernetes), container restarts, and crashes without manual save/load choreography

**Also competitive on:**
- Speed - comparable to or faster than alternatives on typical workloads
- Hit rates - S3-FIFO protects hot data from scans in specific scenarios
- Zero allocations - efficient for high-frequency operations

### Competitive Analysis

Independent benchmark using [scalalang2/go-cache-benchmark](https://github.com/scalalang2/go-cache-benchmark) (500K items, Zipfian distribution) shows bdcache consistently ranks top 1-2 for hit rate across all cache sizes:

- **0.1% cache size**: bdcache **48.12%** vs SIEVE 47.42%, TinyLFU 47.37%
- **1% cache size**: bdcache **64.45%** vs TinyLFU 63.94%, Otter 63.60%
- **10% cache size**: bdcache **80.39%** vs TinyLFU 80.43%, Otter 79.86%

See [benchmarks/](benchmarks/) for detailed methodology and running instructions.

## License

Apache 2.0
