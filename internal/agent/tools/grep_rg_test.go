package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
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
