package tools

import (
	"regexp"
	"testing"
)

func TestRegexCache(t *testing.T) {
	cache := newRegexCache()

	// Test basic caching
	pattern := "test.*pattern"
	regex1, err := cache.get(pattern)
	if err != nil {
		t.Fatalf("Failed to compile regex: %v", err)
	}

	regex2, err := cache.get(pattern)
	if err != nil {
		t.Fatalf("Failed to get cached regex: %v", err)
	}

	// Should be the same instance (cached)
	if regex1 != regex2 {
		t.Error("Expected cached regex to be the same instance")
	}

	// Test that it actually works
	if !regex1.MatchString("test123pattern") {
		t.Error("Regex should match test string")
	}
}

func TestGlobToRegexCaching(t *testing.T) {
	// Test that globToRegex uses pre-compiled regex
	pattern1 := globToRegex("*.{js,ts}")

	// Should not panic and should work correctly
	regex1, err := regexp.Compile(pattern1)
	if err != nil {
		t.Fatalf("Failed to compile glob regex: %v", err)
	}

	if !regex1.MatchString("test.js") {
		t.Error("Glob regex should match .js files")
	}
	if !regex1.MatchString("test.ts") {
		t.Error("Glob regex should match .ts files")
	}
	if regex1.MatchString("test.go") {
		t.Error("Glob regex should not match .go files")
	}
}

// Benchmark to show performance improvement
func BenchmarkRegexCacheVsCompile(b *testing.B) {
	cache := newRegexCache()
	pattern := "test.*pattern.*[0-9]+"

	b.Run("WithCache", func(b *testing.B) {
		for b.Loop() {
			_, err := cache.get(pattern)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithoutCache", func(b *testing.B) {
		for b.Loop() {
			_, err := regexp.Compile(pattern)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
