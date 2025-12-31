package null

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	store := New[string, int]()
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestValidateKey(t *testing.T) {
	store := New[string, int]()
	if err := store.ValidateKey("any-key"); err != nil {
		t.Errorf("ValidateKey() = %v; want nil", err)
	}
}

func TestGet(t *testing.T) {
	store := New[string, int]()
	ctx := context.Background()

	val, expiry, found, err := store.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get() error = %v; want nil", err)
	}
	if found {
		t.Error("Get() found = true; want false")
	}
	if val != 0 {
		t.Errorf("Get() value = %d; want 0", val)
	}
	if !expiry.IsZero() {
		t.Errorf("Get() expiry = %v; want zero", expiry)
	}
}

func TestSet(t *testing.T) {
	store := New[string, int]()
	ctx := context.Background()

	if err := store.Set(ctx, "key", 42, time.Now().Add(time.Hour)); err != nil {
		t.Errorf("Set() error = %v; want nil", err)
	}

	// Verify it didn't actually store
	_, _, found, err := store.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get() error = %v; want nil", err)
	}
	if found {
		t.Error("Get() after Set() found = true; want false")
	}
}

func TestDelete(t *testing.T) {
	store := New[string, int]()
	ctx := context.Background()

	if err := store.Delete(ctx, "key"); err != nil {
		t.Errorf("Delete() error = %v; want nil", err)
	}
}

func TestCleanup(t *testing.T) {
	store := New[string, int]()
	ctx := context.Background()

	n, err := store.Cleanup(ctx, time.Hour)
	if err != nil {
		t.Errorf("Cleanup() error = %v; want nil", err)
	}
	if n != 0 {
		t.Errorf("Cleanup() = %d; want 0", n)
	}
}

func TestLocation(t *testing.T) {
	store := New[string, int]()

	loc := store.Location("key")
	if loc != "null" {
		t.Errorf("Location() = %q; want %q", loc, "null")
	}
}

func TestFlush(t *testing.T) {
	store := New[string, int]()
	ctx := context.Background()

	n, err := store.Flush(ctx)
	if err != nil {
		t.Errorf("Flush() error = %v; want nil", err)
	}
	if n != 0 {
		t.Errorf("Flush() = %d; want 0", n)
	}
}

func TestLen(t *testing.T) {
	store := New[string, int]()
	ctx := context.Background()

	n, err := store.Len(ctx)
	if err != nil {
		t.Errorf("Len() error = %v; want nil", err)
	}
	if n != 0 {
		t.Errorf("Len() = %d; want 0", n)
	}
}

func TestClose(t *testing.T) {
	store := New[string, int]()

	if err := store.Close(); err != nil {
		t.Errorf("Close() error = %v; want nil", err)
	}
}
