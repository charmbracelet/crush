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

// workingSetFileCap bounds the working set to the most recently
// touched tracked files. The read set is cumulative — unbounded it
// degenerates to "every file ever touched" and the pass decays into a
// second chronological one.
const workingSetFileCap = 64

// fillRecencyBandSegments is the segment distance inside which the fill
// pass stays newest-first regardless of entry type: recent is likely
// relevant whatever the type. Beyond the band, fill order becomes type
// rank then recency.
const fillRecencyBandSegments = 8

// entryTypeRank orders event types for the beyond-band fill —
// file_edit > command > decision > file_read > exploration > general.
// decision sits mid-list: hasDecision is a keyword heuristic, so
// top-ranking it would amplify the noisiest signal over entries
// grounded in real tool events.
func entryTypeRank(eventType string) int {
	switch eventType {
	case notebook.EventFileEdit:
		return 5
	case notebook.EventCommand:
		return 4
	case notebook.EventDecision:
		return 3
	case notebook.EventFileRead:
		return 2
	case notebook.EventExploration:
		return 1
	default:
		// EventGeneral and unknown types — the default bucket for
		// MCP/unknown tools, least predictable content.
		return 0
	}
}

// selectionInput carries the per-render inputs selectNotebookEntries
// reads beyond the entry list itself. Every field feeds the prefix
// fingerprint — identical inputs must render byte-identical output.
type selectionInput struct {
	// workingSet maps a file: basename to the tracked absolute paths
	// recorded for it this session — the session's working set,
	// capped to the most recent touches.
	workingSet map[string][]string
	// livePaths reports which tracked paths still exist on disk:
	// path -> exists. Only paths whose basename appears on some
	// candidate entry are stat'd.
	livePaths map[string]bool
	// bandFloor is the coverage key fillRecencyBandSegments back from
	// the boundary; entries at or after it fill newest-first. The
	// zero key puts every entry inside the band.
	bandFloor segmentKey
}

// withinBand reports whether e falls inside the fill recency band.
func (sel selectionInput) withinBand(e notebook.Entry) bool {
	return e.TurnNumber > sel.bandFloor.turn ||
		(e.TurnNumber == sel.bandFloor.turn && e.SegmentNumber >= sel.bandFloor.segment)
}

// workingSetMatch reports how an entry's file: tags intersect the
// working set: confident when a tag resolves to exactly one tracked
// path or the entry text disambiguates a collision by naming a tracked
// full path; ambiguous when the only match is a collided basename.
func (sel selectionInput) workingSetMatch(e notebook.Entry) (confident, ambiguous bool) {
	for _, tag := range e.Tags {
		base, ok := strings.CutPrefix(tag, "file:")
		if !ok {
			continue
		}
		paths, ok := sel.workingSet[base]
		if !ok {
			continue
		}
		if len(paths) == 1 {
			confident = true
			continue
		}
		// Confident only when the text names exactly one tracked
		// path — a suffix matching two of them stays ambiguous.
		matched := 0
		for _, p := range paths {
			if textNamesPath(e.EntryTextFull, p) || textNamesPath(e.EntryText, p) {
				matched++
			}
		}
		if matched == 1 {
			confident = true
		} else {
			ambiguous = true
		}
	}
	return confident, ambiguous
}

// textNamesPath reports whether text mentions path p — either the
// full tracked path or a distinctive suffix of it (two or more
// components, e.g. "api/auth.go"). Generated entry text spells paths
// relative to the working dir, so only suffixes with a directory
// component count as disambiguation. The match must sit at a
// boundary — "webapi/auth.go" must not satisfy "api/auth.go".
func textNamesPath(text, p string) bool {
	if text == "" {
		return false
	}
	if containsPathBounded(text, p) {
		return true
	}
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	for k := len(parts); k >= 2; k-- {
		suffix := strings.Join(parts[len(parts)-k:], "/")
		if containsPathBounded(text, suffix) {
			return true
		}
	}
	return false
}

// containsPathBounded reports whether text contains sub at a path
// boundary — neither the byte before nor after may extend the path.
func containsPathBounded(text, sub string) bool {
	for idx := strings.Index(text, sub); idx >= 0; {
		beforeOK := idx == 0 || !isPathByte(text[idx-1])
		after := idx + len(sub)
		afterOK := after == len(text) || !isPathByte(text[after])
		if beforeOK && afterOK {
			return true
		}
		n := strings.Index(text[idx+1:], sub)
		if n < 0 {
			return false
		}
		idx += n + 1
	}
	return false
}

