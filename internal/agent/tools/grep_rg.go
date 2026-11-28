package tools

import (
	"bufio"
	"bytes"
	"container/heap"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// maxRipgrepJSONLine bounds a single line of ripgrep's JSON output. Longer
// lines (e.g. matches inside huge minified files) are skipped instead of
// being buffered.
const maxRipgrepJSONLine = 32 * 1024 * 1024

// ripgrepCollector keeps the newest limit matches, ordered by file
// modification time, out of a ripgrep --json stream. It returns the same
// matches as stably sorting every match by modification time and keeping
// the first limit, but holds only O(limit) matches in memory and stats
// each file once.
type ripgrepCollector struct {
	limit int
	total int
	kept  int
	files map[string]*rgFileGroup
	heap  rgFileHeap
	stat  func(string) (os.FileInfo, error)
}

// rgFileGroup holds the kept matches of a single file.
type rgFileGroup struct {
	order      int // Order in which the file was first seen.
	modTime    time.Time
	unreadable bool // Stat failed, so its matches are ignored.
	evicted    bool // Dropped from the result; matches are only counted.
	matches    []grepMatch
}

// worse reports whether g sorts after o in the result: older files come
// later, and files with the same modification time keep their stream
// order.
func (g *rgFileGroup) worse(o *rgFileGroup) bool {
	if !g.modTime.Equal(o.modTime) {
		return g.modTime.Before(o.modTime)
	}
	return g.order > o.order
}

// rgFileHeap is a min-heap with the worst kept file at its root.
type rgFileHeap []*rgFileGroup

func (h rgFileHeap) Len() int           { return len(h) }
func (h rgFileHeap) Less(i, j int) bool { return h[i].worse(h[j]) }
func (h rgFileHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *rgFileHeap) Push(x any)        { *h = append(*h, x.(*rgFileGroup)) }

func (h *rgFileHeap) Pop() any {
	old := *h
	n := len(old)
	g := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return g
}

func newRipgrepCollector(limit int, stat func(string) (os.FileInfo, error)) *ripgrepCollector {
	return &ripgrepCollector{
		limit: limit,
		files: make(map[string]*rgFileGroup),
		stat:  stat,
	}
}

// readFrom consumes a ripgrep --json stream until EOF.
func (c *ripgrepCollector) readFrom(r io.Reader) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var buf []byte
	for {
		line, tooLong, err := readBoundedLine(br, buf[:0], maxRipgrepJSONLine)
		buf = line
		if !tooLong {
			c.addLine(bytes.TrimSpace(line))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// readBoundedLine reads one newline-terminated line into buf. A line
// longer than maxLen is consumed and discarded, and tooLong is set.
func readBoundedLine(r *bufio.Reader, buf []byte, maxLen int) ([]byte, bool, error) {
	tooLong := false
	for {
		frag, err := r.ReadSlice('\n')
		if !tooLong {
			if len(buf)+len(frag) > maxLen {
				tooLong = true
				buf = buf[:0]
			} else {
				buf = append(buf, frag...)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return buf, tooLong, err
	}
}

func (c *ripgrepCollector) addLine(line []byte) {
	if len(line) == 0 {
		return
	}
	var match ripgrepMatch
	if err := json.Unmarshal(line, &match); err != nil {
		return
	}
	if match.Type != "match" || len(match.Data.Submatches) == 0 {
		return
	}
	// Only get the first match of each line.
	c.add(grepMatch{
		path:     match.Data.Path.Text,
		lineNum:  match.Data.LineNumber,
		charNum:  match.Data.Submatches[0].Start + 1, // Ensure 1-based.
		lineText: truncateGrepLine(strings.TrimSpace(match.Data.Lines.Text)),
	})
}

func (c *ripgrepCollector) add(m grepMatch) {
	g, ok := c.files[m.path]
	if !ok {
		g = &rgFileGroup{order: len(c.files)}
		c.files[m.path] = g
		if fi, err := c.stat(m.path); err != nil {
			g.unreadable = true // Skip files we can't access.
		} else {
			g.modTime = fi.ModTime()
		}
	}
	if g.unreadable {
		return
	}
	c.total++
	// An evicted file already had at least limit better matches ahead of
	// it, and those are only ever replaced by better ones.
	if g.evicted {
		return
	}

	m.modTime = g.modTime
	if len(g.matches) == 0 {
		heap.Push(&c.heap, g)
	}
	g.matches = append(g.matches, m)
	c.kept++

	// Drop the worst files while the remaining ones still fill the limit.
	for c.heap.Len() > 0 && c.kept-len(c.heap[0].matches) >= c.limit {
		worst := heap.Pop(&c.heap).(*rgFileGroup)
		c.kept -= len(worst.matches)
		worst.matches = nil
		worst.evicted = true
	}
}

// results returns the kept matches, newest file first, and whether any
// matches were left out.
func (c *ripgrepCollector) results() ([]grepMatch, bool) {
	groups := slices.Clone(c.heap)
	slices.SortFunc(groups, func(a, b *rgFileGroup) int {
		switch {
		case b.worse(a):
			return -1
		case a.worse(b):
			return 1
		default:
			return 0
		}
	})

	matches := make([]grepMatch, 0, c.kept)
	for _, g := range groups {
		matches = append(matches, g.matches...)
	}
	limit := max(c.limit, 0)
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, c.total > limit
}

// truncateGrepLine shortens a matched line to the width the grep tool
// displays, so oversized lines are not held in memory. A string's width
// never exceeds its length in bytes, so short lines skip measuring.
func truncateGrepLine(lineText string) string {
	if len(lineText) > maxGrepContentWidth && ansi.StringWidth(lineText) > maxGrepContentWidth {
		return ansi.Truncate(lineText, maxGrepContentWidth, "...")
	}
	return lineText
}
