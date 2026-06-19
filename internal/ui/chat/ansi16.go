package chat

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// remapANSI16 replaces basic ANSI 16-color SGR codes with 24-bit
// truecolor from palette. Programs emit \x1b[31m etc. and trust the
// terminal to pick the color; inside Crush's TUI those defaults are
// often illegible on our dark background. Rewriting them to explicit
// RGB keeps output readable regardless of terminal configuration.
//
// Uses [ansi.DecodeSequence] for parsing (same approach as
// [colorprofile.Writer]) since there is no upstream palette-remap API.
func remapANSI16(s string, palette [16]color.Color) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}

	var buf strings.Builder
	buf.Grow(len(s))

	parser := ansi.GetParser()
	defer ansi.PutParser(parser)

	var state byte
	for len(s) > 0 {
		parser.Reset()
		seq, _, n, newState := ansi.DecodeSequence(s, state, parser)

		if ansi.HasCsiPrefix(seq) && parser.Command() == 'm' {
			remapSGR(parser.Params(), palette, &buf)
		} else {
			buf.WriteString(seq)
		}

		s = s[n:]
		state = newState
	}

	return buf.String()
}

// remapSGR rewrites one SGR sequence, replacing 16-color params with
// truecolor from palette. Extended colors (38/48/58 with ;5;n or
// ;2;r;g;b sub-params) pass through unchanged. Non-color attributes
// (bold, italic, etc.) and default color resets (39/49/59) also pass
// through.
func remapSGR(params ansi.Params, palette [16]color.Color, buf *strings.Builder) {
	buf.WriteString("\x1b[")

	first := true
	for i := 0; i < len(params); i++ {
		p := params[i].Param(0)

		if !first {
			buf.WriteByte(';')
		}
		first = false

		switch {
		// Extended color introducers consume subsequent params as
		// arguments. Skip them whole so they aren't misread.
		case p == 38 || p == 48 || p == 58:
			buf.WriteString(strconv.Itoa(p))
			if i+1 < len(params) {
				sub := params[i+1].Param(0)
				switch sub {
				case 5: // 256-color: 38;5;n
					buf.WriteByte(';')
					buf.WriteString(strconv.Itoa(sub))
					if i+2 < len(params) {
						buf.WriteByte(';')
						buf.WriteString(strconv.Itoa(params[i+2].Param(0)))
						i += 2
					} else {
						i++
					}
				case 2: // truecolor: 38;2;r;g;b
					buf.WriteByte(';')
					buf.WriteString(strconv.Itoa(sub))
					for j := 2; j <= 4 && i+j < len(params); j++ {
						buf.WriteByte(';')
						buf.WriteString(strconv.Itoa(params[i+j].Param(0)))
					}
					i += min(4, len(params)-i-1)
				default:
					i++
				}
			}

		case p >= 30 && p <= 37:
			writeTruecolor(buf, 38, palette[p-30])
		case p >= 90 && p <= 97:
			writeTruecolor(buf, 38, palette[8+p-90])
		case p >= 40 && p <= 47:
			writeTruecolor(buf, 48, palette[p-40])
		case p >= 100 && p <= 107:
			writeTruecolor(buf, 48, palette[8+p-100])

		default:
			buf.WriteString(strconv.Itoa(p))
		}
	}

	buf.WriteByte('m')
}

// writeTruecolor appends "introducer;2;r;g;b" to buf. Nil color emits
// the bare introducer so the terminal default applies.
func writeTruecolor(buf *strings.Builder, introducer int, c color.Color) {
	if c == nil {
		buf.WriteString(strconv.Itoa(introducer))
		return
	}
	r, g, b, _ := c.RGBA()
	buf.WriteString(strconv.Itoa(introducer))
	buf.WriteString(";2;")
	buf.WriteString(strconv.Itoa(int(r >> 8)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(g >> 8)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(b >> 8)))
}
