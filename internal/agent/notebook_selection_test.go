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

// selectEntries selects with an empty selectionInput — the baseline
// behavior before working-set, band, and liveness inputs exist.
func selectEntries(entries []notebook.Entry, refs []string, floor segmentKey) []notebook.Entry {
	sel, _ := selectNotebookEntries(entries, refs, floor, selectionInput{})
	return sel
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
		got := selectEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 1})
		require.Contains(t, entryIDs(got), "e1")
	})

	t.Run("text match on basename without tag", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("e1", 1, 1, notebook.EventGeneral, "we discussed auth.go briefly", 10),
			nbEntry("e2", 3, 1, notebook.EventGeneral, "totally different", 10),
		}
		got := selectEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 2})
		require.Contains(t, entryIDs(got), "e1")
	})

	t.Run("recency keeps two most recent turns", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("old", 1, 1, notebook.EventGeneral, "old", 10),
			nbEntry("mid", 3, 1, notebook.EventGeneral, "mid", 10),
			nbEntry("new", 4, 1, notebook.EventGeneral, "new", 10),
		}
		got := selectEntries(entries, nil, segmentKey{turn: 3})
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
		got := selectEntries(entries, []string{"file:target.go"}, segmentKey{turn: 300})
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
		got := selectEntries(entries, nil, segmentKey{turn: 9})
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
		got := selectEntries(entries, nil, segmentKey{turn: 100})
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
		got := selectEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 4})
		require.Len(t, got, 1)
	})

	t.Run("supersession drops stale file_read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read1", 1, 1, notebook.EventFileRead, "first read of auth.go", 10, "file:auth.go"),
			nbEntry("read2", 3, 1, notebook.EventFileRead, "second read of auth.go", 10, "file:auth.go"),
			nbEntry("other", 3, 2, notebook.EventGeneral, "unrelated", 10),
		}
		got := selectEntries(entries, nil, segmentKey{turn: 1})
		require.NotContains(t, entryIDs(got), "read1")
		require.Contains(t, entryIDs(got), "read2")
	})

	t.Run("newer edit supersedes older read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read", 1, 1, notebook.EventFileRead, "read auth.go", 10, "file:auth.go"),
			nbEntry("edit", 2, 1, notebook.EventFileEdit, "edited auth.go", 10, "file:auth.go"),
		}
		got := selectEntries(entries, nil, segmentKey{turn: 1})
		require.NotContains(t, entryIDs(got), "read")
		require.Contains(t, entryIDs(got), "edit")
	})

	t.Run("failed edit does not supersede older read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read", 1, 1, notebook.EventFileRead, "read auth.go", 10, "file:auth.go"),
			nbEntryResult("edit", 2, 1, notebook.EventFileEdit, "failed edit auth.go", 10, false, "file:auth.go"),
		}
		got := selectEntries(entries, nil, segmentKey{turn: 2})
		require.Contains(t, entryIDs(got), "read")
		require.Contains(t, entryIDs(got), "edit")
	})

	t.Run("failed read does not supersede older read", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("read1", 1, 1, notebook.EventFileRead, "first read of auth.go", 10, "file:auth.go"),
			nbEntryResult("read2", 3, 1, notebook.EventFileRead, "failed re-read", 10, false, "file:auth.go"),
		}
		got := selectEntries(entries, nil, segmentKey{turn: 2})
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
		got := selectEntries(entries, nil, segmentKey{turn: 1})
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
		got := selectEntries(entries, nil, segmentKey{turn: 1})
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
		got := selectEntries(entries, nil, segmentKey{turn: 1})
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
		got := selectEntries(entries, nil, segmentKey{turn: 0})
		require.Equal(t, []string{"b", "c", "a"}, entryIDs(got))
	})

	t.Run("entry without TokenCount uses estimate", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			nbEntry("e", 1, 1, notebook.EventGeneral, strings.Repeat("x", 400), 0),
		}
		got := selectEntries(entries, nil, segmentKey{turn: 0})
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
		got := selectEntries(entries, nil, segmentKey{turn: 0, segment: 1})
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
		got := selectEntries(entries, nil, segmentKey{turn: 0, segment: 2})
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
		got := selectEntries(entries, nil, segmentKey{turn: 0, segment: 2})
		require.NotContains(t, entryIDs(got), "old-pinned")
	})
}

// selForFiles builds a selectionInput whose working set maps each
// basename to tracked paths; live marks which paths exist.
func selForFiles(ws map[string][]string, live map[string]bool) selectionInput {
	return selectionInput{workingSet: ws, livePaths: live}
}

