package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

type helpCmp struct {
	width  int
	height int
	keys   []key.Binding
}

func (h *helpCmp) Init() tea.Cmd {
	return nil
}

func (h *helpCmp) SetBindings(k []key.Binding) {
	h.keys = k
}

func (h *helpCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = 90
		h.height = msg.Height
	}
	return h, nil
}

func removeDuplicateBindings(bindings []key.Binding) []key.Binding {
	seen := make(map[string]struct{})
	result := make([]key.Binding, 0, len(bindings))

	// Process bindings in reverse order
	for i := len(bindings) - 1; i >= 0; i-- {
		b := bindings[i]
		k := strings.Join(b.Keys(), " ")
		if _, ok := seen[k]; ok {
			// duplicate, skip
			continue
		}
		seen[k] = struct{}{}
		// Add to the beginning of result to maintain original order
		result = append([]key.Binding{b}, result...)
	}

	return result
}

func (h *helpCmp) render() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	helpKeyStyle := styles.Bold().
		Background(t.Background()).
		Foreground(t.Text()).
		Padding(0, 1, 0, 0)

	helpDescStyle := styles.Regular().
		Background(t.Background()).
		Foreground(t.TextMuted())

	// Compile list of bindings to render
	bindings := removeDuplicateBindings(h.keys)

	// Enumerate through each group of bindings, populating a series of
	// pairs of columns, one for keys, one for descriptions
	var (
		pairs []string
		width int
		rows  = 12 - 2
	)

	for i := 0; i < len(bindings); i += rows {
		var (
			keys  []string
			descs []string
		)
		for j := i; j < min(i+rows, len(bindings)); j++ {
			keys = append(keys, helpKeyStyle.Render(bindings[j].Help().Key))
			descs = append(descs, helpDescStyle.Render(bindings[j].Help().Desc))
		}

		// Render pair of columns; beyond the first pair, render a three space
		// left margin, in order to visually separate the pairs.
		var cols []string
		if len(pairs) > 0 {
			cols = []string{baseStyle.Render("   ")}
		}

		maxDescWidth := 0
		for _, desc := range descs {
			if maxDescWidth < lipgloss.Width(desc) {
				maxDescWidth = lipgloss.Width(desc)
			}
		}
		for i := range descs {
			remainingWidth := maxDescWidth - lipgloss.Width(descs[i])
			if remainingWidth > 0 {
				descs[i] = descs[i] + baseStyle.Render(strings.Repeat(" ", remainingWidth))
			}
		}
		maxKeyWidth := 0
		for _, key := range keys {
			if maxKeyWidth < lipgloss.Width(key) {
				maxKeyWidth = lipgloss.Width(key)
			}
		}
		for i := range keys {
			remainingWidth := maxKeyWidth - lipgloss.Width(keys[i])
			if remainingWidth > 0 {
				keys[i] = keys[i] + baseStyle.Render(strings.Repeat(" ", remainingWidth))
			}
		}

		cols = append(cols,
			strings.Join(keys, "\n"),
			strings.Join(descs, "\n"),
		)

		pair := baseStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, cols...))
		// check whether it exceeds the maximum width avail (the width of the
		// terminal, subtracting 2 for the borders).
		width += lipgloss.Width(pair)
		if width > h.width-2 {
			break
		}
		pairs = append(pairs, pair)
	}

	// https://github.com/charmbracelet/lipgloss/issues/209
	if len(pairs) > 1 {
		prefix := pairs[:len(pairs)-1]
		lastPair := pairs[len(pairs)-1]
		prefix = append(prefix, lipgloss.Place(
			lipgloss.Width(lastPair),   // width
			lipgloss.Height(prefix[0]), // height
			lipgloss.Left,              // x
			lipgloss.Top,               // y
			lastPair,                   // content
			lipgloss.WithWhitespaceBackground(t.Background()),
		))
		content := baseStyle.Width(h.width).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				prefix...,
			),
		)
		return content
	}

	// Join pairs of columns and enclose in a border
	content := baseStyle.Width(h.width).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			pairs...,
		),
	)
	return content
}

func (h *helpCmp) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	content := h.render()
	header := baseStyle.
		Bold(true).
		Width(lipgloss.Width(content)).
		Foreground(t.Primary()).
		Render("Keyboard Shortcuts")

	return baseStyle.Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.TextMuted()).
		Width(h.width).
		BorderBackground(t.Background()).
		Render(
			lipgloss.JoinVertical(lipgloss.Center,
				header,
				baseStyle.Render(strings.Repeat(" ", lipgloss.Width(header))),
				content,
			),
		)
}

type HelpCmp interface {
	tea.Model
	SetBindings([]key.Binding)
}

func NewHelpCmp() HelpCmp {
	return &helpCmp{}
}
