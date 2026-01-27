package dialog

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// AgentItem wraps a [subagent.Subagent] to implement the [list.FilterableItem] interface.
type AgentItem struct {
	Agent   *subagent.Subagent
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var (
	_ list.FilterableItem = &AgentItem{}
	_ list.Focusable      = &AgentItem{}
	_ list.MatchSettable  = &AgentItem{}
)

// Filter returns the filterable value of the agent (name + description).
func (a *AgentItem) Filter() string {
	return a.Agent.Name + " " + a.Agent.Description
}

// SetMatch sets the fuzzy match for the agent item.
func (a *AgentItem) SetMatch(m fuzzy.Match) {
	a.cache = nil
	a.m = m
}

// Render returns the string representation of the agent item.
func (a *AgentItem) Render(width int) string {
	if a.cache == nil {
		a.cache = make(map[int]string)
	}

	cached, ok := a.cache[width]
	if ok {
		return cached
	}

	style := a.t.Dialog.NormalItem
	if a.focused {
		style = a.t.Dialog.SelectedItem
	}

	name := a.Agent.Name
	desc := a.Agent.Description

	// Calculate widths.
	lineWidth := width
	descWidth := lineWidth - lipgloss.Width(name) - 3 // 3 for " - " separator

	// Truncate description if needed.
	if descWidth > 10 {
		desc = ansi.Truncate(desc, descWidth, "...")
	} else {
		desc = ""
	}

	var content string
	if desc != "" {
		// Style the name part with emphasis.
		nameStyled := name
		descStyled := desc
		if a.focused {
			descStyled = a.t.HalfMuted.Render(desc)
		} else {
			descStyled = a.t.Subtle.Render(desc)
		}
		content = nameStyled + " - " + descStyled
	} else {
		content = name
	}

	// Apply highlighting for matched indexes.
	if matches := len(a.m.MatchedIndexes); matches > 0 {
		// Only highlight the name portion.
		nameLen := len(a.Agent.Name)
		var parts []string
		var lastPos int
		ranges := matchedRanges(a.m.MatchedIndexes)
		for _, rng := range ranges {
			// Only apply matches within the name.
			if rng[0] >= nameLen {
				continue
			}
			start := rng[0]
			stop := min(rng[1], nameLen-1)
			if start > lastPos {
				parts = append(parts, name[lastPos:start])
			}
			parts = append(parts,
				ansi.NewStyle().Underline(true).String(),
				name[start:stop+1],
				ansi.NewStyle().Underline(false).String(),
			)
			lastPos = stop + 1
		}
		if lastPos < len(name) {
			parts = append(parts, name[lastPos:])
		}
		if len(parts) > 0 {
			highlightedName := ""
			for _, p := range parts {
				highlightedName += p
			}
			if desc != "" {
				descStyled := desc
				if a.focused {
					descStyled = a.t.HalfMuted.Render(desc)
				} else {
					descStyled = a.t.Subtle.Render(desc)
				}
				content = highlightedName + " - " + descStyled
			} else {
				content = highlightedName
			}
		}
	}

	content = style.Render(content)
	a.cache[width] = content
	return content
}

// SetFocused sets the focus state of the agent item.
func (a *AgentItem) SetFocused(focused bool) {
	if a.focused != focused {
		a.cache = nil
	}
	a.focused = focused
}
