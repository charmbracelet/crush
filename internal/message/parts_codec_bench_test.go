package message

import (
	"fmt"
	"strings"
	"testing"
)

// benchParts builds serialized parts of roughly the target size by
// repeating realistic text tool-result content.
func benchParts(b *testing.B, targetBytes int) []byte {
	b.Helper()
	unit := strings.Repeat("line of file content that a tool result would carry\n", 10) // ~590B
	n := targetBytes/len(unit) + 1
	parts := make([]ContentPart, 0, n)
	for range n {
		parts = append(parts, TextContent{Text: unit})
	}
	data, err := marshalParts(parts)
	if err != nil {
		b.Fatal(err)
	}
	return data
}

type benchSize struct {
	name string
	size int
}

func benchmarkSizes() []benchSize {
	return []benchSize{
		{"1KB", 1 << 10},
		{"16KB", 16 << 10},   // typical assistant message with a few tool results
		{"256KB", 256 << 10}, // large streamed message (issue #3579's rewrite path)
	}
}

func BenchmarkMarshalParts(b *testing.B) {
	for _, s := range benchmarkSizes() {
		parts := []ContentPart{TextContent{Text: strings.Repeat("x", s.size)}}
		b.Run(s.name, func(b *testing.B) {
			b.SetBytes(int64(s.size))
			for b.Loop() {
				if _, err := marshalParts(parts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompressParts(b *testing.B) {
	for _, s := range benchmarkSizes() {
		data := benchParts(b, s.size)
		b.Run(s.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				compressParts(data)
			}
		})
		b.Run(fmt.Sprintf("%s/ratio", s.name), func(b *testing.B) {
			stored := compressParts(data)
			b.ReportMetric(float64(len(stored))/float64(len(data)), "ratio")
			for b.Loop() {
			}
		})
	}
}

func BenchmarkDecompressParts(b *testing.B) {
	for _, s := range benchmarkSizes() {
		data := benchParts(b, s.size)
		stored := compressParts(data)
		b.Run(s.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				if _, err := decompressPartsIfStored(stored); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkUnmarshalParts(b *testing.B) {
	for _, s := range benchmarkSizes() {
		data := benchParts(b, s.size)
		b.Run(s.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				if _, err := unmarshalParts(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLegacyJSONPassthrough measures the sniff cost paid by rows
// written before compression shipped.
func BenchmarkLegacyJSONPassthrough(b *testing.B) {
	data := benchParts(b, 16<<10)
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := decompressPartsIfStored(data); err != nil {
			b.Fatal(err)
		}
	}
}
