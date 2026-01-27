package diffview

import (
	"slices"

	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/x/exp/slice"
)

type splitHunk struct {
	fromLine int
	toLine   int
	lines    []*splitLine
}

type splitLine struct {
	before *udiff.Line
	after  *udiff.Line
}

func hunkToSplit(h *udiff.Hunk) (sh splitHunk) {
	lines := slices.Clone(h.Lines)
	sh = splitHunk{
		fromLine: h.FromLine,
		toLine:   h.ToLine,
		lines:    make([]*splitLine, 0, len(lines)),
	}

	for {
		var ul udiff.Line
		var ok bool
		ul, lines, ok = slice.Shift(lines)
		if !ok {
			break
		}

		var sl splitLine

		switch ul.Kind {
		// For equal lines, add as is
		case udiff.Equal:
			sl.before = &ul
			sl.after = &ul

		// For inserted lines, set after and keep before as nil
		case udiff.Insert:
			sl.before = nil
			sl.after = &ul

		// For deleted lines, set before and loop over the next lines
		// searching for the equivalent after line.
		case udiff.Delete:
			sl.before = &ul

		inner:
			for i, l := range lines {
				switch l.Kind {
				case udiff.Insert:
					var ll udiff.Line
					ll, lines, _ = slice.DeleteAt(lines, i)
					sl.after = &ll
					break inner
				case udiff.Equal:
					break inner
				}
			}
		}

		sh.lines = append(sh.lines, &sl)
	}

	return sh
}
