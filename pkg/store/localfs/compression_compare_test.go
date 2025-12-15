//nolint:errcheck,thelper // benchmark code
package localfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/klauspost/compress/s2"
	"github.com/pierrec/lz4/v4"
)

const testDataPath = "/Users/t/dev/r2r/risque-testdata/known-good/08volt/social.json"

// TestCompressionComparison compares S2 vs LZ4 for real-world JSON data.
func TestCompressionComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compression comparison in short mode")
	}

	// Load test data
	data, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Skipf("test data not found: %v", err)
	}

	fmt.Printf("\nCompression Comparison: S2 vs LZ4\n")
	fmt.Printf("Test file: %s (%d bytes)\n\n", testDataPath, len(data))

	// Test with different chunk sizes to simulate various cache value sizes
	sizes := []int{1024, 4096, 16384, 65536, len(data)}

	fmt.Println("### Encode Performance (ns/op)")
	fmt.Println()
	fmt.Println("| Data Size | S2 Encode | LZ4 Encode | Winner |")
	fmt.Println("|-----------|-----------|------------|--------|")

	for _, size := range sizes {
		if size > len(data) {
			size = len(data)
		}
		chunk := data[:size]

		s2Result := testing.Benchmark(benchS2Encode(chunk))
		lz4Result := testing.Benchmark(benchLZ4Encode(chunk))

		winner := "S2"
		if lz4Result.NsPerOp() < s2Result.NsPerOp() {
			winner = "LZ4"
		}

		fmt.Printf("| %9d | %9d | %10d | %6s |\n",
			size, s2Result.NsPerOp(), lz4Result.NsPerOp(), winner)
	}

	fmt.Println()
	fmt.Println("### Decode Performance (ns/op)")
	fmt.Println()
	fmt.Println("| Data Size | S2 Decode | LZ4 Decode | Winner |")
	fmt.Println("|-----------|-----------|------------|--------|")

	for _, size := range sizes {
		if size > len(data) {
			size = len(data)
		}
		chunk := data[:size]

		s2Result := testing.Benchmark(benchS2Decode(chunk))
		lz4Result := testing.Benchmark(benchLZ4Decode(chunk))

		winner := "S2"
		if lz4Result.NsPerOp() < s2Result.NsPerOp() {
			winner = "LZ4"
		}

		fmt.Printf("| %9d | %9d | %10d | %6s |\n",
			size, s2Result.NsPerOp(), lz4Result.NsPerOp(), winner)
	}

	fmt.Println()
	fmt.Println("### Compressed Size (bytes)")
	fmt.Println()
	fmt.Println("| Data Size | S2 Size | LZ4 Size | S2 Ratio | LZ4 Ratio | Winner |")
	fmt.Println("|-----------|---------|----------|----------|-----------|--------|")

	for _, size := range sizes {
		if size > len(data) {
			size = len(data)
		}
		chunk := data[:size]

		s2Compressed := s2.Encode(nil, chunk)
		lz4Compressed := make([]byte, lz4.CompressBlockBound(len(chunk)))
		n, _ := lz4.CompressBlock(chunk, lz4Compressed, nil)
		lz4Compressed = lz4Compressed[:n]

		s2Ratio := float64(len(s2Compressed)) / float64(size) * 100
		lz4Ratio := float64(len(lz4Compressed)) / float64(size) * 100

		winner := "S2"
		if len(lz4Compressed) < len(s2Compressed) {
			winner = "LZ4"
		}

		fmt.Printf("| %9d | %7d | %8d | %7.1f%% | %8.1f%% | %6s |\n",
			size, len(s2Compressed), len(lz4Compressed), s2Ratio, lz4Ratio, winner)
	}

	fmt.Println()
	fmt.Println("### Allocations")
	fmt.Println()
	fmt.Println("| Data Size | S2 Enc Allocs | LZ4 Enc Allocs | S2 Dec Allocs | LZ4 Dec Allocs |")
	fmt.Println("|-----------|---------------|----------------|---------------|----------------|")

	for _, size := range sizes {
		if size > len(data) {
			size = len(data)
		}
		chunk := data[:size]

		s2EncResult := testing.Benchmark(benchS2Encode(chunk))
		lz4EncResult := testing.Benchmark(benchLZ4Encode(chunk))
		s2DecResult := testing.Benchmark(benchS2Decode(chunk))
		lz4DecResult := testing.Benchmark(benchLZ4Decode(chunk))

		fmt.Printf("| %9d | %13d | %14d | %13d | %14d |\n",
			size,
			s2EncResult.AllocsPerOp(), lz4EncResult.AllocsPerOp(),
			s2DecResult.AllocsPerOp(), lz4DecResult.AllocsPerOp())
	}

	fmt.Println()
}

func benchS2Encode(data []byte) func(*testing.B) {
	return func(b *testing.B) {
		for range b.N {
			s2.Encode(nil, data)
		}
	}
}

func benchS2Decode(data []byte) func(*testing.B) {
	compressed := s2.Encode(nil, data)
	return func(b *testing.B) {
		for range b.N {
			s2.Decode(nil, compressed)
		}
	}
}

func benchLZ4Encode(data []byte) func(*testing.B) {
	dst := make([]byte, lz4.CompressBlockBound(len(data)))
	return func(b *testing.B) {
		for range b.N {
			lz4.CompressBlock(data, dst, nil)
		}
	}
}

func benchLZ4Decode(data []byte) func(*testing.B) {
	compressed := make([]byte, lz4.CompressBlockBound(len(data)))
	n, _ := lz4.CompressBlock(data, compressed, nil)
	compressed = compressed[:n]
	dst := make([]byte, len(data))
	return func(b *testing.B) {
		for range b.N {
			lz4.UncompressBlock(compressed, dst)
		}
	}
}
