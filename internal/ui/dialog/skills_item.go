package dialog

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

var _ ListItem = &SkillItem{}

type SkillItem struct {
	entry   skills.CatalogEntry
	action  Action
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

func NewSkillItem(t *styles.Styles, entry skills.CatalogEntry, action Action) *SkillItem {
	return &SkillItem{entry: entry, action: action, t: t}
}

func (s *SkillItem) Filter() string {
	return s.entry.Label + " " + s.entry.Name + " " + s.entry.Description
}

func (s *SkillItem) ID() string { return s.entry.ID }

func (s *SkillItem) SetFocused(focused bool) {
	if s.focused != focused {
		s.cache = nil
	}
	s.focused = focused
}

func (s *SkillItem) SetMatch(m fuzzy.Match) {
	if s.m.Str == m.Str && s.m.Index == m.Index &&
		s.m.Score == m.Score && slices.Equal(s.m.MatchedIndexes, m.MatchedIndexes) {
		return
	}
	s.cache = nil
	s.m = m
}

func (s *SkillItem) Action() Action { return s.action }

func (s *SkillItem) Render(width int) string {
	if s.cache == nil {
		s.cache = make(map[int]string)
	}
	if cached, ok := s.cache[width]; ok {
		return cached
	}

	var lineStyle, descStyle lipgloss.Style
	if s.focused {
		lineStyle = s.t.Dialog.SelectedItem.Width(width)
		descStyle = s.t.Dialog.SelectedItem.Width(width)
	} else {
		lineStyle = s.t.Dialog.NormalItem.Width(width)
		descStyle = s.t.Subtle.Padding(0, 1).Width(width)
	}

	title := ansi.Truncate(s.entry.Label, max(0, width-2), "…")
	if len(s.m.MatchedIndexes) > 0 {
		var lastPos int
		var parts []string
		for _, rng := range matchedRanges(s.m.MatchedIndexes) {
			start, stop := bytePosToVisibleCharPos(title, rng)
			if start > lastPos {
				parts = append(parts, ansi.Cut(title, lastPos, start))
			}
			parts = append(parts,
				ansi.NewStyle().Underline(true).String(),
				ansi.Cut(title, start, stop+1),
				ansi.NewStyle().Underline(false).String(),
			)
			lastPos = stop + 1
		}
		if lastPos < ansi.StringWidth(title) {
			parts = append(parts, ansi.Cut(title, lastPos, ansi.StringWidth(title)))
		}
		title = strings.Join(parts, "")
	}
	description := ansi.Truncate(strings.TrimSpace(s.entry.Description), max(0, width-2), "…")
	if description == "" {
		description = " "
	}

	rendered := lipgloss.JoinVertical(
		lipgloss.Left,
		lineStyle.Render(title),
		descStyle.Render(description),
	)
	s.cache[width] = rendered
	return rendered
}
