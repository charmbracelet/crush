package dialog

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

// legacyVisibleItems is the filter ModelsList used before it matched each
// group against its own items only: every group fuzzy-matched every model,
// then kept the ones it owned. It describes the result as strings.
func legacyVisibleItems(groups []ModelGroup, q string) []string {
	query := strings.ToLower(strings.ReplaceAll(q, " ", ""))
	var all []*ModelItem
	for _, g := range groups {
		all = append(all, g.Items...)
	}
	var out []string
	for _, g := range groups {
		name := strings.ToLower(g.Title) + " "
		names := make([]string, len(all))
		for i, item := range all {
			names[i] = fmt.Sprintf("%s%s", name, item.Filter())
		}
		matches := fuzzy.Find(query, names)
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Index < matches[j].Index
		})
		added := 0
		for _, match := range matches {
			item := all[match.Index]
			if !slices.Contains(g.Items, item) {
				continue
			}
			if added == 0 {
				out = append(out, "group "+g.Title)
			}
			idxs := []int{}
			for _, idx := range match.MatchedIndexes {
				if idx >= len(name) {
					idxs = append(idxs, idx-len(name))
				}
			}
			out = append(out, fmt.Sprintf("%s/%s %v", g.Title, item.ID(), idxs))
			added++
		}
		if added > 0 {
			out = append(out, "spacer")
		}
	}
	return out
}

// describeModelsItems describes list items the way legacyVisibleItems does.
func describeModelsItems(t *testing.T, items []list.Item) []string {
	t.Helper()
	var out []string
	var group string
	for _, it := range items {
		switch it := it.(type) {
		case *ModelGroup:
			group = it.Title
			out = append(out, "group "+it.Title)
		case *ModelItem:
			idxs := it.m.MatchedIndexes
			if idxs == nil {
				idxs = []int{}
			}
			out = append(out, fmt.Sprintf("%s/%s %v", group, it.ID(), idxs))
		case *list.SpacerItem:
			out = append(out, "spacer")
		default:
			t.Fatalf("unexpected item %T", it)
		}
	}
	return out
}

// TestModelsListFilterMatchesPerGroup verifies filtering each group on its
// own gives the same items, order and highlights as matching every model
// once per group, and keeps the same header and spacer items across
// filters.
func TestModelsListFilterMatchesPerGroup(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	providers := []catwalk.Provider{
		{ID: "openai", Name: "OpenAI"},
		{ID: "anthropic", Name: "Anthropic"},
		{ID: "gemini", Name: "Gemini"},
	}
	modelNames := []string{"GPT 5", "Claude Sonnet", "Gemini Pro", "Mini", "Large Thinker"}

	recent := NewModelGroup(&sty, "Recently used", false)
	recent.AppendItems(
		NewModelItem(&sty, providers[1], catwalk.Model{ID: "claude-sonnet", Name: "Claude Sonnet"}, ModelTypeLarge, false),
		NewModelItem(&sty, providers[0], catwalk.Model{ID: "gpt-5", Name: "GPT 5"}, ModelTypeLarge, false),
	)
	groups := []ModelGroup{recent}
	for _, p := range providers {
		g := NewModelGroup(&sty, p.Name, true)
		for i, n := range modelNames {
			id := fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(n, " ", "-")), i)
			g.AppendItems(NewModelItem(&sty, p, catwalk.Model{ID: id, Name: n}, ModelTypeLarge, false))
		}
		groups = append(groups, g)
	}

	l := NewModelsList(&sty)
	l.SetGroups(groups...)
	headers := map[*ModelGroup]bool{}
	for i := range l.Len() {
		if g, ok := l.ItemAt(i).(*ModelGroup); ok {
			headers[g] = true
		}
	}

	matched := 0
	for _, q := range []string{"", "g", "gem", "gemini", "openai", "anthropic gpt", "claude", "son", "recently", "zzz", "mini"} {
		want := legacyVisibleItems(l.groups, q)
		if len(want) > 0 {
			matched++
		}
		l.SetFilter(q)
		var items []list.Item
		for i := range l.Len() {
			item := l.ItemAt(i)
			items = append(items, item)
			if g, ok := item.(*ModelGroup); ok {
				require.Truef(t, headers[g], "query %q: header %q was recreated", q, g.Title)
			}
		}
		if q == "" {
			continue
		}
		require.Equalf(t, want, describeModelsItems(t, items), "query %q", q)
	}
	require.Greater(t, matched, 5, "most queries should match something")
}
