# bdcache - Big Dumb Cache

[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/bdcache.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/bdcache)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/bdcache)](https://goreportcard.com/report/github.com/codeGROOVE-dev/bdcache)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Simple, fast, reliable Go cache with [S3-FIFO eviction](https://s3fifo.com/) - better hit rates than LRU.

## Why?

- **S3-FIFO Algorithm** - [Superior cache hit rates](https://s3fifo.com/) compared to LRU/LFU
- **Fast** - ~19ns per operation, zero allocations
- **Reliable** - Memory cache always works, even if persistence fails
- **Smart Persistence** - Local files for dev. Cloud Datastore for Cloud Run
- **Minimal Dependencies** - Only one optional dependency (Cloud Datastore)

## Install

```bash
go get github.com/codeGROOVE-dev/bdcache
```

## Use

```go
// Memory only
cache, err := bdcache.New[string, int](ctx)
if err != nil {
    panic(err)
}
if err := cache.Set(ctx, "answer", 42, 0); err != nil {
    panic(err)
}
val, found, err := cache.Get(ctx, "answer")

// With smart persistence (files for dev, Datastore for Cloud Run)
cache, err := bdcache.New[string, User](ctx, bdcache.WithBestStore("myapp"))
```

## Features

- **S3-FIFO eviction** - Better than LRU ([learn more](https://s3fifo.com/))
- **Type safe** - Go generics
- **Persistence** - Local files (gob) or Cloud Datastore (JSON)
- **Graceful degradation** - Cache works even if persistence fails
- **Per-item TTL** - Optional expiration

## Performance

```
BenchmarkCache_Get_Hit-16      66M ops/sec    18.5 ns/op    0 allocs
BenchmarkCache_Set-16          61M ops/sec    19.3 ns/op    0 allocs
```

## License

Apache 2.0