// isPathByte reports whether b can be part of a file path token.
func isPathByte(b byte) bool {
	return b == '/' || b == '\\' || b == '.' || b == '-' || b == '_' ||
		'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9'
}

// entryIsDead reports whether every resolved file: tag on the entry
// points at a file that no longer exists. Tags with no tracked path
// are unresolvable and default to live — absence of tracking data is
// not evidence of death.
func (sel selectionInput) entryIsDead(e notebook.Entry) bool {
	resolved, live := false, false
	for _, tag := range e.Tags {
		base, ok := strings.CutPrefix(tag, "file:")
		if !ok {
			continue
		}
		for _, p := range sel.workingSet[base] {
			if exists, tracked := sel.livePaths[p]; tracked {
				resolved = true
				if exists {
					live = true
				}
			}
		}
	}
	return resolved && !live
}

// selectionDiff records which pass contributed each rendered entry —
// the sufficiency instrumentation for selection quality.
type selectionDiff struct {
	recency int
	pinned  int
	refs    int
	working int
	fill    int
}

// total returns the number of entries selection produced.
func (d selectionDiff) total() int {
	return d.recency + d.pinned + d.refs + d.working + d.fill
}

// selectNotebookEntries chooses which notebook entries to render into
// the request. Selection order:
//
//  1. Entries from the two most recent segments (immediate context) —
//     recency runs first so a large ref set can't starve it.
//  2. Entries pinned to files under active edit (an edit entry in the
//     last two segments pins all entries for that file).
//  3. Entries relevant to refs (file paths from the current user
//     message and active todos).
//  4. Entries tagged to the session working set — files touched but
//     not named in the prompt. Newest-first; entries with collided
//     basenames and no disambiguating full path in their text run in
//     a trailing sub-pass.
//  5. Remaining entries until the token cap: newest-first inside the
//     recency band, type rank then recency beyond it. Entries whose
//     resolved file: tags all point at deleted files demote below
//     live-file entries in both halves.
//
// Entries are deduplicated by ID, superseded file reads are dropped in
// favor of newer entries for the same file, and the result is returned
// in chronological order for rendering. The returned selectionDiff
// records which pass contributed each entry.
func selectNotebookEntries(entries []notebook.Entry, refs []string, floor segmentKey, sel selectionInput) ([]notebook.Entry, selectionDiff) {
	var diff selectionDiff
	if len(entries) == 0 {
		return nil, diff
	}

	seen := make(map[string]bool, len(entries))
	selected := make([]notebook.Entry, 0, len(entries))
	var used int64

	trySelect := func(e notebook.Entry, pass func(*selectionDiff)) {
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
		pass(&diff)
	}

	recency := func(d *selectionDiff) { d.recency++ }
	pinnedP := func(d *selectionDiff) { d.pinned++ }
	refsP := func(d *selectionDiff) { d.refs++ }
	working := func(d *selectionDiff) { d.working++ }
	fill := func(d *selectionDiff) { d.fill++ }

	// Pass 1: entries at or after the recency floor — the last two
	// covered segments. Recency is keyed on (turn, segment), not
	// turns: a single long turn would otherwise select every entry it
	// ever produced.
	for _, e := range entries {
		if e.TurnNumber > floor.turn || (e.TurnNumber == floor.turn && e.SegmentNumber >= floor.segment) {
			trySelect(e, recency)
		}
	}
	// Pass 1.5: entries pinned to files under active edit — an edit
	// in the last two segments keeps every entry for that file alive.
	pinned := notebook.PinnedFileTagsSince(entries, floor.turn, floor.segment)
	if len(pinned) > 0 {
		for _, e := range entries {
			for _, tag := range e.Tags {
				if pinned[tag] {
					trySelect(e, pinnedP)
					break
				}
			}
		}
	}
	// Pass 2: entries matching explicit file paths from the user
	// prompt and active todos.
	for _, e := range entries {
		if entryMatchesRefs(e, refs) {
			trySelect(e, refsP)
		}
	}
	// Pass 2.5: entries tagged to the session working set, newest
	// first. The bound is file recency, not entry age: a recently
	// touched file's older entries still promote — deliberate, since
	// a file touched this segment keeps all its history relevant;
	// the 64-file recency cap bounds the set itself.
	// Confident matches run before ambiguous collisions — a
	// basename shared by two tracked files only promotes entries
	// whose text names one of the tracked paths. Dead entries are
	// skipped entirely: a dead tag is by definition a working-set
	// member, so they fall through to the fill pass's dead buckets —
	// demotion below every live entry, not just live working-set
	// entries.
	if len(sel.workingSet) > 0 {
		for i := len(entries) - 1; i >= 0; i-- {
			if confident, _ := sel.workingSetMatch(entries[i]); confident && !sel.entryIsDead(entries[i]) {
				trySelect(entries[i], working)
			}
		}
		for i := len(entries) - 1; i >= 0; i-- {
			confident, ambiguous := sel.workingSetMatch(entries[i])
			if !confident && ambiguous && !sel.entryIsDead(entries[i]) {
				trySelect(entries[i], working)
			}
		}
	}
	// Pass 3: fill the remaining budget. Live entries first — entries
	// tagged only to deleted files demote below them — and within each
	// half: inside the recency band newest-first, beyond it type rank
	// then recency.
	var bandLive, oldLive, bandDead, oldDead []notebook.Entry
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		switch dead := sel.entryIsDead(e); {
		case sel.withinBand(e) && !dead:
			bandLive = append(bandLive, e)
		case sel.withinBand(e):
			bandDead = append(bandDead, e)
		case !dead:
			oldLive = append(oldLive, e)
		default:
			oldDead = append(oldDead, e)
		}
	}
	slices.SortStableFunc(oldLive, typeThenRecency)
	slices.SortStableFunc(oldDead, typeThenRecency)
	for _, e := range slices.Concat(bandLive, oldLive, bandDead, oldDead) {
		trySelect(e, fill)
	}

	selected = dropSupersededReads(selected)
	slices.SortStableFunc(selected, func(a, b notebook.Entry) int {
		if a.TurnNumber != b.TurnNumber {
			return int(a.TurnNumber - b.TurnNumber)
		}
		return int(a.EventNumber - b.EventNumber)
	})
	return selected, diff
}

