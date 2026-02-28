package util

import (
	"testing"

	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

func TestPositionToByteOffset(t *testing.T) {
	tests := []struct {
		name      string
		lineText  string
		utf16Char uint32
		expected  int
	}{
		{
			name:      "ASCII only",
			lineText:  "hello world",
			utf16Char: 6,
			expected:  6,
		},
		{
			name:      "CJK characters (3 bytes each in UTF-8, 1 UTF-16 unit)",
			lineText:  "你好world",
			utf16Char: 2,
			expected:  6,
		},
		{
			name:      "CJK - position after CJK",
			lineText:  "var x = \"你好world\"",
			utf16Char: 11,
			expected:  15,
		},
		{
			name:      "Emoji (4 bytes in UTF-8, 2 UTF-16 units)",
			lineText:  "👋hello",
			utf16Char: 2,
			expected:  4,
		},
		{
			name:      "Multiple emoji",
			lineText:  "👋👋world",
			utf16Char: 4,
			expected:  8,
		},
		{
			name:      "Mixed content",
			lineText:  "Hello👋你好",
			utf16Char: 8,
			expected:  12,
		},
		{
			name:      "Position 0",
			lineText:  "hello",
			utf16Char: 0,
			expected:  0,
		},
		{
			name:      "Position beyond end",
			lineText:  "hi",
			utf16Char: 100,
			expected:  2,
		},
		{
			name:      "Empty string",
			lineText:  "",
			utf16Char: 0,
			expected:  0,
		},
		{
			name:      "Surrogate pair at start",
			lineText:  "𐐷hello",
			utf16Char: 2,
			expected:  4,
		},
		{
			name:      "ZWJ family emoji (1 grapheme, 7 runes, 11 UTF-16 units)",
			lineText:  "hello👨\u200d👩\u200d👧\u200d👦world",
			utf16Char: 16,
			expected:  30,
		},
		{
			name:      "ZWJ family emoji - offset into middle of grapheme cluster",
			lineText:  "hello👨\u200d👩\u200d👧\u200d👦world",
			utf16Char: 8,
			expected:  12,
		},
		{
			name:      "Flag emoji (1 grapheme, 2 runes, 4 UTF-16 units)",
			lineText:  "hello🇺🇸world",
			utf16Char: 9,
			expected:  13,
		},
		{
			name:      "Combining character (1 grapheme, 2 runes, 2 UTF-16 units)",
			lineText:  "caf\u0065\u0301!",
			utf16Char: 5,
			expected:  6,
		},
		{
			name:      "Skin tone modifier (1 grapheme, 2 runes, 4 UTF-16 units)",
			lineText:  "hi👋🏽bye",
			utf16Char: 6,
			expected:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := positionToByteOffset(tt.lineText, tt.utf16Char)
			if result != tt.expected {
				t.Errorf("positionToByteOffset(%q, %d) = %d, want %d",
					tt.lineText, tt.utf16Char, result, tt.expected)
			}
		})
	}
}

