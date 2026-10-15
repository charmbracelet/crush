package provider

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
)

// D1: which invisible / non-printing code points survive displaySafe?
func TestDisplaySafeHidesCombiningSurvivors(t *testing.T) {
	cands := []struct {
		name string
		r    rune
	}{
		{"VS1 U+FE00", 0xFE00}, {"VS15 U+FE0E", 0xFE0E}, {"VS16 U+FE0F", 0xFE0F},
		{"VS17 U+E0100", 0xE0100}, {"VS256 U+E01EF", 0xE01EF},
		{"CGJ U+034F", 0x034F}, {"Khmer inherent AQ U+17B4", 0x17B4}, {"Khmer inherent AA U+17B5", 0x17B5},
		{"Mongolian FVS1 U+180B", 0x180B}, {"Mongolian FVS3 U+180D", 0x180D}, {"Mongolian FVS4 U+180F", 0x180F},
		{"ZWSP U+200B", 0x200B}, {"WJ U+2060", 0x2060}, {"BOM U+FEFF", 0xFEFF}, {"SHY U+00AD", 0x00AD},
		{"RLO U+202E", 0x202E}, {"LRI U+2066", 0x2066}, {"TAG A U+E0041", 0xE0041},
		{"Hangul filler U+3164", 0x3164}, {"Hangul choseong filler U+115F", 0x115F}, {"half Hangul filler U+FFA0", 0xFFA0},
		{"Braille blank U+2800", 0x2800}, {"CR", '\r'}, {"ESC", 0x1b}, {"CSI U+9B", 0x9b}, {"DEL", 0x7f},
		{"NBSP U+00A0", 0x00A0}, {"EM SPACE U+2003", 0x2003}, {"IDEOGRAPHIC SPACE U+3000", 0x3000},
		{"OGHAM SPACE U+1680", 0x1680}, {"MUSICAL NULL NOTEHEAD U+1D159", 0x1D159},
		{"Combining enclosing keycap U+20E3", 0x20E3}, {"Kaithi number sign U+110BD", 0x110BD},
		{"Hangul Jungseong filler U+1160", 0x1160}, {"Arabic letter mark U+061C", 0x061C},
		{"Tifinagh/ancient: U+2D7F tifinagh consonant joiner", 0x2D7F}, {"Variation selector-4 U+180F", 0x180F},
		{"Lao/Thai? U+0E31 (visible mark)", 0x0E31}, {"Interlinear anchor U+FFF9", 0xFFF9},
	}
	var survivors []string
	for _, c := range cands {
		in := "A" + string(c.r) + "B"
		out := displaySafe(in)
		if out == in {
			survivors = append(survivors, fmt.Sprintf("%s (cat Mn=%v Mc=%v Me=%v Cf=%v Zs=%v So=%v)", c.name,
				unicode.Is(unicode.Mn, c.r), unicode.Is(unicode.Mc, c.r), unicode.Is(unicode.Me, c.r), unicode.Is(unicode.Cf, c.r), unicode.Is(unicode.Zs, c.r), unicode.Is(unicode.So, c.r)))
		}
	}
	t.Logf("code points that pass through displaySafe unchanged:\n  %s", strings.Join(survivors, "\n  "))
	// hidden-payload demo: a visible emoji carrying 5 invisible bytes as variation selectors
	payload := "ok 🙂"
	for _, b := range []byte("rm -r") {
		if b < 16 {
			payload += string(rune(0xFE00 + int(b)))
		} else {
			payload += string(rune(0xE0100 + int(b) - 16))
		}
	}
	out := displaySafe(payload)
	t.Logf("payload runes in=%d out=%d unchanged=%v printed=%q", len([]rune(payload)), len([]rune(out)), out == payload, out)
}
