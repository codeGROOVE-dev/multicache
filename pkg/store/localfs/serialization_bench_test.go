//nolint:errcheck,thelper // benchmark code
package localfs

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/klauspost/compress/s2"
)

// TestSerializationComparison compares gob vs JSON vs compressed JSON vs S2.
func TestSerializationComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping serialization comparison in short mode")
	}

	fmt.Println()
	fmt.Println("Serialization Format Comparison")
	fmt.Println()

	valueSizes := []int{64, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768}

	fmt.Println("### Encode Performance")
	fmt.Println()
	fmt.Println("| Value Size | Gob ns/op | JSON ns/op | S2 JSON ns/op | Gzip JSON ns/op |")
	fmt.Println("|------------|-----------|------------|---------------|-----------------|")

	for _, size := range valueSizes {
		gobResult := testing.Benchmark(benchEncodeGob(size))
		jsonResult := testing.Benchmark(benchEncodeJSON(size))
		s2Result := testing.Benchmark(benchEncodeS2JSON(size))
		gzipResult := testing.Benchmark(benchEncodeGzipJSON(size))

		fmt.Printf("| %10d | %9.0f | %10.0f | %13.0f | %15.0f |\n",
			size,
			float64(gobResult.NsPerOp()),
			float64(jsonResult.NsPerOp()),
			float64(s2Result.NsPerOp()),
			float64(gzipResult.NsPerOp()))
	}

	fmt.Println()
	fmt.Println("### Decode Performance")
	fmt.Println()
	fmt.Println("| Value Size | Gob ns/op | JSON ns/op | S2 JSON ns/op | Gzip JSON ns/op |")
	fmt.Println("|------------|-----------|------------|---------------|-----------------|")

	for _, size := range valueSizes {
		gobResult := testing.Benchmark(benchDecodeGob(size))
		jsonResult := testing.Benchmark(benchDecodeJSON(size))
		s2Result := testing.Benchmark(benchDecodeS2JSON(size))
		gzipResult := testing.Benchmark(benchDecodeGzipJSON(size))

		fmt.Printf("| %10d | %9.0f | %10.0f | %13.0f | %15.0f |\n",
			size,
			float64(gobResult.NsPerOp()),
			float64(jsonResult.NsPerOp()),
			float64(s2Result.NsPerOp()),
			float64(gzipResult.NsPerOp()))
	}

	fmt.Println()
	fmt.Println("### Decode Allocations")
	fmt.Println()
	fmt.Println("| Value Size | Gob allocs | JSON allocs | S2 allocs | Gzip allocs |")
	fmt.Println("|------------|------------|-------------|-----------|-------------|")

	for _, size := range valueSizes {
		gobResult := testing.Benchmark(benchDecodeGob(size))
		jsonResult := testing.Benchmark(benchDecodeJSON(size))
		s2Result := testing.Benchmark(benchDecodeS2JSON(size))
		gzipResult := testing.Benchmark(benchDecodeGzipJSON(size))

		fmt.Printf("| %10d | %10d | %11d | %9d | %11d |\n",
			size,
			gobResult.AllocsPerOp(),
			jsonResult.AllocsPerOp(),
			s2Result.AllocsPerOp(),
			gzipResult.AllocsPerOp())
	}

	fmt.Println()
	fmt.Println("### Encoded Size (bytes on disk)")
	fmt.Println()
	fmt.Println("| Value Size | Gob Size | JSON Size | S2 JSON Size | Gzip JSON Size |")
	fmt.Println("|------------|----------|-----------|--------------|----------------|")

	for _, size := range valueSizes {
		gobSize := measureEncodedSize(size, encodeGob)
		jsonSize := measureEncodedSize(size, encodeJSON)
		s2Size := measureEncodedSize(size, encodeS2JSON)
		gzipSize := measureEncodedSize(size, encodeGzipJSON)

		fmt.Printf("| %10d | %8d | %9d | %12d | %14d |\n",
			size, gobSize, jsonSize, s2Size, gzipSize)
	}

	fmt.Println()
}