func TestSelectNotebookEntries_WorkingSet(t *testing.T) {
	t.Parallel()

	t.Run("touched-but-unnamed file promotes its entry", func(t *testing.T) {
		t.Parallel()
		// old.go was read this session but is not in the prompt refs;
		// its entry should arrive via the working-set pass even though
		// it is older than unrelated fill candidates.
		entries := []notebook.Entry{
			nbEntry("ws", 1, 1, notebook.EventFileRead, "read old.go", 10, "file:old.go"),
			nbEntry("unrelated", 5, 1, notebook.EventGeneral, "unrelated", 10),
		}
		sel := selForFiles(map[string][]string{"old.go": {"/w/old.go"}}, nil)
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 6}, sel)
		require.Contains(t, entryIDs(got), "ws")
		require.Equal(t, 1, diff.working)
	})

	t.Run("large working set does not starve recency", func(t *testing.T) {
		t.Parallel()
		// Recency runs before the working-set pass, so a huge touched
		// set can't evict the most recent segments.
		var entries []notebook.Entry
		for i := range 200 {
			entries = append(entries, nbEntry(
				fmt.Sprintf("ws%d", i), int64(1+i), 1,
				notebook.EventFileRead, "read", 200, fmt.Sprintf("file:f%d.go", i)))
		}
		entries = append(entries,
			nbEntry("recent1", 300, 1, notebook.EventGeneral, "recent", 10),
			nbEntry("recent2", 301, 1, notebook.EventGeneral, "recent", 10),
		)
		ws := make(map[string][]string, 200)
		for i := range 200 {
			ws[fmt.Sprintf("f%d.go", i)] = []string{fmt.Sprintf("/w/f%d.go", i)}
		}
		got, _ := selectNotebookEntries(entries, nil, segmentKey{turn: 300}, selForFiles(ws, nil))
		require.Contains(t, entryIDs(got), "recent1")
		require.Contains(t, entryIDs(got), "recent2")
	})

	t.Run("relative-path suffix disambiguates a collision", func(t *testing.T) {
		t.Parallel()
		// Entry text spells the path relative to the working dir;
		// "api/auth.go" is a >=2-component suffix of exactly one
		// tracked path, so the match is confident.
		entries := []notebook.Entry{
			nbEntry("clear", 1, 1, notebook.EventFileRead, "read internal/api/auth.go", 10, "file:auth.go"),
		}
		sel := selForFiles(
			map[string][]string{"auth.go": {"/w/internal/api/auth.go", "/w/web/auth.go"}},
			nil,
		)
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 2}, sel)
		require.Equal(t, []string{"clear"}, entryIDs(got))
		require.Equal(t, 1, diff.working)
	})

	t.Run("collided basename without disambiguation runs second", func(t *testing.T) {
		t.Parallel()
		// Two tracked files share the basename; only the entry whose
		// text names a tracked path is confident. The ambiguous entry
		// still selects in the trailing sub-pass when budget allows.
		entries := []notebook.Entry{
			nbEntry("ambig", 1, 1, notebook.EventFileEdit, "edited auth.go", 10, "file:auth.go"),
			nbEntry("clear", 1, 2, notebook.EventFileRead, "read /w/api/auth.go", 10, "file:auth.go"),
		}
		sel := selForFiles(
			map[string][]string{"auth.go": {"/w/api/auth.go", "/w/web/auth.go"}},
			nil,
		)
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 2}, sel)
		require.ElementsMatch(t, []string{"ambig", "clear"}, entryIDs(got))
		require.Equal(t, 2, diff.working)
	})
}

func TestSelectNotebookEntries_TypeWeighting(t *testing.T) {
	t.Parallel()

	t.Run("old file_edit outranks newer command beyond the band", func(t *testing.T) {
		t.Parallel()
		// Both entries are beyond the band floor; under budget
		// pressure the higher-ranked file_edit wins.
		entries := []notebook.Entry{
			nbSegEntry("edit", 0, 0, 1, notebook.EventFileEdit, "edited a.go", 6000, "file:a.go"),
			nbSegEntry("cmd", 0, 1, 1, notebook.EventCommand, "ran tests", 6000),
			nbSegEntry("recent", 0, 20, 1, notebook.EventGeneral, "recent", 10),
		}
		sel := selectionInput{bandFloor: segmentKey{turn: 0, segment: 10}}
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 21}, sel)
		require.Contains(t, entryIDs(got), "recent")
		require.Contains(t, entryIDs(got), "edit")
		require.NotContains(t, entryIDs(got), "cmd")
		// "recent" sits inside the band but before the recency floor,
		// so it lands via fill too.
		require.Equal(t, 2, diff.fill)
	})

	t.Run("inside the band newest wins regardless of type", func(t *testing.T) {
		t.Parallel()
		// Same shapes but both entries are inside the band: recency
		// rules, so the newer general entry beats the older edit.
		entries := []notebook.Entry{
			nbSegEntry("edit", 0, 15, 1, notebook.EventFileEdit, "edited a.go", 6000, "file:a.go"),
			nbSegEntry("gen", 0, 16, 1, notebook.EventGeneral, "chatter", 6000),
		}
		sel := selectionInput{bandFloor: segmentKey{turn: 0, segment: 10}}
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 21}, sel)
		// Budget fits one; the newer general entry beats the older
		// edit on recency inside the band.
		require.Contains(t, entryIDs(got), "gen")
		require.NotContains(t, entryIDs(got), "edit")
		require.Equal(t, 1, diff.fill)
	})
}

