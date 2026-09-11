package agent

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/notebook"
)

// maxNotebookInjectionTokens caps the rendered notebook block injected
// into each request. Raw recent turns are never dropped to make room
// for notebook entries — the cap applies to entries only.
const maxNotebookInjectionTokens = 12_000

// selectNotebookEntries chooses which notebook entries to render into
// the request. Selection order:
//
//  1. Entries from the two most recent turns (immediate context) —
//     recency runs first so a large ref set can't starve it.
//  2. Entries relevant to refs (file paths from the current user
//     message and active todos).
//  3. Remaining entries, newest first, until the token cap.
//
// Entries are deduplicated by ID, superseded file reads are dropped in
// favor of newer entries for the same file, and the result is returned
// in chronological order for rendering.
func selectNotebookEntries(entries []notebook.Entry, refs []string, maxTurn int64) []notebook.Entry {
	if len(entries) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(entries))
	selected := make([]notebook.Entry, 0, len(entries))
	var used int64

	trySelect := func(e notebook.Entry) {
		if seen[e.ID] {
			return
		}
		tokens := e.TokenCount
		if tokens <= 0 {
			tokens = approxTokenCount(e.EntryText)
		}
		// Note: the budget measures pre-render tokens, while
		// RenderEntries may emit compressEntry output — imprecise but
		// fine for a soft cap.
		// Skip oversized entries but keep looking — a large entry must
		// not stop smaller ones from filling the budget. +1 token
		// accounts for the "\n\n" join separator RenderEntries emits.
		if used+tokens+1 > maxNotebookInjectionTokens {
			return
		}
		seen[e.ID] = true
		used += tokens + 1
		selected = append(selected, e)
	}

	// Pass 1: the two most recent turns.
	for _, e := range entries {
		if e.TurnNumber >= maxTurn-1 {
			trySelect(e)
		}
	}
	// Pass 2: entries matching explicit file paths from the user
	// prompt and active todos.
	for _, e := range entries {
		if entryMatchesRefs(e, refs) {
			trySelect(e)
		}
	}
	// Pass 3: fill the remaining budget newest-first.
	for i := len(entries) - 1; i >= 0; i-- {
		trySelect(entries[i])
	}

	selected = dropSupersededReads(selected)
	slices.SortStableFunc(selected, func(a, b notebook.Entry) int {
		if a.TurnNumber != b.TurnNumber {
			return int(a.TurnNumber - b.TurnNumber)
		}
		return int(a.EventNumber - b.EventNumber)
	})
	return selected
}

// formatTurnRanges renders a sorted turn list compactly, grouping
// consecutive numbers: 3,4,5,9 -> "3-5, 9".
func formatTurnRanges(turns []int64) string {
	if len(turns) == 0 {
		return ""
	}
	var b strings.Builder
	start, prev := turns[0], turns[0]
	flush := func() {
		if start == prev {
			fmt.Fprintf(&b, "%d", start)
		} else {
			fmt.Fprintf(&b, "%d-%d", start, prev)
		}
	}
	for _, t := range turns[1:] {
		if t == prev+1 {
			prev = t
			continue
		}
		flush()
		b.WriteString(", ")
		start, prev = t, t
	}
	flush()
	return b.String()
}

// entryMatchesRefs reports whether an entry is relevant to any of the
// "file:basename" refs: a matching tag, or the basename appearing in the
// entry text. The basename substring match is intentionally loose —
// it is a prioritization hint: false positives (auth.go matching
// oauth.go) waste a little budget rather than corrupting output.
func entryMatchesRefs(e notebook.Entry, refs []string) bool {
	for _, ref := range refs {
		basename := strings.TrimPrefix(ref, "file:")
		for _, tag := range e.Tags {
			if tag == ref {
				return true
			}
		}
		if basename != "" && (strings.Contains(e.EntryText, basename) ||
			strings.Contains(e.EntryTextFull, basename)) {
			return true
		}
	}
	return false
}

// dropSupersededReads drops older file_read entries when a newer entry
// for the same file exists — a re-read or an edit makes the earlier
// read's snapshot stale. Entries without file tags are untouched.
func dropSupersededReads(entries []notebook.Entry) []notebook.Entry {
	// newestForFile maps a file: tag to the newest entry tagged with it.
	type key struct{ turn, event int64 }
	newestForFile := map[string]key{}
	for _, e := range entries {
		for _, tag := range e.Tags {
			if !strings.HasPrefix(tag, "file:") {
				continue
			}
			k := key{e.TurnNumber, e.EventNumber}
			if cur, ok := newestForFile[tag]; !ok || k.turn > cur.turn || (k.turn == cur.turn && k.event > cur.event) {
				newestForFile[tag] = k
			}
		}
	}
	out := entries[:0]
	for _, e := range entries {
		if e.EventType != notebook.EventFileRead {
			out = append(out, e)
			continue
		}
		drop := false
		for _, tag := range e.Tags {
			if !strings.HasPrefix(tag, "file:") {
				continue
			}
			newest := newestForFile[tag]
			if newest.turn > e.TurnNumber || (newest.turn == e.TurnNumber && newest.event > e.EventNumber) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}
