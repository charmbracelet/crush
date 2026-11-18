package chat

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// PendingAssistantID is the placeholder spinner's id.
const PendingAssistantID = "pending-assistant"

// PendingUserID is the id of the prompt echo shown until the stored user
// message arrives.
const PendingUserID = "pending-user"

// NewPendingUserItem echoes a prompt into the transcript the moment it is
// sent, instead of waiting for the agent to store it and publish it back.
// It is an ordinary user message item over an unsaved message, so the swap
// for the stored one is invisible.
func NewPendingUserItem(sty *styles.Styles, content string, atts []message.Attachment) MessageItem {
	parts := []message.ContentPart{message.TextContent{Text: content}}
	for _, at := range atts {
		parts = append(parts, message.BinaryContent{
			Path:     at.FilePath,
			MIMEType: at.MimeType,
			Data:     at.Content,
		})
	}
	return NewUserMessageItem(sty, &message.Message{
		ID:        PendingUserID,
		Role:      message.User,
		Parts:     parts,
		CreatedAt: time.Now().Unix(),
	}, newAttachmentRenderer(sty))
}

// newTurnAnim builds the working spinner shown while a turn is in flight,
// suffixed with suffix. The placeholder and the assistant message share it
// so the handover between them is invisible.
func newTurnAnim(sty *styles.Styles, id string, suffix func() string) *anim.Anim {
	return anim.New(anim.Settings{
		ID:          id,
		Size:        15,
		GradColorA:  sty.WorkingGradFromColor,
		GradColorB:  sty.WorkingGradToColor,
		LabelColor:  sty.ThinkingLabelColor,
		CycleColors: true,
		Suffix:      suffix,
		SuffixColor: sty.WorkingTimerColor,
	})
}

// turnTimerSlot lets an item's spinner show a session's turn time while the
// UI has handed it the timer.
type turnTimerSlot struct{ sessionID string }

// set hands the timer to sessionID, or takes it away when empty, and
// reports whether that changed anything.
func (s *turnTimerSlot) set(sessionID string) bool {
	changed := s.sessionID != sessionID
	s.sessionID = sessionID
	return changed
}

func (s *turnTimerSlot) text() string {
	if s.sessionID == "" {
		return ""
	}
	return common.Elapsed(s.sessionID)
}

// PendingAssistantItem is the turn timer at the tail of the transcript: a
// spinner while tokens stream or are awaited, otherwise "Running" and the
// turn's elapsed time.
type PendingAssistantItem struct {
	*list.Versioned
	sty       *styles.Styles
	anim      *anim.Anim
	sessionID string
	live      bool
	// shown is the timer text as of the last Advance, to tell when it ticks.
	shown string
}

var (
	_ MessageItem  = (*PendingAssistantItem)(nil)
	_ Animatable   = (*PendingAssistantItem)(nil)
	_ animRestyler = (*PendingAssistantItem)(nil)
)

// NewPendingAssistantItem creates the placeholder spinner for a turn sent
// in sessionID.
func NewPendingAssistantItem(sty *styles.Styles, sessionID string) *PendingAssistantItem {
	p := &PendingAssistantItem{
		Versioned: list.NewVersioned(),
		sty:       sty,
		sessionID: sessionID,
		live:      true,
	}
	p.anim = newTurnAnim(sty, PendingAssistantID, p.timer)
	return p
}

// Live reports whether the item is spinning rather than showing "Running".
func (p *PendingAssistantItem) Live() bool { return p.live }

// SetLive spins the item while live tokens stream in, and shows "Running"
// with the elapsed time while nothing does.
func (p *PendingAssistantItem) SetLive(live bool) {
	if p.live != live {
		p.live = live
		p.Bump()
	}
}

// ID implements MessageItem.
func (p *PendingAssistantItem) ID() string { return PendingAssistantID }

// FilterValue implements list.Item.
func (p *PendingAssistantItem) FilterValue() string { return "" }

// Spinning implements [Animatable].
func (p *PendingAssistantItem) Spinning() bool { return true }

// restyleAnim implements [animRestyler]. It mirrors
// [AssistantMessageItem.restyleAnim], which takes the spinner over.
func (p *PendingAssistantItem) restyleAnim() {
	p.anim.SetColors(
		p.sty.ThinkingLabelColor,
		p.sty.WorkingGradFromColor,
		p.sty.WorkingGradToColor,
		p.sty.WorkingTimerColor,
	)
}

// Advance implements [Animatable]. While not live it only changes when its
// timer text does.
func (p *PendingAssistantItem) Advance() bool {
	if !p.live {
		t := p.timer()
		if t == p.shown {
			return false
		}
		p.shown = t
		p.Bump()
		return true
	}
	if !p.anim.Advance() {
		return false
	}
	// A frame changes the render but not the list cache's content hashes.
	p.Bump()
	return true
}

// RawRender implements list.RawRenderable.
func (p *PendingAssistantItem) RawRender(int) string {
	if p.live {
		return p.anim.Render()
	}
	t := p.timer()
	if t == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(p.sty.WorkingTimerColor).Render("Running " + t)
}

func (p *PendingAssistantItem) timer() string { return common.Elapsed(p.sessionID) }

// Render implements list.Item.
func (p *PendingAssistantItem) Render(width int) string {
	prefix := p.sty.Messages.AssistantBlurred.Render()
	lines := strings.Split(p.RawRender(width), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
