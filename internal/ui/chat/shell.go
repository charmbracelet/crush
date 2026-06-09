package chat

import (
	"fmt"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// shellSeq provides unique IDs for ShellItems even when the same
// command is run multiple times.
var shellSeq atomic.Int64

const (
	shellMaxCollapsedLines = 10
	shellHScrollStep       = 5
)

// ShellItem renders a bang-mode shell command result in the chat with a
// vertical bar on the left and plain-text output.
type ShellItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id              string
	command         string
	output          string
	exitCode        int
	expandedContent bool
	xOffset         int
	maxLineWidth    int // computed during render, used to clamp xOffset
	sty             *styles.Styles
}

var (
	_ Expandable         = (*ShellItem)(nil)
	_ list.Highlightable = (*ShellItem)(nil)
	_ KeyEventHandler    = (*ShellItem)(nil)
)

// NewShellItem creates a new ShellItem for displaying bang-mode results.
func NewShellItem(sty *styles.Styles, command, output string, exitCode int) MessageItem {
	v := list.NewVersioned()
	return &ShellItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       fmt.Sprintf("shell-%d-%s", shellSeq.Add(1), command),
		command:                  command,
		output:                   output,
		exitCode:                 exitCode,
		sty:                      sty,
	}
}

func (s *ShellItem) ID() string          { return s.id }
func (s *ShellItem) FilterValue() string { return s.command }
func (s *ShellItem) Finished() bool      { return true }

func (s *ShellItem) Render(width int) string {
	innerWidth := max(0, width-MessageLeftPaddingTotal)
	content := s.RawRender(innerWidth)

	var prefix string
	if s.focused {
		prefix = s.sty.Messages.ShellBarFocused.Render()
	} else {
		prefix = s.sty.Messages.ShellBarBlurred.Render()
	}
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		lines[i] = prefix + ln
	}
	out := strings.Join(lines, "\n")

	return s.renderHighlighted(out, width, lipgloss.Height(out))
}

// HandleMouseClick implements MouseClickable so clicks select this item.
func (s *ShellItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	return btn == ansi.MouseLeft
}

// HandleKeyEvent implements KeyEventHandler for horizontal scrolling.
func (s *ShellItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	switch key.String() {
	case "shift+left", "H":
		if s.xOffset > 0 {
			s.xOffset = max(0, s.xOffset-shellHScrollStep)
			s.Bump()
			return true, nil
		}
	case "shift+right", "L":
		s.xOffset = min(s.xOffset+shellHScrollStep, max(s.maxLineWidth, s.xOffset))
		s.Bump()
		return true, nil
	}
	return false, nil
}

// ScrollHorizontal adjusts the horizontal scroll offset by delta columns.
func (s *ShellItem) ScrollHorizontal(delta int) {
	s.xOffset = max(0, s.xOffset+delta)
	if s.maxLineWidth > 0 {
		s.xOffset = min(s.xOffset, s.maxLineWidth)
	}
	s.Bump()
}

// ToggleExpanded toggles the expanded state and invalidates the cache.
func (s *ShellItem) ToggleExpanded() bool {
	s.expandedContent = !s.expandedContent
	s.Bump()
	return s.expandedContent
}

func (s *ShellItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	cmd := strings.ReplaceAll(s.command, "\n", " ")
	cmd = strings.ReplaceAll(cmd, "\t", "    ")

	var prompt string
	if s.focused {
		prompt = s.sty.Messages.ShellPrompt.Render("$")
	} else {
		prompt = s.sty.Messages.ShellPromptBlurred.Render("$")
	}

	highlighted, err := common.SyntaxHighlight(s.sty, cmd, "cmd.sh", s.sty.Background)
	if err != nil || highlighted == "" {
		highlighted = s.sty.Messages.ShellCommand.Render(cmd)
	}
	header := prompt + " " + highlighted

	if s.exitCode != 0 {
		header += " " + s.sty.Messages.ShellExitCode.Render(fmt.Sprintf("(exit %d)", s.exitCode))
	}

	if s.output == "" {
		return header
	}

	output := strings.TrimRight(s.output, "\n")
	lines := strings.Split(output, "\n")

	maxLines := shellMaxCollapsedLines
	if s.expandedContent {
		maxLines = len(lines)
	}

	displayLines := lines
	if len(lines) > maxLines {
		displayLines = lines[:maxLines]
	}

	// Compute max line width for scroll clamping.
	maxW := 0
	for _, ln := range displayLines {
		w := ansi.StringWidth(ln)
		if w > maxW {
			maxW = w
		}
	}
	s.maxLineWidth = max(0, maxW-cappedWidth)

	var body strings.Builder
	for _, ln := range displayLines {
		scrolled := ansi.GraphemeWidth.Cut(ln, s.xOffset, len(ln))
		truncated := ansi.Truncate(scrolled, cappedWidth, "…")
		if s.xOffset > 0 && strings.TrimSpace(truncated) != "" {
			truncated = "…" + truncated
		}
		body.WriteString(s.sty.Messages.ShellOutput.Render(truncated))
		body.WriteString("\n")
	}

	if len(lines) > maxLines && !s.expandedContent {
		body.WriteString(s.sty.Messages.ShellTruncation.Render(
			fmt.Sprintf("… %d more lines", len(lines)-maxLines),
		))
	} else {
		result := body.String()
		return header + "\n" + strings.TrimRight(result, "\n")
	}

	return header + "\n" + body.String()
}
