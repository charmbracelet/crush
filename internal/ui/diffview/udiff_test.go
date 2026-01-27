package diffview_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/x/exp/golden"
)

func TestUdiff(t *testing.T) {
	before := `package main

	import (
		"fmt"
	)

	func main() {
		fmt.Println("Hello, World!")
	}`

	after := `package main

	import (
		"fmt"
	)

	func main() {
		content := "Hello, World!"
		fmt.Println(content)
	}`

	t.Run("Unified", func(t *testing.T) {
		content := udiff.Unified("main.go", "main.go", before, after)
		golden.RequireEqual(t, []byte(content))
	})

	t.Run("ToUnifiedDiff", func(t *testing.T) {
		toUnifiedDiff := func(t *testing.T, before, after string, contextLines int) udiff.UnifiedDiff {
			edits := udiff.Strings(before, after)
			unifiedDiff, err := udiff.ToUnifiedDiff("main.go", "main.go", before, edits, contextLines)
			if err != nil {
				t.Fatalf("ToUnifiedDiff failed: %v", err)
			}
			return unifiedDiff
		}
		toJSON := func(t *testing.T, unifiedDiff udiff.UnifiedDiff) []byte {
			var buff bytes.Buffer
			encoder := json.NewEncoder(&buff)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(unifiedDiff); err != nil {
				t.Fatalf("Failed to encode unified diff: %v", err)
			}
			return buff.Bytes()
		}

		t.Run("DefaultContextLines", func(t *testing.T) {
			unifiedDiff := toUnifiedDiff(t, before, after, udiff.DefaultContextLines)

			t.Run("Content", func(t *testing.T) {
				golden.RequireEqual(t, []byte(unifiedDiff.String()))
			})
			t.Run("JSON", func(t *testing.T) {
				golden.RequireEqual(t, toJSON(t, unifiedDiff))
			})
		})

		t.Run("DefaultContextLinesPlusOne", func(t *testing.T) {
			unifiedDiff := toUnifiedDiff(t, before, after, udiff.DefaultContextLines+1)

			t.Run("Content", func(t *testing.T) {
				golden.RequireEqual(t, []byte(unifiedDiff.String()))
			})
			t.Run("JSON", func(t *testing.T) {
				golden.RequireEqual(t, toJSON(t, unifiedDiff))
			})
		})

		t.Run("DefaultContextLinesPlusTwo", func(t *testing.T) {
			unifiedDiff := toUnifiedDiff(t, before, after, udiff.DefaultContextLines+2)

			t.Run("Content", func(t *testing.T) {
				golden.RequireEqual(t, []byte(unifiedDiff.String()))
			})
			t.Run("JSON", func(t *testing.T) {
				golden.RequireEqual(t, toJSON(t, unifiedDiff))
			})
		})
	})
}
