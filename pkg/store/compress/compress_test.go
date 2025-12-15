package compress

import (
	"bytes"
	"testing"
)

var benchData = []byte(`{"key":"test-key-12345","value":{"name":"benchmark","count":42,"tags":["test","benchmark","compression"],"created":"2024-01-01T00:00:00Z"},"expiry":"2025-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}`)

func BenchmarkCompressors(b *testing.B) {
	compressors := []struct {
		name string
		c    Compressor
	}{
		{"None", None()},
		{"S2", S2()},
		{"Zstd-1", Zstd(1)},
		{"Zstd-4", Zstd(4)},
	}

	for _, tc := range compressors {
		b.Run(tc.name+"/Encode", func(b *testing.B) {
			b.SetBytes(int64(len(benchData)))
			b.ReportAllocs()
			for range b.N {
				_, _ = tc.c.Encode(benchData) //nolint:errcheck // benchmark
			}
		})

		// Pre-encode for decode benchmark
		encoded, _ := tc.c.Encode(benchData) //nolint:errcheck // setup for benchmark
		b.Run(tc.name+"/Decode", func(b *testing.B) {
			b.SetBytes(int64(len(encoded)))
			b.ReportAllocs()
			for range b.N {
				_, _ = tc.c.Decode(encoded) //nolint:errcheck // benchmark
			}
		})
	}
}

func TestCompressorsRoundTrip(t *testing.T) {
	compressors := []struct {
		name string
		c    Compressor
		ext  string
	}{
		{"None", None(), ""},
		{"S2", S2(), ".s"},
		{"Zstd-1", Zstd(1), ".z"},
		{"Zstd-4", Zstd(4), ".z"},
	}

	for _, tc := range compressors {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := tc.c.Encode(benchData)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			decoded, err := tc.c.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}

			if !bytes.Equal(decoded, benchData) {
				t.Errorf("roundtrip failed: got %q, want %q", decoded, benchData)
			}

			if tc.c.Extension() != tc.ext {
				t.Errorf("Extension = %q, want %q", tc.c.Extension(), tc.ext)
			}
		})
	}
}

func TestNoneZeroCopy(t *testing.T) {
	c := None()
	data := []byte("test data")

	encoded, err := c.Encode(data)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if &encoded[0] != &data[0] {
		t.Error("None.Encode should return same slice (zero-copy)")
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if &decoded[0] != &data[0] {
		t.Error("None.Decode should return same slice (zero-copy)")
	}
}
