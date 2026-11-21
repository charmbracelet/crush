package diff

import (
	"sort"
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

// RestoreLineEndings rebuilds the raw bytes of a file after an edit was applied
// to its LF-normalized form. The edit tools read a file as LF so that
// old_string matches whichever convention the file happens to use, but the file
// on disk has to come back with the endings it already had: one CRLF in an
// otherwise LF file must not turn the whole file CRLF, and a line an edit
// introduces must not arrive in a different convention than its neighbours.
//
// raw is the file as it was read and before is the LF-normalized form of it,
// which is the form the edit was applied to; after is the edited LF-normalized
// form. Only the lines that differ between before and after are written anew,
// so every other byte is copied from raw verbatim.
//
// A line the edit introduces takes the ending of the region it lands in: the
// first line of the raw region it replaces, or, for an edit that only inserts,
// the line it follows.
func RestoreLineEndings(raw, before, after string) string {
	if !strings.Contains(raw, "\r\n") {
		// Without a CRLF in the file the normalized form is the file, so the
		// edit already holds the right bytes.
		return after
	}

	index := newLineIndex(raw)
	var out strings.Builder
	out.Grow(len(raw))
	copied := 0
	for _, edit := range udiff.Lines(before, after) {
		start, end := index.rawOffset(edit.Start), index.rawOffset(edit.End)
		out.WriteString(raw[copied:start])
		out.WriteString(reterminate(edit.New, index.endingFor(start, end)))
		copied = end
	}
	out.WriteString(raw[copied:])
	return out.String()
}

// reterminate rewrites the newlines of text to ending. The text is the
// normalized form of the file, except that a new_string can carry a CRLF of its
// own, which is folded into ending rather than doubled up.
func reterminate(text, ending string) string {
	if ending == "\n" {
		return text
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", ending)
}

// lineIndex maps offsets in a file's LF-normalized form back to the raw bytes
// they came from. Normalization only drops the CR of a CRLF, so both forms hold
// the same lines in the same order.
type lineIndex struct {
	raw       string
	rawStart  []int // raw offset of every line start, ending with len(raw)
	normStart []int // offset of the same line start in the normalized form
}

func newLineIndex(raw string) *lineIndex {
	index := &lineIndex{raw: raw}
	rawStart, normStart := 0, 0
	for {
		index.rawStart = append(index.rawStart, rawStart)
		index.normStart = append(index.normStart, normStart)
		if rawStart == len(raw) {
			return index
		}
		rawLen, normLen := lineLengths(raw[rawStart:])
		rawStart, normStart = rawStart+rawLen, normStart+normLen
	}
}

// lineLengths reports the length of the line at the start of s as raw bytes and
// as normalized characters, which differ by the CR of a CRLF ending.
func lineLengths(s string) (raw, norm int) {
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		// An unterminated last line cannot hold a CRLF, so it is the same
		// length in both forms.
		return len(s), len(s)
	}
	if i > 0 && s[i-1] == '\r' {
		return i + 1, i
	}
	return i + 1, i + 1
}

// rawOffset maps an offset in the normalized form to the raw offset it came
// from. An offset inside a line maps to the same character, because
// normalization only removed the CR of a CRLF.
func (index *lineIndex) rawOffset(norm int) int {
	line := sort.Search(len(index.normStart), func(i int) bool {
		return index.normStart[i] > norm
	}) - 1
	return index.rawStart[line] + (norm - index.normStart[line])
}

// endingFor reports the ending a line introduced in place of the raw region
// [start, end) should use, which is the ending of the first line of that region
// and, for a region that holds no ending of its own, of the line before it, then
// of the line after it.
func (index *lineIndex) endingFor(start, end int) string {
	if i := strings.IndexByte(index.raw[start:end], '\n'); i >= 0 {
		return index.endingAt(start + i)
	}
	if i := strings.LastIndexByte(index.raw[:start], '\n'); i >= 0 {
		return index.endingAt(i)
	}
	if i := strings.IndexByte(index.raw[end:], '\n'); i >= 0 {
		return index.endingAt(end + i)
	}
	return "\n"
}

// endingAt reports the terminator of the line whose LF sits at index i.
func (index *lineIndex) endingAt(i int) string {
	if i > 0 && index.raw[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}
