package compaction

import (
	"fmt"
	"sort"
	"strings"
)

// The ledger is a second record of a session: what happened, what was decided,
// and why. Compaction renders a checkpoint from it rather than asking a model to
// rewrite the past each time, which is what keeps the rationale that
// summarize-the-summary normally loses.
//
// Entries are append-only. Nothing is edited or deleted; a superseded claim is
// retired by appending a Drop that names it. That keeps the ledger auditable
// against the transcript and means an earlier reading of the session can always
// be reconstructed.

// EntryKind classifies one ledger record.
type EntryKind string

const (
	// KindObservation is something that happened or was established: a file
	// changed, a command's outcome, a constraint the user stated.
	KindObservation EntryKind = "observation"
	// KindReflection is a durable conclusion drawn from observations, citing
	// the entries it rests on. Reflections outlive the evidence that formed
	// them, which is why they are a separate kind rather than a longer note.
	KindReflection EntryKind = "reflection"
	// KindDrop retires earlier entries without deleting them.
	KindDrop EntryKind = "drop"
)

// Relevance decides what survives a render that cannot keep everything.
type Relevance int

const (
	// RelevanceContext is background that the transcript still holds, so
	// losing it in a render costs little.
	RelevanceContext Relevance = iota
	// RelevanceNotable is a concrete thing that happened.
	RelevanceNotable
	// RelevanceDecision is a choice with a rationale, a rejected approach, or
	// a standing constraint. These are the facts a re-summarized session
	// tends to lose and the reason the ledger exists, so they are the last
	// entries dropped.
	RelevanceDecision
)

// Entry is one append-only record.
type Entry struct {
	// Seq orders entries within a session. It is assigned by the store rather
	// than the caller so a render does not depend on insert timing.
	Seq int
	// ID names the entry so reflections and drops can cite it.
	ID string
	// Kind is one of the EntryKind constants.
	Kind EntryKind
	// Text is what to remember, written to be read without the transcript.
	Text string
	// Relevance weights Text against the render budget.
	Relevance Relevance
	// Sources lists message ids the entry was distilled from. Keeping them
	// makes the ledger checkable against the transcript instead of a second
	// story nobody can verify.
	Sources []string
	// Retires names the Seqs this entry supersedes. Only KindDrop uses it.
	Retires []int
}

// Ledger is the projection a checkpoint renders from.
type Ledger struct {
	Entries []Entry
	// CoversThrough is the highest transcript sequence the ledger accounts
	// for. Everything after it is still raw and has not been observed.
	CoversThrough int
}

// Empty reports whether there is nothing to render from, which is when the
// caller should fall back to summarizing with a model.
func (l Ledger) Empty() bool { return len(l.active()) == 0 }

// Unobserved returns the entries the ledger holds that have no counterpart in a
// transcript region, which is how a render stays honest about what it never
// saw. It compares by message id rather than position because the ledger is
// keyed by what was observed, not by where.
func (l Ledger) Unobserved(ids []string) []string {
	seen := make(map[string]bool, len(l.Entries))
	for _, e := range l.Entries {
		for _, id := range e.Sources {
			seen[id] = true
		}
	}
	var missing []string
	for _, id := range ids {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

// active returns entries that no Drop has retired, in insertion order. Drops
// apply transitively: retiring an entry that itself retired others does not
// resurrect the earlier ones.
func (l Ledger) active() []Entry {
	dropped := make(map[int]bool)
	for _, e := range l.Entries {
		if e.Kind != KindDrop {
			continue
		}
		for _, seq := range e.Retires {
			dropped[seq] = true
		}
	}
	out := make([]Entry, 0, len(l.Entries))
	for _, e := range l.Entries {
		if e.Kind == KindDrop || dropped[e.Seq] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// RenderBudget is how much context the rendered ledger may occupy.
type RenderBudget int64

// Rendered is the outcome of a render, including what had to be left out. A
// caller that finds the omissions unacceptable can choose prose instead, so the
// render never silently discards something the model might have kept.
type Rendered struct {
	Text string
	Kept int
	// Dropped counts entries omitted to fit the budget. Any dropped entry at
	// RelevanceDecision is worth a second look: that is the class of fact the
	// ledger exists to preserve.
	Dropped     int
	DroppedHigh int
	Tokens      int64
	// CoversThrough echoes the ledger watermark so a caller can record how far
	// the rendered checkpoint speaks for.
	CoversThrough int
}

// Render turns the ledger into checkpoint text without consulting a model.
//
// Selection is by weight then recency, but emission is chronological within
// each section, because "we rejected X before settling on Y" only reads
// correctly in the order it happened.
//
// A budget of 0 means no limit, which is what preview wants: show everything
// and let the caller decide if it is too much.
func (l Ledger) Render(budget RenderBudget) Rendered {
	entries := l.active()

	// Fill the budget from the facts that cost the most to lose, newest first
	// among equals.
	ranked := make([]Entry, len(entries))
	copy(ranked, entries)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Relevance != ranked[j].Relevance {
			return ranked[i].Relevance > ranked[j].Relevance
		}
		return ranked[i].Seq > ranked[j].Seq
	})

	kept := make([]Entry, 0, len(ranked))
	var tokens int64
	for _, e := range ranked {
		cost := textTokens(e.Text)
		if budget > 0 && tokens+cost > int64(budget) {
			// Decisions are the point of the exercise, so spend whatever the
			// budget has left on them rather than dropping one to fit a
			// lesser entry that happened to come first.
			if e.Relevance == RelevanceDecision {
				kept = append(kept, e)
				tokens += cost
			}
			continue
		}
		kept = append(kept, e)
		tokens += cost
	}

	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Seq < kept[j].Seq })

	var b strings.Builder
	dropped, droppedHigh := 0, 0
	for _, e := range entries {
		if !containsSeq(kept, e.Seq) {
			dropped++
			if e.Relevance == RelevanceDecision {
				droppedHigh++
			}
		}
	}

	section(&b, "Durable conclusions", kept, KindReflection)
	section(&b, "What happened", kept, KindObservation)
	if dropped > 0 {
		fmt.Fprintf(&b,
			"%d lower-priority details were left out of this rendering to fit its "+
				"budget; they remain in the full transcript.\n", dropped)
	}

	return Rendered{
		Text:          strings.TrimRight(b.String(), "\n"),
		Kept:          len(kept),
		Dropped:       dropped,
		DroppedHigh:   droppedHigh,
		Tokens:        tokens,
		CoversThrough: l.CoversThrough,
	}
}

func section(b *strings.Builder, title string, kept []Entry, kind EntryKind) {
	var lines []string
	for _, e := range kept {
		if e.Kind == kind {
			lines = append(lines, "- "+e.Text)
		}
	}
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n%s\n", title, strings.Join(lines, "\n"))
}

func containsSeq(entries []Entry, seq int) bool {
	for _, e := range entries {
		if e.Seq == seq {
			return true
		}
	}
	return false
}
