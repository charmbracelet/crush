package chat

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// PendingAssistantID is the placeholder spinner's id.
const PendingAssistantID = "pending-assistant"

// newTurnAnim builds the working spinner shown while a turn is in flight,
// suffixed with that session's elapsed time. The placeholder and the
// assistant message that replaces it share this so the handover between
// them is invisible.
func newTurnAnim(sty *styles.Styles, id, sessionID string) *anim.Anim {
	return anim.New(anim.Settings{
		ID:          id,
		Size:        15,
		GradColorA:  sty.WorkingGradFromColor,
		GradColorB:  sty.WorkingGradToColor,
		LabelColor:  sty.WorkingLabelColor,
		CycleColors: true,
		Suffix: func() string {
			return common.Elapsed(sessionID)
		},
		SuffixColor: sty.WorkingTimerColor,
	})
}

// PendingAssistantItem holds the turn spinner between a prompt being sent
// and the assistant message that owns the animation arriving.
type PendingAssistantItem struct {
	*list.Versioned
	sty  *styles.Styles
	anim *anim.Anim
}

var (
	_ MessageItem = (*PendingAssistantItem)(nil)
	_ Animatable  = (*PendingAssistantItem)(nil)
)

// NewPendingAssistantItem creates the placeholder spinner for a turn sent
// in sessionID.
func NewPendingAssistantItem(sty *styles.Styles, sessionID string) *PendingAssistantItem {
	return &PendingAssistantItem{
		Versioned: list.NewVersioned(),
		sty:       sty,
		anim:      newTurnAnim(sty, PendingAssistantID, sessionID),
	}
}

// ID implements MessageItem.
func (p *PendingAssistantItem) ID() string { return PendingAssistantID }

// FilterValue implements list.Item.
func (p *PendingAssistantItem) FilterValue() string { return "" }

// Finished implements list.Item. Never finished; it is removed instead.
func (p *PendingAssistantItem) Finished() bool { return false }

// Spinning implements [Animatable].
func (p *PendingAssistantItem) Spinning() bool { return true }

// Advance implements [Animatable].
func (p *PendingAssistantItem) Advance() bool {
	if !p.anim.Advance() {
		return false
	}
	// A frame changes the render but not the list cache's content hashes.
	p.Bump()
	return true
}

// RawRender implements list.RawRenderable.
func (p *PendingAssistantItem) RawRender(int) string { return p.anim.Render() }

// Render implements list.Item.
func (p *PendingAssistantItem) Render(width int) string {
	prefix := p.sty.Messages.AssistantBlurred.Render()
	lines := strings.Split(p.RawRender(width), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
