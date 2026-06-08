package dialog

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

// versionedItem is the cross-cutting interface every dialog list
// item must satisfy under F6: every documented mutator must bump
// the shared version counter so the list-level memo invalidates
// frozen entries.
type versionedItem interface {
	list.Item
	Version() uint64
}

// requireBump asserts that running mutate() advances the item's
// Version().
func requireBump(t *testing.T, name string, item versionedItem, mutate func()) {
	t.Helper()
	before := item.Version()
	mutate()
	after := item.Version()
	require.Greaterf(t, after, before, "%s must bump Version() (before=%d, after=%d)", name, before, after)
}

// requireNoBump asserts that running mutate() leaves the item's
// Version() unchanged. Used to lock in the dedupe contract: a
// mutator called with a value identical to the current state must
// not gratuitously invalidate the list cache.
func requireNoBump(t *testing.T, name string, item versionedItem, mutate func()) {
	t.Helper()
	before := item.Version()
	mutate()
	after := item.Version()
	require.Equalf(t, before, after, "%s must NOT bump Version() when state is unchanged (before=%d, after=%d)", name, before, after)
}

// equivMatch returns a fuzzy.Match whose fields and indexes are
// equivalent to the supplied seed but allocated as a fresh struct
// so callers exercise the value-equality dedupe path rather than
// referential equality.
func equivMatch(seed fuzzy.Match) fuzzy.Match {
	return fuzzy.Match{
		Str:            seed.Str,
		Index:          seed.Index,
		Score:          seed.Score,
		MatchedIndexes: append([]int(nil), seed.MatchedIndexes...),
	}
}

// TestCommandItem_MutatorsBumpVersion covers F6 §4.5 for the
// commands palette items: SetFocused and SetMatch bump Version()
// on observable change and dedupe otherwise.
func TestCommandItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewCommandItem(&sty, "id", "Title", "ctrl+t", nil)

	requireBump(t, "SetFocused[true]", item, func() {
		item.SetFocused(true)
	})
	requireNoBump(t, "SetFocused[true again]", item, func() {
		item.SetFocused(true)
	})
	requireBump(t, "SetFocused[false]", item, func() {
		item.SetFocused(false)
	})

	match := fuzzy.Match{
		Str:            "Title",
		Index:          0,
		Score:          5,
		MatchedIndexes: []int{0, 1, 2},
	}
	requireBump(t, "SetMatch[new]", item, func() {
		item.SetMatch(match)
	})
	requireNoBump(t, "SetMatch[same]", item, func() {
		item.SetMatch(equivMatch(match))
	})
	requireBump(t, "SetMatch[different]", item, func() {
		item.SetMatch(fuzzy.Match{
			Str:            "Title",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{0, 2},
		})
	})
}

// TestModelItem_MutatorsBumpVersion covers F6 §4.5 for the model
// picker items.
func TestModelItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	prov := catwalk.Provider{ID: "openai", Name: "OpenAI"}
	model := catwalk.Model{ID: "gpt-4", Name: "GPT-4"}
	item := NewModelItem(&sty, prov, model, ModelTypeLarge, true)

	requireBump(t, "SetFocused[true]", item, func() {
		item.SetFocused(true)
	})
	requireNoBump(t, "SetFocused[true again]", item, func() {
		item.SetFocused(true)
	})

	match := fuzzy.Match{
		Str:            "GPT-4",
		Index:          0,
		Score:          5,
		MatchedIndexes: []int{0, 1, 2},
	}
	requireBump(t, "SetMatch[new]", item, func() {
		item.SetMatch(match)
	})
	requireNoBump(t, "SetMatch[same]", item, func() {
		item.SetMatch(equivMatch(match))
	})
	requireBump(t, "SetMatch[different]", item, func() {
		item.SetMatch(fuzzy.Match{
			Str:            "GPT-4",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{1},
		})
	})
}

// TestSessionItem_MutatorsBumpVersion covers F6 §4.5 for the
// sessions dialog items.
func TestSessionItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &SessionItem{
		Versioned: list.NewVersioned(),
		Session:   session.Session{ID: "sess-1", Title: "My Session"},
		t:         &sty,
	}

	requireBump(t, "SetFocused[true]", item, func() {
		item.SetFocused(true)
	})
	requireNoBump(t, "SetFocused[true again]", item, func() {
		item.SetFocused(true)
	})

	match := fuzzy.Match{
		Str:            "My Session",
		Index:          0,
		Score:          5,
		MatchedIndexes: []int{0, 1, 2},
	}
	requireBump(t, "SetMatch[new]", item, func() {
		item.SetMatch(match)
	})
	requireNoBump(t, "SetMatch[same]", item, func() {
		item.SetMatch(equivMatch(match))
	})
	requireBump(t, "SetMatch[different]", item, func() {
		item.SetMatch(fuzzy.Match{
			Str:            "My Session",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{3, 4},
		})
	})
}

// TestReasoningItem_MutatorsBumpVersion covers F6 §4.5 for the
// reasoning effort dialog items.
func TestReasoningItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &ReasoningItem{
		Versioned: list.NewVersioned(),
		effort:    "medium",
		title:     "Medium",
		t:         &sty,
	}

	requireBump(t, "SetFocused[true]", item, func() {
		item.SetFocused(true)
	})
	requireNoBump(t, "SetFocused[true again]", item, func() {
		item.SetFocused(true)
	})

	match := fuzzy.Match{
		Str:            "Medium",
		Index:          0,
		Score:          5,
		MatchedIndexes: []int{0, 1, 2},
	}
	requireBump(t, "SetMatch[new]", item, func() {
		item.SetMatch(match)
	})
	requireNoBump(t, "SetMatch[same]", item, func() {
		item.SetMatch(equivMatch(match))
	})
	requireBump(t, "SetMatch[different]", item, func() {
		item.SetMatch(fuzzy.Match{
			Str:            "Medium",
			Index:          0,
			Score:          5,
			MatchedIndexes: []int{2, 3},
		})
	})
}
