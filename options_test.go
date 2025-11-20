package bdcache

import (
	"os"
	"testing"
)

func TestOptions_WithCloudDatastore(t *testing.T) {
	opts := defaultOptions()
	WithCloudDatastore("test-project")(opts)

	if opts.CacheID != "test-project" {
		t.Errorf("CacheID = %s; want test-project", opts.CacheID)
	}
	if !opts.UseDatastore {
		t.Error("UseDatastore should be true")
	}
}

// TestOptions_WithBestStore tests WithBestStore with K_SERVICE.
func TestOptions_WithBestStore_WithKService(t *testing.T) {
	// Set K_SERVICE environment variable
	os.Setenv("K_SERVICE", "test-service")
	defer os.Unsetenv("K_SERVICE")

	opts := defaultOptions()
	WithBestStore("test-cache")(opts)

	if !opts.UseDatastore {
		t.Error("UseDatastore should be true when K_SERVICE is set")
	}
}

// TestOptions_WithBestStore_WithoutKService tests WithBestStore without K_SERVICE.
func TestOptions_WithBestStore_WithoutKService(t *testing.T) {
	// Ensure K_SERVICE is not set
	os.Unsetenv("K_SERVICE")

	opts := defaultOptions()
	WithBestStore("test-cache")(opts)

	if opts.UseDatastore {
		t.Error("UseDatastore should be false when K_SERVICE is not set")
	}
}
