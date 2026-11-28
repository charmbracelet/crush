package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeFileInfo struct {
	fs.FileInfo
	modTime time.Time
}

func (f fakeFileInfo) ModTime() time.Time { return f.modTime }

func rgEvent(t *testing.T, typ, path string, line int, text string) string {
	t.Helper()
	ev := map[string]any{
		"type": typ,
		"data": map[string]any{
			"path":        map[string]any{"text": path},
			"lines":       map[string]any{"text": text + "\n"},
			"line_number": line,
			"submatches":  []map[string]any{{"start": 2}},
		},
	}
	b, err := json.Marshal(ev)
	require.NoError(t, err)
	return string(b) + "\n"
}

func TestRipgrepCollectorMatchesFullSort(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for seed := range uint64(50) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(seed, seed))

			modTimes := map[string]time.Time{}
			var stream strings.Builder
			var all []grepMatch
			numFiles := rng.IntN(40) + 1
			for f := range numFiles {
				path := fmt.Sprintf("file%d.go", f)
				unreadable := rng.IntN(10) == 0
				// Few distinct mod times so ties are common.
				mt := base.Add(time.Duration(rng.IntN(5)) * time.Hour)
				if !unreadable {
					modTimes[path] = mt
				}
				stream.WriteString(rgEvent(t, "begin", path, 0, ""))
				for l := range rng.IntN(30) + 1 {
					text := fmt.Sprintf("line %d of %s", l+1, path)
					stream.WriteString(rgEvent(t, "match", path, l+1, text))
					if !unreadable {
						all = append(all, grepMatch{
							path: path, modTime: mt, lineNum: l + 1,
							charNum: 3, lineText: text,
						})
					}
				}
				stream.WriteString(rgEvent(t, "end", path, 0, ""))
			}
			stream.WriteString(`{"type":"summary","data":{}}` + "\n")

			stats := 0
			stat := func(p string) (os.FileInfo, error) {
				stats++
				mt, ok := modTimes[p]
				if !ok {
					return nil, os.ErrNotExist
				}
				return fakeFileInfo{modTime: mt}, nil
			}

			for _, limit := range []int{1, 7, 20, 100} {
				stats = 0
				c := newRipgrepCollector(limit, stat)
				require.NoError(t, c.readFrom(strings.NewReader(stream.String())))
				got, truncated := c.results()

				want := append([]grepMatch(nil), all...)
				sort.SliceStable(want, func(i, j int) bool {
					return want[i].modTime.After(want[j].modTime)
				})
				wantTruncated := len(want) > limit
				if wantTruncated {
					want = want[:limit]
				}

				require.Equal(t, want, got, "limit %d", limit)
				require.Equal(t, wantTruncated, truncated, "limit %d", limit)
				require.Equal(t, numFiles, stats, "each file is stat'ed once")
			}
		})
	}
}

func TestRipgrepCollectorSkipsOversizedLines(t *testing.T) {
	t.Parallel()

	stat := func(string) (os.FileInfo, error) {
		return fakeFileInfo{modTime: time.Unix(1, 0)}, nil
	}
	huge := strings.Repeat("x", 200*1024)
	stream := rgEvent(t, "match", "a.go", 1, "small one") +
		rgEvent(t, "match", "a.go", 2, huge) +
		rgEvent(t, "match", "a.go", 3, "small two")

	c := newRipgrepCollector(100, stat)
	br := bufio.NewReaderSize(strings.NewReader(stream), 16)
	var lines [][]byte
	for {
		line, tooLong, err := readBoundedLine(br, nil, 100*1024)
		if !tooLong {
			lines = append(lines, line)
		}
		if err != nil {
			break
		}
	}
	for _, l := range lines {
		c.addLine(l)
	}
	got, truncated := c.results()
	require.False(t, truncated)
	require.Len(t, got, 2)
	require.Equal(t, 1, got[0].lineNum)
	require.Equal(t, 3, got[1].lineNum)

	// Long lines under the cap are kept, but truncated for display.
	c = newRipgrepCollector(100, stat)
	require.NoError(t, c.readFrom(strings.NewReader(stream)))
	got, _ = c.results()
	require.Len(t, got, 3)
	require.Equal(t, 2, got[1].lineNum)
	require.Equal(t, maxGrepContentWidth, len(got[1].lineText))
	require.True(t, strings.HasSuffix(got[1].lineText, "..."))
}

// TestRipgrepCollectorInterleavedFiles checks that the collector, which
// only remembers kept files and the current one, still returns the full
// sort's result when a file's matches are split across the stream.
func TestRipgrepCollectorInterleavedFiles(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for seed := range uint64(50) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(seed, seed))

			// Distinct mod times, since a full sort interleaves the
			// matches of tied files while the collector groups them.
			numFiles := rng.IntN(20) + 1
			modTimes := map[string]time.Time{}
			remaining := make([]int, numFiles)
			minutes := rng.Perm(1000)
			for f := range numFiles {
				path := fmt.Sprintf("file%d.go", f)
				if rng.IntN(10) != 0 {
					modTimes[path] = base.Add(time.Duration(minutes[f]) * time.Minute)
				}
				remaining[f] = rng.IntN(15) + 1
			}

			var stream strings.Builder
			var all []grepMatch
			next := make([]int, numFiles)
			for {
				var open []int
				for f, n := range remaining {
					if n > 0 {
						open = append(open, f)
					}
				}
				if len(open) == 0 {
					break
				}
				f := open[rng.IntN(len(open))]
				path := fmt.Sprintf("file%d.go", f)
				for range min(remaining[f], rng.IntN(4)+1) {
					next[f]++
					remaining[f]--
					text := fmt.Sprintf("line %d of %s", next[f], path)
					stream.WriteString(rgEvent(t, "match", path, next[f], text))
					if mt, ok := modTimes[path]; ok {
						all = append(all, grepMatch{
							path: path, modTime: mt, lineNum: next[f],
							charNum: 3, lineText: text,
						})
					}
				}
			}

			stat := func(p string) (os.FileInfo, error) {
				mt, ok := modTimes[p]
				if !ok {
					return nil, os.ErrNotExist
				}
				return fakeFileInfo{modTime: mt}, nil
			}

			for _, limit := range []int{1, 7, 20, 100} {
				c := newRipgrepCollector(limit, stat)
				require.NoError(t, c.readFrom(strings.NewReader(stream.String())))
				got, truncated := c.results()

				want := append([]grepMatch{}, all...)
				sort.SliceStable(want, func(i, j int) bool {
					return want[i].modTime.After(want[j].modTime)
				})
				wantTruncated := len(want) > limit
				if wantTruncated {
					want = want[:limit]
				}

				require.Equal(t, want, got, "limit %d", limit)
				require.Equal(t, wantTruncated, truncated, "limit %d", limit)
				require.LessOrEqual(t, len(c.files), limit+1, "only kept files are remembered")
			}
		})
	}
}

