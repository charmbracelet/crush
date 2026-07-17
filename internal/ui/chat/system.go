package chat

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// SystemMessageKind identifies a category of Crush system message. The kind
// doubles as a stable list ID so a given advisory never appears twice and can
// be recomputed in place as state changes.
type SystemMessageKind string

const (
	// SystemMessageContextWarning warns that the active model's context
	// window is too small for Crush to work well.
	SystemMessageContextWarning SystemMessageKind = "context-warning"
	// SystemMessageSuperYolo warns that super yolo (sysadmin) mode
	// auto-approves every command, including dangerous ones.
	SystemMessageSuperYolo SystemMessageKind = "super-yolo"
)

// systemMessageFooter is the label rendered in the footer rule beneath every
// system message.
const systemMessageFooter = "Crush System Message"

// SystemMessageID returns the stable list ID for a system message of the
// given kind.
func SystemMessageID(kind SystemMessageKind) string {
	return fmt.Sprintf("system:%s", kind)
}

// SystemMessageItem is a Crush-generated advisory rendered inline in the chat
// like a message, but never persisted and never sent to the agent. It is a
// pure function of its title and body; callers recompute it from live state
// (model, permission mode) and it self-clears when that state resolves.
type SystemMessageItem struct {
	*list.Versioned
	*cachedMessageItem

	kind  SystemMessageKind
	title string
	body  string
	sty   *styles.Styles
}

var _ MessageItem = (*SystemMessageItem)(nil)

// NewSystemMessageItem creates a system message item. The body must have every
// run pre-styled by the caller (base and accent runs alike): it is only
// reflowed to width at render time, never recolored, so relying on an outer
// foreground would leave text after an accent run uncolored.
func NewSystemMessageItem(sty *styles.Styles, kind SystemMessageKind, title, body string) *SystemMessageItem {
	return &SystemMessageItem{
		Versioned:         list.NewVersioned(),
		cachedMessageItem: &cachedMessageItem{},
		kind:              kind,
		title:             title,
		body:              body,
		sty:               sty,
	}
}

// Kind returns the system message kind.
func (s *SystemMessageItem) Kind() SystemMessageKind {
	return s.kind
}

// Finished implements list.Item. System messages are immutable after
// construction, so the entry is safe to freeze.
func (s *SystemMessageItem) Finished() bool {
	return true
}

// ID implements MessageItem.
func (s *SystemMessageItem) ID() string {
	return SystemMessageID(s.kind)
}

// RawRender implements MessageItem.
func (s *SystemMessageItem) RawRender(width int) string {
	innerWidth := max(0, width-MessageLeftPaddingTotal)
	content, _, ok := s.getCachedRender(innerWidth)
	if !ok {
		content = s.renderContent(innerWidth)
		height := lipgloss.Height(content)
		s.setCachedRender(content, innerWidth, height)
	}
	return content
}

// Render implements MessageItem. It prefixes each line with the shared
// section header inset so the block aligns with assistant messages.
func (s *SystemMessageItem) Render(width int) string {
	if cached, ok := s.getCachedPrefixedRender(width, 0); ok {
		return cached
	}
	prefix := s.sty.Messages.SectionHeader.Render()
	lines := strings.Split(s.RawRender(width), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	out := strings.Join(lines, "\n")
	s.setCachedPrefixedRender(out, width, 0)
	return out
}

func (s *SystemMessageItem) renderContent(width int) string {
	textWidth := min(width, maxTextWidth)

	header := s.sty.Messages.SystemBadge.Render("!") + " " + s.sty.Messages.SystemTitle.Render(s.title)

	// The body arrives with every run already colored (see the builders in
	// model/system_messages.go), so we only reflow it; applying a foreground
	// here would be clobbered by the inner resets that end each colored run.
	// Wrapping is done per paragraph: re-wrapping a segment that already
	// contains newlines misplaces padding, so blank-line separators are kept
	// outside the reflow.
	paragraphs := strings.Split(s.body, "\n\n")
	for i, p := range paragraphs {
		paragraphs[i] = lipgloss.NewStyle().Width(textWidth).Render(p)
	}
	body := strings.Join(paragraphs, "\n\n")

	footerLabel := s.sty.Messages.SystemFooterIcon.Render(styles.ModelIcon) + " " +
		s.sty.Messages.SystemFooterLabel.Render(systemMessageFooter)
	footer := common.Section(s.sty, footerLabel, width)

	return strings.Join([]string{header, "", body, "", footer}, "\n")
}