func TestApplyTextEdit_UTF16(t *testing.T) {
	// Test that UTF-16 offsets are correctly converted to byte offsets
	tests := []struct {
		name     string
		lines    []string
		edit     protocol.TextEdit
		expected []string
	}{
		{
			name:  "ASCII only - no conversion needed",
			lines: []string{"hello world"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 6},
					End:   protocol.Position{Line: 0, Character: 11},
				},
				NewText: "universe",
			},
			expected: []string{"hello universe"},
		},
		{
			name:  "CJK characters - edit after Chinese characters",
			lines: []string{`var x = "你好world"`},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					// "你好" = 2 UTF-16 units, but 6 bytes in UTF-8
					// Position 11 is where "world" starts in UTF-16
					Start: protocol.Position{Line: 0, Character: 11},
					End:   protocol.Position{Line: 0, Character: 16},
				},
				NewText: "universe",
			},
			expected: []string{`var x = "你好universe"`},
		},
		{
			name:  "Emoji - edit after emoji (2 UTF-16 units)",
			lines: []string{`fmt.Println("👋hello")`},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					// 👋 = 2 UTF-16 units, 4 bytes in UTF-8
					// Position 15 is where "hello" starts in UTF-16
					Start: protocol.Position{Line: 0, Character: 15},
					End:   protocol.Position{Line: 0, Character: 20},
				},
				NewText: "world",
			},
			expected: []string{`fmt.Println("👋world")`},
		},
		{
			name: "ZWJ family emoji - edit after grapheme cluster",
			// "hello👨‍👩‍👧‍👦world" — family is 1 grapheme but 11 UTF-16 units
			lines: []string{"hello\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466world"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					// "hello" = 5 UTF-16 units, family = 11 UTF-16 units
					// "world" starts at UTF-16 offset 16
					Start: protocol.Position{Line: 0, Character: 16},
					End:   protocol.Position{Line: 0, Character: 21},
				},
				NewText: "earth",
			},
			expected: []string{"hello\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466earth"},
		},
		{
			name: "ZWJ family emoji - edit splits grapheme cluster in half",
			// LSP servers can position into the middle of a grapheme cluster.
			// After "hello" (5 UTF-16 units), the ZWJ family emoji starts.
			// UTF-16 offset 7 lands between 👨 (2 units) and ZWJ, inside
			// the grapheme cluster. The byte offset for position 7 is 9
			// (5 bytes for "hello" + 4 bytes for 👨).
			lines: []string{"hello\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466world"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 7},
					End:   protocol.Position{Line: 0, Character: 16},
				},
				NewText: "",
			},
			// Keeps "hello" + 👨 (first rune of cluster) then removes
			// the rest of the cluster, leaving "hello👨world".
			expected: []string{"hello\U0001F468world"},
		},
		{
			name: "Flag emoji - edit after flag",
			// 🇺🇸 = 2 regional indicator runes, 4 UTF-16 units, 8 bytes
			lines: []string{"hello🇺🇸world"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 9},
					End:   protocol.Position{Line: 0, Character: 14},
				},
				NewText: "earth",
			},
			expected: []string{"hello🇺🇸earth"},
		},
		{
			name: "Combining accent - edit after composed character",
			// "café!" where é = e + U+0301 (2 code points, 2 UTF-16 units)
			lines: []string{"caf\u0065\u0301!"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					// "caf" = 3, "e" = 1, U+0301 = 1, total = 5 UTF-16 units
					Start: protocol.Position{Line: 0, Character: 5},
					End:   protocol.Position{Line: 0, Character: 6},
				},
				NewText: "?",
			},
			expected: []string{"caf\u0065\u0301?"},
		},
		{
			name: "Skin tone modifier - edit after modified emoji",
			// 👋🏽 = U+1F44B U+1F3FD = 2 runes, 4 UTF-16 units, 8 bytes
			lines: []string{"hi👋🏽bye"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					// "hi" = 2, 👋🏽 = 4, total = 6 UTF-16 units
					Start: protocol.Position{Line: 0, Character: 6},
					End:   protocol.Position{Line: 0, Character: 9},
				},
				NewText: "later",
			},
			expected: []string{"hi👋🏽later"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyTextEdit(tt.lines, tt.edit, powernap.UTF16)
			if err != nil {
				t.Fatalf("applyTextEdit failed: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestApplyTextEdit_UTF8(t *testing.T) {
	// Test that UTF-8 offsets are used directly without conversion
	tests := []struct {
		name     string
		lines    []string
		edit     protocol.TextEdit
		expected []string
	}{
		{
			name:  "ASCII only - direct byte offset",
			lines: []string{"hello world"},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 6},
					End:   protocol.Position{Line: 0, Character: 11},
				},
				NewText: "universe",
			},
			expected: []string{"hello universe"},
		},
		{
			name:  "CJK characters - byte offset used directly",
			lines: []string{`var x = "你好world"`},
			edit: protocol.TextEdit{
				Range: protocol.Range{
					// With UTF-8 encoding, position 15 is the byte offset
					Start: protocol.Position{Line: 0, Character: 15},
					End:   protocol.Position{Line: 0, Character: 20},
				},
				NewText: "universe",
			},
			expected: []string{`var x = "你好universe"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyTextEdit(tt.lines, tt.edit, powernap.UTF8)
			if err != nil {
				t.Fatalf("applyTextEdit failed: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}