// testRgPath returns the ripgrep binary on $PATH, or "" when there is none.
// getRg always reports no ripgrep under tests, so tests that need to run
// the real binary pass this path explicitly.
var testRgPath = sync.OnceValue(func() string {
	path, err := exec.LookPath("rg")
	if err != nil {
		return ""
	}
	return path
})

func TestSearchWithRipgrepBinary(t *testing.T) {
	t.Parallel()

	rg := testRgPath()
	if rg == "" {
		t.Skip("rg is not in $PATH")
	}

	// Files are listed oldest first, with the number of matching lines in
	// each. Ignored files hold matches that must never be returned.
	type file struct {
		name    string
		matches int
		ignored bool
	}
	files := []file{
		{name: "a.txt", matches: 3},
		{name: "b.txt", matches: 1},
		{name: "ignored_git.txt", matches: 5, ignored: true},
		{name: "sub/c.txt", matches: 4},
		{name: "ignored_crush.txt", matches: 5, ignored: true},
		{name: "d.txt", matches: 2},
		{name: "big.txt", matches: 12},
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored_git.txt\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".crushignore"), []byte("ignored_crush.txt\n"), 0o644))

	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	var all []grepMatch
	for i, f := range files {
		path := filepath.Join(dir, filepath.FromSlash(f.name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		var content strings.Builder
		var lines []string
		for m := range f.matches {
			content.WriteString("filler line\n")
			text := fmt.Sprintf("x needle %d in %s", m, f.name)
			content.WriteString(text + "\n")
			lines = append(lines, text)
		}
		require.NoError(t, os.WriteFile(path, []byte(content.String()), 0o644))
		mt := base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, os.Chtimes(path, mt, mt))
		if f.ignored {
			continue
		}
		fi, err := os.Stat(path)
		require.NoError(t, err)
		for m, text := range lines {
			all = append(all, grepMatch{
				path:     path,
				modTime:  fi.ModTime(),
				lineNum:  2 * (m + 1),
				charNum:  3,
				lineText: text,
			})
		}
	}

	// The reference is the pre-streaming behavior: every match, stably
	// sorted newest file first, then cut to the limit.
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].modTime.After(all[j].modTime)
	})
	total := len(all)

	for _, limit := range []int{1, 5, 12, 13, 16, total - 1, total, total + 10} {
		t.Run(fmt.Sprintf("limit %d", limit), func(t *testing.T) {
			t.Parallel()

			got, truncated, err := searchWithRipgrepBinary(t.Context(), rg, "needle", dir, "", limit)
			require.NoError(t, err)

			want := all
			if len(want) > limit {
				want = want[:limit]
			}
			require.Equal(t, want, got)
			require.Equal(t, total > limit, truncated)
		})
	}

	t.Run("one file over the limit", func(t *testing.T) {
		t.Parallel()

		// big.txt alone has 12 matches, so only --max-count limit+1
		// leaves ripgrep enough to know more were found.
		got, truncated, err := searchWithRipgrepBinary(t.Context(), rg, "needle", dir, "big.txt", 4)
		require.NoError(t, err)
		require.Len(t, got, 4)
		require.True(t, truncated)
		for i, m := range got {
			require.Equal(t, filepath.Join(dir, "big.txt"), m.path)
			require.Equal(t, 2*(i+1), m.lineNum)
		}
	})

	t.Run("exactly the limit", func(t *testing.T) {
		t.Parallel()

		got, truncated, err := searchWithRipgrepBinary(t.Context(), rg, "needle", dir, "big.txt", 12)
		require.NoError(t, err)
		require.Len(t, got, 12)
		require.False(t, truncated)
	})

	t.Run("no matches", func(t *testing.T) {
		t.Parallel()

		got, truncated, err := searchWithRipgrepBinary(t.Context(), rg, "no_such_text_anywhere", dir, "", 10)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
		require.False(t, truncated)
	})

	t.Run("ripgrep error", func(t *testing.T) {
		t.Parallel()

		// An invalid regex makes ripgrep exit with status 2, which must be
		// reported so the caller falls back to the regex search.
		_, _, err := searchWithRipgrepBinary(t.Context(), rg, "(", dir, "", 10)
		require.Error(t, err)
	})

	t.Run("missing binary", func(t *testing.T) {
		t.Parallel()

		_, _, err := searchWithRipgrepBinary(t.Context(), "", "needle", dir, "", 10)
		require.Error(t, err)
	})
}
