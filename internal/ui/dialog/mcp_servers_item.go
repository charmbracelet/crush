package dialog

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// MCPServerItem wraps a [mcp.ClientInfo] to implement the [list.FilterableItem] interface.
type MCPServerItem struct {
	Server  *mcp.ClientInfo
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var (
	_ list.FilterableItem = &MCPServerItem{}
	_ list.Focusable      = &MCPServerItem{}
	_ list.MatchSettable  = &MCPServerItem{}
)

// Filter returns the filterable value of the server (name).
func (s *MCPServerItem) Filter() string {
	return s.Server.Name
}

// SetMatch sets the fuzzy match for the server item.
func (s *MCPServerItem) SetMatch(m fuzzy.Match) {
	s.cache = nil
	s.m = m
}

// Render returns the string representation of the server item.
func (s *MCPServerItem) Render(width int) string {
	if s.cache == nil {
		s.cache = make(map[int]string)
	}

	cached, ok := s.cache[width]
	if ok {
		return cached
	}

	style := s.t.Dialog.NormalItem
	if s.focused {
		style = s.t.Dialog.SelectedItem
	}

	name := s.Server.Name

	// Build status indicator.
	var statusIcon string
	var statusText string
	switch s.Server.State {
	case mcp.StateConnected:
		statusIcon = s.t.ItemOnlineIcon.String()
		if s.Server.Counts.Tools > 0 || s.Server.Counts.Prompts > 0 {
			parts := []string{}
			if s.Server.Counts.Tools > 0 {
				parts = append(parts, fmt.Sprintf("%d tools", s.Server.Counts.Tools))
			}
			if s.Server.Counts.Prompts > 0 {
				parts = append(parts, fmt.Sprintf("%d prompts", s.Server.Counts.Prompts))
			}
			statusText = fmt.Sprintf(" - %s", parts[0])
			if len(parts) > 1 {
				statusText = fmt.Sprintf(" - %s, %s", parts[0], parts[1])
			}
		}
	case mcp.StateStarting:
		statusIcon = s.t.ItemBusyIcon.String()
		statusText = " - starting..."
	case mcp.StateError:
		statusIcon = s.t.ItemErrorIcon.String()
		statusText = " - error"
	case mcp.StateDisabled:
		statusIcon = s.t.ItemOfflineIcon.String()
		statusText = " - disabled"
	default:
		statusIcon = s.t.ItemOfflineIcon.String()
	}

	// Calculate widths.
	lineWidth := width

	// Truncate if needed.
	maxNameWidth := lineWidth - len(statusText) - 4 // 4 for icon and spacing
	if len(name) > maxNameWidth {
		name = ansi.Truncate(name, maxNameWidth, "...")
	}

	// Apply highlighting for matched indexes.
	if matches := len(s.m.MatchedIndexes); matches > 0 {
		nameLen := len(s.Server.Name)
		var parts []string
		var lastPos int
		ranges := matchedRanges(s.m.MatchedIndexes)
		for _, rng := range ranges {
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
			name = highlightedName
		}
	}

	// Build the final content.
	var statusStyled string
	if s.focused {
		statusStyled = s.t.HalfMuted.Render(statusText)
	} else {
		statusStyled = s.t.Subtle.Render(statusText)
	}

	content := statusIcon + " " + name + statusStyled
	content = style.Render(content)
	s.cache[width] = content
	return content
}

// SetFocused sets the focus state of the server item.
func (s *MCPServerItem) SetFocused(focused bool) {
	if s.focused != focused {
		s.cache = nil
	}
	s.focused = focused
}