func TestSelectNotebookEntries_DeadDemotion(t *testing.T) {
	t.Parallel()

	t.Run("dead-file entry demotes below same-age live entry", func(t *testing.T) {
		t.Parallel()
		// The dead entry is NEWER than the live one — without
		// demotion inside the working-set pass it would be tried
		// first and win the budget. gone.go's tracked path is dead,
		// so the live entry must win instead.
		entries := []notebook.Entry{
			nbSegEntry("alive", 0, 0, 1, notebook.EventFileRead, "read here.go", 6000, "file:here.go"),
			nbSegEntry("dead", 0, 0, 2, notebook.EventFileRead, "read gone.go", 6000, "file:gone.go"),
		}
		sel := selectionInput{
			bandFloor:  segmentKey{turn: 0, segment: 10},
			workingSet: map[string][]string{"gone.go": {"/w/gone.go"}, "here.go": {"/w/here.go"}},
			livePaths:  map[string]bool{"/w/gone.go": false, "/w/here.go": true},
		}
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 21}, sel)
		require.Contains(t, entryIDs(got), "alive")
		require.NotContains(t, entryIDs(got), "dead")
		// Dead tags resolve through the working set, so the live
		// entry arrives via the working-set pass — demotion bites
		// inside that pass, not fill.
		require.Equal(t, 1, diff.working)
	})

	t.Run("dead entry still selected when budget allows", func(t *testing.T) {
		t.Parallel()
		// Dead entries demote, never drop — historical context stays
		// recallable.
		entries := []notebook.Entry{
			nbSegEntry("dead", 0, 0, 1, notebook.EventFileRead, "read gone.go", 10, "file:gone.go"),
		}
		sel := selectionInput{
			workingSet: map[string][]string{"gone.go": {"/w/gone.go"}},
			livePaths:  map[string]bool{"/w/gone.go": false},
		}
		got, _ := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 21}, sel)
		require.Contains(t, entryIDs(got), "dead")
	})

	t.Run("unresolvable tag stays live", func(t *testing.T) {
		t.Parallel()
		// A file: tag with no tracked path is unresolvable — absence
		// of tracking data is not evidence of deletion. Make the live
		// entry smaller so budget pressure cannot mask the untracked
		// one — the assertion is ordering, not survival.
		entries := []notebook.Entry{
			nbSegEntry("untracked", 0, 0, 1, notebook.EventFileRead, "read mystery.go", 10, "file:mystery.go"),
			nbSegEntry("alive", 0, 0, 2, notebook.EventFileRead, "read here.go", 10, "file:here.go"),
		}
		sel := selectionInput{
			bandFloor:  segmentKey{turn: 0, segment: 10},
			workingSet: map[string][]string{"here.go": {"/w/here.go"}},
			livePaths:  map[string]bool{"/w/here.go": true},
		}
		got, _ := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 21}, sel)
		require.ElementsMatch(t, []string{"untracked", "alive"}, entryIDs(got))
	})

	t.Run("dead ws entry loses to a live fill entry", func(t *testing.T) {
		t.Parallel()
		// The live entry's file was never tracked — it can only win
		// via fill. The dead entry IS working-set tagged, so this
		// proves dead working-set entries demote below live fill.
		entries := []notebook.Entry{
			nbSegEntry("dead", 0, 0, 2, notebook.EventFileRead, "read gone.go", 6000, "file:gone.go"),
			nbSegEntry("alive", 0, 0, 1, notebook.EventFileRead, "read here.go", 6000, "file:here.go"),
		}
		sel := selectionInput{
			bandFloor:  segmentKey{turn: 0, segment: 10},
			workingSet: map[string][]string{"gone.go": {"/w/gone.go"}},
			livePaths:  map[string]bool{"/w/gone.go": false},
		}
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 0, segment: 21}, sel)
		require.Contains(t, entryIDs(got), "alive")
		require.NotContains(t, entryIDs(got), "dead")
		require.Equal(t, 0, diff.working)
		require.Equal(t, 1, diff.fill)
	})

	t.Run("dead demotion does not apply to explicit refs", func(t *testing.T) {
		t.Parallel()
		// The user named the file — a dead entry wins the refs pass
		// even though it loses the fill pass.
		entries := []notebook.Entry{
			nbSegEntry("dead", 0, 0, 1, notebook.EventFileRead, "read gone.go", 6000, "file:gone.go"),
			nbSegEntry("alive", 0, 0, 2, notebook.EventFileRead, "read here.go", 6000, "file:here.go"),
		}
		sel := selectionInput{
			bandFloor:  segmentKey{turn: 0, segment: 10},
			workingSet: map[string][]string{"gone.go": {"/w/gone.go"}, "here.go": {"/w/here.go"}},
			livePaths:  map[string]bool{"/w/gone.go": false, "/w/here.go": true},
		}
		got, diff := selectNotebookEntries(entries, []string{"file:gone.go"}, segmentKey{turn: 0, segment: 21}, sel)
		require.Contains(t, entryIDs(got), "dead")
		require.Equal(t, 1, diff.refs)
	})
}

