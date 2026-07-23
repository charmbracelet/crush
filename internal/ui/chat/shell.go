package chat

import (
	"fmt"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/ui/anim"
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
	output          strings.Builder
	exitCode        int
	expandedContent bool
	xOffset         int
	maxLineWidth    int // computed during render, used to clamp xOffset
	sty             *styles.Styles
	pending         bool
	anim            *anim.Anim
}

var (
	_ Expandable         = (*ShellItem)(nil)
	_ list.Highlightable = (*ShellItem)(nil)
	_ KeyEventHandler    = (*ShellItem)(nil)
	_ Animatable         = (*ShellItem)(nil)
)

// NewShellItem creates a new ShellItem for displaying bang-mode results.
func NewShellItem(sty *styles.Styles, command, output string, exitCode int) MessageItem {
	v := list.NewVersioned()
	s := &ShellItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       fmt.Sprintf("shell-%d-%s", shellSeq.Add(1), command),
		command:                  command,
		exitCode:                 exitCode,
		sty:                      sty,
	}
	s.replaceOutput(output)
	return s
}

// NewPendingShellItem creates a ShellItem in a pending/running state that
// displays a spinner until Complete is called with the results.
func NewPendingShellItem(sty *styles.Styles, command string) *ShellItem {
	v := list.NewVersioned()
	id := fmt.Sprintf("shell-%d-%s", shellSeq.Add(1), command)
	s := &ShellItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       id,
		command:                  command,
		sty:                      sty,
		pending:                  true,
	}
	s.anim = anim.New(anim.Settings{
		ID:         id,
		Label:      "Running",
		LabelColor: sty.WorkingLabelColor,
		GradColorA: sty.WorkingGradFromColor,
		GradColorB: sty.WorkingGradToColor,
		NoScramble: true,
	})
	return s
}

// Complete transitions a pending ShellItem to a finished state with output.
func (s *ShellItem) Complete(output string, exitCode int) {
	s.replaceOutput(output)
	s.exitCode = exitCode
	s.pending = false
	s.Bump()
}

// AppendOutput appends incremental output to a pending ShellItem.
func (s *ShellItem) AppendOutput(chunk string) {
	if !s.pending {
		return
	}
	s.output.WriteString(chunk)
	s.Bump()
}

func (s *ShellItem) replaceOutput(output string) {
	s.output.Reset()
	s.output.Grow(len(output))
	s.output.WriteString(output)
}

func (s *ShellItem) ID() string          { return s.id }
func (s *ShellItem) FilterValue() string { return s.command }
func (s *ShellItem) Finished() bool      { return !s.pending }

// StartAnimation starts the spinner animation for pending shell items.
func (s *ShellItem) StartAnimation() tea.Cmd {
	if !s.pending {
		return nil
	}
	return s.anim.Start()
}

// Animate advances the spinner animation for pending shell items.
func (s *ShellItem) Animate(msg anim.StepMsg) tea.Cmd {
	if !s.pending {
		return nil
	}
	s.Bump()
	return s.anim.Animate(msg)
}

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

// HandleKeyEvent implements KeyEventHandler for copy and horizontal scrolling.
func (s *ShellItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	switch k := key.String(); k {
	case "c", "y":
		text := "$ " + s.command + "\n" + ansi.Strip(s.output.String())
		return true, common.CopyToClipboard(text, "Shell output copied to clipboard")
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

	highlighted := s.sty.Messages.ShellCommand.Render(cmd)
	header := prompt + " " + highlighted

	if s.pending {
		if s.output.Len() == 0 {
			// Nothing streamed yet: show the spinner under the header.
			return header + "\n" + s.anim.Render()
		}
	} else if s.exitCode != 0 {
		header += " " + s.sty.Messages.ShellExitCode.Render(fmt.Sprintf("(exit %d)", s.exitCode))
	}

	if s.output.Len() == 0 {
		return header
	}

	// Remap raw ANSI 16-color codes onto legible Charmtone colors so
	// dark terminal defaults don't render illegibly on Crush's
	// background.
	// Strip trailing whitespace and bare ANSI resets before remapping.
	// Programs like `task` emit "\x1b[0m\n" after their last line of
	// output; trimming only "\n" misses these because the reset bytes
	// sit between the content and the newline.
	fullOutput := s.output.String()
	raw := fullOutput
	for {
		trimmed := strings.TrimRight(raw, " \t\r\n")
		trimmed = strings.TrimSuffix(trimmed, "\x1b[0m")
		if trimmed == raw {
			break
		}
		raw = trimmed
	}
	if raw == "" {
		return header
	}

	// Count lines from the trimmed output directly so truncation
	// logic cannot drift from the trimming loop above.
	totalLines := strings.Count(raw, "\n") + 1
	truncatedCount := 0
	if !s.expandedContent && totalLines > shellMaxCollapsedLines {
		truncatedCount = totalLines - shellMaxCollapsedLines
		if s.pending {
			raw = lastLines(raw, shellMaxCollapsedLines)
		} else {
			raw = firstLines(raw, shellMaxCollapsedLines)
		}
	}

	output := common.RemapANSI16(raw, s.sty.ANSI)
	lines := strings.Split(output, "\n")

	// Compute max line width for scroll clamping.
	maxW := 0
	for _, ln := range lines {
		w := ansi.StringWidth(ln)
		if w > maxW {
			maxW = w
		}
	}
	s.maxLineWidth = max(0, maxW-cappedWidth)

	var body strings.Builder

	// When streaming, hidden lines are above the visible tail, so show
	// the "more lines" notice before the output.
	if truncatedCount > 0 && s.pending {
		body.WriteString(s.sty.Messages.ShellTruncation.Render(
			fmt.Sprintf("… %d earlier lines", truncatedCount),
		))
		body.WriteString("\n")
	}

	for _, ln := range lines {
		scrolled := ansi.GraphemeWidth.Cut(ln, s.xOffset, len(ln))
		truncated := ansi.Truncate(scrolled, cappedWidth, "…")
		if s.xOffset > 0 && strings.TrimSpace(truncated) != "" {
			truncated = "…" + truncated
		}
		body.WriteString(s.sty.Messages.ShellOutput.Render(truncated))
		body.WriteString("\n")
	}

	// When finished, hidden lines are below, so show the notice after.
	if truncatedCount > 0 && !s.pending && !s.expandedContent {
		body.WriteString(s.sty.Messages.ShellTruncation.Render(
			fmt.Sprintf("… %d more lines", truncatedCount),
		))
		return header + "\n" + body.String()
	}

	result := header + "\n" + strings.TrimRight(body.String(), "\n")

	// While streaming, keep the spinner pinned below the latest output.
	if s.pending {
		result += "\n" + s.anim.Render()
	}

	return result
}

func firstLines(s string, count int) string {
	if count <= 0 {
		return ""
	}
	end := 0
	for i := range count {
		next := strings.IndexByte(s[end:], '\n')
		if next < 0 {
			return s
		}
		end += next
		if i == count-1 {
			return s[:end]
		}
		end++
	}
	return s
}

func lastLines(s string, count int) string {
	if count <= 0 {
		return ""
	}
	start := len(s)
	for range count {
		previous := strings.LastIndexByte(s[:start], '\n')
		if previous < 0 {
			return s
		}
		start = previous
	}
	return s[start+1:]
}