// Test entry for serialization benchmarks.
type testEntry struct {
	Key       string
	Value     []byte
	Expiry    time.Time
	UpdatedAt time.Time
}

func makeTestEntry(valueSize int) testEntry {
	value := make([]byte, valueSize)
	// Fill with semi-realistic data (not all zeros, which compresses too well)
	for i := range value {
		value[i] = byte(i % 256)
	}
	return testEntry{
		Key:       "test-key-12345",
		Value:     value,
		Expiry:    time.Now().Add(time.Hour),
		UpdatedAt: time.Now(),
	}
}

// Encode functions
func encodeGob(e testEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeJSON(e testEntry) ([]byte, error) {
	return json.Marshal(e)
}

func encodeGzipJSON(e testEntry) ([]byte, error) {
	jsonData, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeS2JSON(e testEntry) ([]byte, error) {
	jsonData, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return s2.Encode(nil, jsonData), nil
}

// Decode functions
func decodeGob(data []byte) (testEntry, error) {
	var e testEntry
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&e); err != nil {
		return e, err
	}
	return e, nil
}

func decodeJSON(data []byte) (testEntry, error) {
	var e testEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return e, err
	}
	return e, nil
}

func decodeGzipJSON(data []byte) (testEntry, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return testEntry{}, err
	}
	defer gz.Close()

	jsonData, err := io.ReadAll(gz)
	if err != nil {
		return testEntry{}, err
	}

	var e testEntry
	if err := json.Unmarshal(jsonData, &e); err != nil {
		return e, err
	}
	return e, nil
}

func decodeS2JSON(data []byte) (testEntry, error) {
	jsonData, err := s2.Decode(nil, data)
	if err != nil {
		return testEntry{}, err
	}

	var e testEntry
	if err := json.Unmarshal(jsonData, &e); err != nil {
		return e, err
	}
	return e, nil
}

// Benchmark encode functions
func benchEncodeGob(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		b.ResetTimer()
		for range b.N {
			encodeGob(e)
		}
	}
}

func benchEncodeJSON(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		b.ResetTimer()
		for range b.N {
			encodeJSON(e)
		}
	}
}

func benchEncodeGzipJSON(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		b.ResetTimer()
		for range b.N {
			encodeGzipJSON(e)
		}
	}
}

func benchEncodeS2JSON(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		b.ResetTimer()
		for range b.N {
			encodeS2JSON(e)
		}
	}
}

// Benchmark decode functions
func benchDecodeGob(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		data, _ := encodeGob(e)
		b.ResetTimer()
		for range b.N {
			decodeGob(data)
		}
	}
}

func benchDecodeJSON(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		data, _ := encodeJSON(e)
		b.ResetTimer()
		for range b.N {
			decodeJSON(data)
		}
	}
}

func benchDecodeGzipJSON(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		data, _ := encodeGzipJSON(e)
		b.ResetTimer()
		for range b.N {
			decodeGzipJSON(data)
		}
	}
}

func benchDecodeS2JSON(valueSize int) func(*testing.B) {
	return func(b *testing.B) {
		e := makeTestEntry(valueSize)
		data, _ := encodeS2JSON(e)
		b.ResetTimer()
		for range b.N {
			decodeS2JSON(data)
		}
	}
}

func measureEncodedSize(valueSize int, encode func(testEntry) ([]byte, error)) int {
	e := makeTestEntry(valueSize)
	data, _ := encode(e)
	return len(data)
}

// Exported benchmarks for go test -bench
func BenchmarkEncodeGob1K(b *testing.B)      { benchEncodeGob(1024)(b) }
func BenchmarkEncodeJSON1K(b *testing.B)     { benchEncodeJSON(1024)(b) }
func BenchmarkEncodeGzipJSON1K(b *testing.B) { benchEncodeGzipJSON(1024)(b) }
func BenchmarkDecodeGob1K(b *testing.B)      { benchDecodeGob(1024)(b) }
func BenchmarkDecodeJSON1K(b *testing.B)     { benchDecodeJSON(1024)(b) }
func BenchmarkDecodeGzipJSON1K(b *testing.B) { benchDecodeGzipJSON(1024)(b) }
