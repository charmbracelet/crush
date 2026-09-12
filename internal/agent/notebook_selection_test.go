package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/notebook"
	"github.com/stretchr/testify/require"
)

func nbEntry(id string, turn, event int64, eventType, text string, tokens int64, tags ...string) notebook.Entry {
	return nbEntryResult(id, turn, event, eventType, text, tokens, true, tags...)
}

// nbSegEntry is nbEntry with a non-zero segment number — the
// intra-turn ordering arm of selection only engages when entries
// carry distinct segments.
func nbSegEntry(id string, turn, segment, event int64, eventType, text string, tokens int64, tags ...string) notebook.Entry {
	e := nbEntry(id, turn, event, eventType, text, tokens, tags...)
	e.SegmentNumber = segment
	return e
}

func nbEntryResult(id string, turn, event int64, eventType, text string, tokens int64, succeeded bool, tags ...string) notebook.Entry {
	return notebook.Entry{
		ID:          id,
		TurnNumber:  turn,
		EventNumber: event,
		EventType:   eventType,
		EntryText:   text,
		TokenCount:  tokens,
		Tags:        tags,
		Succeeded:   succeeded,
	}
}

func entryIDs(entries []notebook.Entry) []string {
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestFormatTurnRanges(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", formatTurnRanges(nil))
	require.Equal(t, "3", formatTurnRanges([]int64{3}))
	require.Equal(t, "3-5", formatTurnRanges([]int64{3, 4, 5}))
	require.Equal(t, "3-5, 9, 11-12", formatTurnRanges([]int64{3, 4, 5, 9, 11, 12}))
	require.Equal(t, "1, 3", formatTurnRanges([]int64{1, 3}))
}

func TestSelectNotebookEntries(t *testing.T) {
	t.Parallel()

	t.Run("path relevance selects tagged entries", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("e1", 1, 1, notebook.EventFileRead, "read auth.go contents", 10, "file:auth.go"),
			nbEntry("e2", 1, 2, notebook.EventGeneral, "unrelated", 10, "phase:general"),
			nbEntry("e3", 2, 1, notebook.EventDecision, "decided X", 10, "phase:decision"),
		}
		got := selectNotebookEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 1})
		require.Contains(t, entryIDs(got), "e1")
	})

	t.Run("text match on basename without tag", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("e1", 1, 1, notebook.EventGeneral, "we discussed auth.go briefly", 10),
			nbEntry("e2", 3, 1, notebook.EventGeneral, "totally different", 10),
		}
		got := selectNotebookEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 2})
		require.Contains(t, entryIDs(got), "e1")
	})

	t.Run("recency keeps two most recent turns", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("old", 1, 1, notebook.EventGeneral, "old", 10),
			nbEntry("mid", 3, 1, notebook.EventGeneral, "mid", 10),
			nbEntry("new", 4, 1, notebook.EventGeneral, "new", 10),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 3})
		require.Contains(t, entryIDs(got), "mid")
		require.Contains(t, entryIDs(got), "new")
	})

	t.Run("recency survives a large ref set", func(t *testing.T) {
		t.Parallel()
		// Many ref-matching entries plus two recent-turn entries; if
		// refs ran before recency, the ref set would exhaust the
		// budget and the recent entries would be dropped.
		var entries []notebook.Entry
		for i := range 200 {
			entries = append(entries, nbEntry(
				fmt.Sprintf("ref%d", i), int64(1+i), 1,
				notebook.EventFileRead, "content of target.go", 200,
				"file:target.go"))
		}
		entries = append(entries,
			nbEntry("recent1", 300, 1, notebook.EventGeneral, "recent", 10),
			nbEntry("recent2", 301, 1, notebook.EventGeneral, "recent", 10),
		)
		got := selectNotebookEntries(entries, []string{"file:target.go"}, segmentKey{turn: 300})
		require.Contains(t, entryIDs(got), "recent1")
		require.Contains(t, entryIDs(got), "recent2")
	})

	t.Run("fills remaining budget newest first", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("old", 1, 1, notebook.EventGeneral, "old", 10),
			nbEntry("mid", 5, 1, notebook.EventGeneral, "mid", 10),
			nbEntry("new", 10, 1, notebook.EventGeneral, "new", 10),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 9})
		require.ElementsMatch(t, []string{"old", "mid", "new"}, entryIDs(got))
	})

	t.Run("cap enforcement skips oversized entries", func(t *testing.T) {
		t.Parallel()
		var entries []notebook.Entry
		// One huge entry plus many small ones; total small > cap.
		entries = append(entries, nbEntry("huge", 1, 1, notebook.EventGeneral, "huge", maxNotebookInjectionTokens-1))
		for i := range 100 {
			entries = append(entries, nbEntry(fmt.Sprintf("s%d", i), int64(2+i), 1, notebook.EventGeneral, "small", 200))
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 100})
		var total int64
		for _, e := range got {
			total += e.TokenCount
		}
		require.LessOrEqual(t, total, int64(maxNotebookInjectionTokens))
		// The huge entry must be skipped while smaller ones fill in.
		require.NotContains(t, entryIDs(got), "huge")
	})

	t.Run("dedup by entry ID", func(t *testing.T) {
		t.Parallel()
		e := nbEntry("dup", 5, 1, notebook.EventFileRead, "auth.go", 10, "file:auth.go")
		entries := []notebook.Entry{e, e}
		got := selectNotebookEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 4})
		require.Len(t, got, 1)
	})

	t.Run("supersession drops stale file_read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read1", 1, 1, notebook.EventFileRead, "first read of auth.go", 10, "file:auth.go"),
			nbEntry("read2", 3, 1, notebook.EventFileRead, "second read of auth.go", 10, "file:auth.go"),
			nbEntry("other", 3, 2, notebook.EventGeneral, "unrelated", 10),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 1})
		require.NotContains(t, entryIDs(got), "read1")
		require.Contains(t, entryIDs(got), "read2")
	})

	t.Run("newer edit supersedes older read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read", 1, 1, notebook.EventFileRead, "read auth.go", 10, "file:auth.go"),
			nbEntry("edit", 2, 1, notebook.EventFileEdit, "edited auth.go", 10, "file:auth.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 1})
		require.NotContains(t, entryIDs(got), "read")
		require.Contains(t, entryIDs(got), "edit")
	})

	t.Run("failed edit does not supersede older read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read", 1, 1, notebook.EventFileRead, "read auth.go", 10, "file:auth.go"),
			nbEntryResult("edit", 2, 1, notebook.EventFileEdit, "failed edit auth.go", 10, false, "file:auth.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 2})
		require.Contains(t, entryIDs(got), "read")
		require.Contains(t, entryIDs(got), "edit")
	})

	t.Run("failed read does not supersede older read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read1", 1, 1, notebook.EventFileRead, "first read of auth.go", 10, "file:auth.go"),
			nbEntryResult("read2", 3, 1, notebook.EventFileRead, "failed re-read", 10, false, "file:auth.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 2})
		require.Contains(t, entryIDs(got), "read1")
		require.Contains(t, entryIDs(got), "read2")
	})

	t.Run("failed edit does not mask a later successful edit", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read", 1, 1, notebook.EventFileRead, "read auth.go", 10, "file:auth.go"),
			nbEntryResult("badedit", 2, 1, notebook.EventFileEdit, "failed edit", 10, false, "file:auth.go"),
			nbEntry("goodedit", 3, 1, notebook.EventFileEdit, "edited auth.go", 10, "file:auth.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 1})
		require.NotContains(t, entryIDs(got), "read")
		require.Contains(t, entryIDs(got), "badedit")
		require.Contains(t, entryIDs(got), "goodedit")
	})

	t.Run("pinned file entries beat unpinned fill under budget pressure", func(t *testing.T) {
		t.Parallel()
		// Recent turns consume almost the whole injection budget; a
		// pinned entry from an old turn wins the remainder over an
		// equally old unpinned entry.
		entries := []notebook.Entry{
			nbEntry("old-pinned", 0, 1, notebook.EventExploration, "explored a.go", 400, "file:a.go"),
			nbEntry("old-free", 0, 2, notebook.EventExploration, "unrelated", 400),
			nbEntry("filler", 2, 1, notebook.EventGeneral, "big", 11400),
			nbEntry("edit", 2, 2, notebook.EventFileEdit, "edited a.go", 100, "file:a.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 1})
		require.Contains(t, entryIDs(got), "old-pinned")
		require.NotContains(t, entryIDs(got), "old-free")
		require.Contains(t, entryIDs(got), "edit")
	})

	t.Run("failed edit still pins", func(t *testing.T) {
		t.Parallel()
		// A file in an edit-fail-retry loop is still under active
		// edit — its older entries stay pinned.
		entries := []notebook.Entry{
			nbEntry("old-pinned", 0, 1, notebook.EventExploration, "explored a.go", 400, "file:a.go"),
			nbEntry("old-free", 0, 2, notebook.EventExploration, "unrelated", 400),
			nbEntry("filler", 2, 1, notebook.EventGeneral, "big", 11400),
			nbEntryResult("edit", 2, 2, notebook.EventFileEdit, "failed edit a.go", 100, false, "file:a.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 1})
		require.Contains(t, entryIDs(got), "old-pinned")
		require.NotContains(t, entryIDs(got), "old-free")
	})

	t.Run("output chronological", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("a", 5, 1, notebook.EventGeneral, "a", 10),
			nbEntry("b", 1, 1, notebook.EventGeneral, "b", 10),
			nbEntry("c", 3, 1, notebook.EventGeneral, "c", 10),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 0})
		require.Equal(t, []string{"b", "c", "a"}, entryIDs(got))
	})

	t.Run("entry without TokenCount uses estimate", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("e", 1, 1, notebook.EventGeneral, strings.Repeat("x", 400), 0),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 0})
		require.Len(t, got, 1)
	})

	t.Run("recency floor is segment-grained within one turn", func(t *testing.T) {
		t.Parallel()
		// Three segments of the same turn, each too big to coexist
		// with both others under the cap. A segment-grained floor
		// selects the two tail segments; a turn-grained floor would
		// select the first two instead.
		entries := []notebook.Entry{
			nbSegEntry("seg0", 0, 0, 1, notebook.EventGeneral, "old", 5000),
			nbSegEntry("seg1", 0, 1, 1, notebook.EventGeneral, "mid", 5000),
			nbSegEntry("seg2", 0, 2, 1, notebook.EventGeneral, "new", 5000),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 1})
		require.ElementsMatch(t, []string{"seg1", "seg2"}, entryIDs(got))
	})

	t.Run("edit in a recent segment pins same-turn entries", func(t *testing.T) {
		t.Parallel()
		// The pin floor applies within a turn: an edit in a segment
		// at or after the floor pins entries from older segments of
		// the same turn.
		entries := []notebook.Entry{
			nbSegEntry("old-pinned", 0, 0, 1, notebook.EventExploration, "explored a.go", 400, "file:a.go"),
			nbSegEntry("old-free", 0, 0, 2, notebook.EventExploration, "unrelated", 400),
			nbSegEntry("filler", 0, 1, 1, notebook.EventGeneral, "big", 11400),
			nbSegEntry("edit", 0, 2, 1, notebook.EventFileEdit, "edited a.go", 100, "file:a.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 2})
		require.Contains(t, entryIDs(got), "old-pinned")
		require.Contains(t, entryIDs(got), "edit")
	})

	t.Run("edit below the floor does not pin", func(t *testing.T) {
		t.Parallel()
		// Same shape but the edit is in a segment before the floor —
		// its file is not "under active edit" and the old entry must
		// compete on budget alone.
		entries := []notebook.Entry{
			nbSegEntry("old-pinned", 0, 0, 1, notebook.EventExploration, "explored a.go", 400, "file:a.go"),
			nbSegEntry("old-free", 0, 0, 2, notebook.EventExploration, "unrelated", 400),
			nbSegEntry("filler", 0, 1, 1, notebook.EventGeneral, "big", 11400),
			nbSegEntry("edit", 0, 0, 3, notebook.EventFileEdit, "edited a.go", 100, "file:a.go"),
		}
		got := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 2})
		require.NotContains(t, entryIDs(got), "old-pinned")
	})
}
