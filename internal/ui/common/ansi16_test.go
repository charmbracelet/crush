package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"
)

func testPalette() [16]color.Color {
	var p [16]color.Color
	for i := range p {
		// Distinct, easy-to-assert colors: R=index, G=0, B=0.
		p[i] = color.RGBA{R: uint8(i), G: 0, B: 0, A: 0xFF}
	}
	return p
}

func TestRemapANSI16(t *testing.T) {
	t.Parallel()

	pal := testPalette()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no escapes passes through",
			in:   "plain text",
			want: "plain text",
		},
		{
			name: "standard foreground red is remapped",
			in:   "\x1b[31mhi\x1b[0m",
			want: "\x1b[38;2;1;0;0mhi\x1b[0m",
		},
		{
			name: "bright foreground red is remapped",
			in:   "\x1b[91mhi\x1b[0m",
			want: "\x1b[38;2;9;0;0mhi\x1b[0m",
		},
		{
			name: "standard background green is remapped",
			in:   "\x1b[42mx\x1b[0m",
			want: "\x1b[48;2;2;0;0mx\x1b[0m",
		},
		{
			name: "bold plus color keeps bold, remaps color",
			in:   "\x1b[1;31mx\x1b[0m",
			want: "\x1b[1;38;2;1;0;0mx\x1b[0m",
		},
		{
			name: "256-color extended is left untouched",
			in:   "\x1b[38;5;196mx\x1b[0m",
			want: "\x1b[38;5;196mx\x1b[0m",
		},
		{
			name: "truecolor extended is left untouched",
			in:   "\x1b[38;2;10;20;30mx\x1b[0m",
			want: "\x1b[38;2;10;20;30mx\x1b[0m",
		},
		{
			name: "non-SGR CSI sequence untouched",
			in:   "\x1b[2J\x1b[31mx",
			want: "\x1b[2J\x1b[38;2;1;0;0mx",
		},
		{
			name: "reset and default fg left as-is",
			in:   "\x1b[0;39mx",
			want: "\x1b[0;39mx",
		},
		{
			name: "underline 256-color extended is left untouched",
			in:   "\x1b[58;5;196mx\x1b[0m",
			want: "\x1b[58;5;196mx\x1b[0m",
		},
		{
			name: "underline truecolor extended is left untouched",
			in:   "\x1b[58;2;10;20;30mx\x1b[0m",
			want: "\x1b[58;2;10;20;30mx\x1b[0m",
		},
		{
			name: "default underline color left as-is",
			in:   "\x1b[59mx",
			want: "\x1b[59mx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, RemapANSI16(tt.in, pal))
		})
	}
}

func TestStripCursorControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text passes through",
			in:   "hello world",
			want: "hello world",
		},
		{
			name: "SGR color sequences preserved",
			in:   "\x1b[31mred\x1b[0m",
			want: "\x1b[31mred\x1b[0m",
		},
		{
			name: "cursor up stripped",
			in:   "line1\n\x1b[Aline2",
			want: "line1\nline2",
		},
		{
			name: "cursor down stripped",
			in:   "top\x1b[Bbottom",
			want: "topbottom",
		},
		{
			name: "erase line stripped",
			in:   "progress\x1b[Kdone",
			want: "progressdone",
		},
		{
			name: "erase display stripped",
			in:   "\x1b[2Jcleared",
			want: "cleared",
		},
		{
			name: "cursor position stripped",
			in:   "\x1b[Hhome",
			want: "home",
		},
		{
			name: "DEC private mode hide cursor stripped",
			in:   "\x1b[?25linvisible\x1b[?25hvisible",
			want: "invisiblevisible",
		},
		{
			name: "DEC save restore cursor stripped",
			in:   "\x1b7saved\x1b8restored",
			want: "savedrestored",
		},
		{
			name: "save restore cursor CSI stripped",
			in:   "\x1b[spos\x1b[upos",
			want: "pospos",
		},
		{
			name: "scroll up down stripped",
			in:   "\x1b[Sscroll\x1b[Tscroll",
			want: "scrollscroll",
		},
		{
			name: "carriage return simulates overwrite",
			in:   "loading...\rdone!",
			want: "done!",
		},
		{
			name: "multiple carriage returns keep last",
			in:   "step1\rstep2\rstep3",
			want: "step3",
		},
		{
			name: "carriage return per line",
			in:   "a\rb\nc\rd",
			want: "b\nd",
		},
		{
			name: "mixed SGR and cursor control",
			in:   "\x1b[31mloading\x1b[0m\r\x1b[32mdone\x1b[0m",
			want: "\x1b[32mdone\x1b[0m",
		},
		{
			name: "git push style progress",
			in:   "Enumerating objects: 10\rEnumerating objects: 50\rEnumerating objects: 100, done.",
			want: "Enumerating objects: 100, done.",
		},
		{
			name: "no escapes fast path",
			in:   "just text no escapes",
			want: "just text no escapes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, StripCursorControl(tt.in))
		})
	}
}

func TestRemapANSI16NilColorFallsBack(t *testing.T) {
	t.Parallel()

	var pal [16]color.Color // all nil
	// With a nil palette entry, the introducer is emitted bare so the
	// terminal default applies rather than crashing.
	require.Equal(t, "\x1b[38mx\x1b[0m", RemapANSI16("\x1b[31mx\x1b[0m", pal))
}
