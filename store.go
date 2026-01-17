package fido

import (
	"context"
	"iter"
	"time"
)

// Store is the persistence backend interface.
type Store[K comparable, V any] interface {
	ValidateKey(key K) error
	Get(ctx context.Context, key K) (V, time.Time, bool, error)
	Set(ctx context.Context, key K, value V, expiry time.Time) error
	Delete(ctx context.Context, key K) error
	Cleanup(ctx context.Context, maxAge time.Duration) (int, error)
	Flush(ctx context.Context) (int, error)
	Len(ctx context.Context) (int, error)
	Close() error
}

// PrefixScanner is an optional interface for stores that support efficient prefix iteration.
// Only meaningful for Store[string, V].
type PrefixScanner[V any] interface {
	// Keys returns an iterator over keys matching prefix.
	// Efficient: only lists keys, does not load values from storage.
	Keys(ctx context.Context, prefix string) iter.Seq[string]

	// Range returns an iterator over key-value pairs matching prefix.
	// More expensive than Keys: loads and decodes values from storage.
	Range(ctx context.Context, prefix string) iter.Seq2[string, V]
}