func TestSelectNotebookEntries_Checkpoints(t *testing.T) {
	t.Parallel()

	ckpt := func(id string, turn, event int64, granularity string, tokens int64, tags ...string) notebook.Entry {
		return nbEntry(id, turn, event, notebook.EventCheckpoint, "checkpoint "+id, tokens,
			append([]string{"granularity:" + granularity, "phase:checkpoint"}, tags...)...)
	}

	t.Run("latest boundary checkpoint outranks edits, stale checkpoint drops", func(t *testing.T) {
		t.Parallel()
		// Three 5000-token entries cannot all fit the 12K injection
		// cap — fill order decides who survives.
		entries := []notebook.Entry{
			ckpt("ckpt-old", 1, 1, notebook.GranularityBoundary, 5000),
			nbEntry("edit", 2, 1, notebook.EventFileEdit, "edited a.go", 5000, "file:a.go"),
			ckpt("ckpt-new", 3, 1, notebook.GranularityBoundary, 5000),
		}
		got, diff := selectNotebookEntries(entries, nil, segmentKey{turn: 100}, selectionInput{})
		ids := entryIDs(got)
		require.Contains(t, ids, "ckpt-new", "the consolidated position renders first")
		require.Contains(t, ids, "edit")
		require.NotContains(t, ids, "ckpt-old",
			"a superseded checkpoint ranks below everything — never outlives its replacement")
		require.Equal(t, 1, diff.checkpoints)
	})

	t.Run("turn digest ranks below edits", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			ckpt("digest", 1, 1, notebook.GranularityTurn, 5000),
			nbEntry("edit", 2, 1, notebook.EventFileEdit, "edited a.go", 5000, "file:a.go"),
			nbEntry("cmd", 2, 2, notebook.EventCommand, "ran tests", 5000),
		}
		got, _ := selectNotebookEntries(entries, nil, segmentKey{turn: 100}, selectionInput{})
		ids := entryIDs(got)
		require.Contains(t, ids, "edit")
		require.Contains(t, ids, "cmd")
		require.NotContains(t, ids, "digest",
			"a turn-grain digest is mid-rank — finer consolidation, ordinary entry")
	})

	t.Run("checkpoint file tags do not supersede the reads it cites", func(t *testing.T) {
		t.Parallel()
		// The checkpoint cites file:x.go as evidence; without the
		// superseder exclusion it would count as the newest
		// observation and drop the read it consolidates.
		entries := []notebook.Entry{
			nbEntry("read", 1, 1, notebook.EventFileRead, "read x.go", 10, "file:x.go"),
			ckpt("ckpt", 2, 1, notebook.GranularityBoundary, 10, "file:x.go"),
		}
		got, _ := selectNotebookEntries(entries, nil, segmentKey{turn: 100}, selectionInput{})
		require.Contains(t, entryIDs(got), "read")
	})

	t.Run("refs pass skips checkpoints", func(t *testing.T) {
		t.Parallel()
		entries := []notebook.Entry{
			ckpt("ckpt", 1, 1, notebook.GranularityBoundary, 10, "file:auth.go"),
		}
		_, diff := selectNotebookEntries(entries, []string{"file:auth.go"}, segmentKey{turn: 100}, selectionInput{})
		require.Equal(t, 0, diff.refs,
			"a checkpoint cites every file it consolidates — ref matching must not promote it")
	})
}