// typeThenRecency orders the beyond-band fill: higher type rank first,
// ties broken by recency (turn, segment, event), newest first.
func typeThenRecency(a, b notebook.Entry) int {
	if d := entryTypeRank(b.EventType) - entryTypeRank(a.EventType); d != 0 {
		return d
	}
	if a.TurnNumber != b.TurnNumber {
		return int(b.TurnNumber - a.TurnNumber)
	}
	if a.SegmentNumber != b.SegmentNumber {
		return int(b.SegmentNumber - a.SegmentNumber)
	}
	return int(b.EventNumber - a.EventNumber)
}

// coveredSegmentFloor returns the recency floor for selection: the
// key of the second-to-last segment that ends at or before the
// boundary. Entries at or after it count as recent. Segments are
// ordered by construction, so the scan is cheap.
func coveredSegmentFloor(segs []segment, boundary int) segmentKey {
	var last, prev segmentKey
	var covered int
	for _, s := range segs {
		if s.end > boundary {
			break
		}
		prev, last = last, s.key()
		covered++
	}
	if covered == 0 {
		if len(segs) > 0 {
			return segs[0].key()
		}
		return segmentKey{}
	}
	// Two covered segments or more: the floor is the second-to-last,
	// matching the "two most recent" window. With a single covered
	// segment, that segment is the floor. The count — not the zero
	// key — distinguishes the cases: (0,0) is a valid prev.
	if covered >= 2 {
		return prev
	}
	return last
}

// fillBandFloor returns the coverage key fillRecencyBandSegments
// covered segments back from the boundary — the fill pass's recency
// band. Fewer covered segments than the band yields the zero key, so
// every entry counts as within band and fill stays purely newest-first.
func fillBandFloor(segs []segment, boundary int) segmentKey {
	var covered []segment
	for _, s := range segs {
		if s.end > boundary {
			break
		}
		covered = append(covered, s)
	}
	if len(covered) <= fillRecencyBandSegments {
		return segmentKey{}
	}
	return covered[len(covered)-fillRecencyBandSegments].key()
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
// read's snapshot stale. Only successful events supersede: a failed
// edit leaves the file — and the earlier read — untouched. Entries
// without file tags are untouched.
func dropSupersededReads(entries []notebook.Entry) []notebook.Entry {
	// newestForFile maps a file: tag to the newest entry tagged with it.
	type key struct{ turn, event int64 }
	newestForFile := map[string]key{}
	for _, e := range entries {
		if !e.Succeeded {
			continue
		}
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
