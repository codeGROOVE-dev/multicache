// Package cloudrun provides automatic persistence backend selection for Cloud Run environments.
// It detects Cloud Run via K_SERVICE environment variable and attempts to use Datastore,
// falling back to local files if Datastore is unavailable or if not running in Cloud Run.
package cloudrun

import (
	"context"
	"log/slog"
	"os"

	"github.com/codeGROOVE-dev/bdcache"
	"github.com/codeGROOVE-dev/bdcache/persist/datastore"
	"github.com/codeGROOVE-dev/bdcache/persist/localfs"
)

// New creates a persistence layer optimized for Cloud Run environments.
// It automatically selects the best available backend:
//   - In Cloud Run (K_SERVICE env var set): tries Datastore, falls back to local files on error
//   - Outside Cloud Run: uses local files
//
// The cacheID is used as the database name for Datastore or subdirectory for local files.
// This function always succeeds by falling back to local files if Datastore is unavailable.
func New[K comparable, V any](ctx context.Context, cacheID string) (bdcache.PersistenceLayer[K, V], error) {
	// Check if running in Cloud Run
	if os.Getenv("K_SERVICE") != "" {
		slog.Debug("detected cloud run environment", "service", os.Getenv("K_SERVICE"))

		// Try Datastore first in Cloud Run
		p, err := datastore.New[K, V](ctx, cacheID)
		if err == nil {
			slog.Info("using datastore persistence", "cacheID", cacheID)
			return p, nil
		}
		slog.Warn("datastore unavailable in cloud run, falling back to local files",
			"error", err,
			"cacheID", cacheID)
	}

	// Fall back to local files (either not in Cloud Run, or Datastore failed)
	p, err := localfs.New[K, V](cacheID, "")
	if err != nil {
		return nil, err
	}

	slog.Info("using local file persistence", "cacheID", cacheID)
	return p, nil
}
