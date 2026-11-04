package model

import (
	"image"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Saved plan previews leave the conversation and its scroll position intact.
type savedPlanPreview struct {
	name   string
	chat   *Chat
	source *dialog.Plans
}

func (m *UI) showSavedPlan(msg dialog.PlanReadMsg) tea.Cmd {
	if m.dialog.Dialog(dialog.PlansID) != msg.Owner || msg.Owner == nil {
		return nil
	}
	if msg.Err != nil {
		return util.ReportError(msg.Err)
	}
	preview := NewChat(m.com, "always")
	text := message.Message{ID: "saved-plan-preview", Role: message.Assistant, Parts: []message.ContentPart{
		message.TextContent{Text: msg.Content},
		message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
	}}
	preview.SetMessages(chat.NewAssistantMessageItem(m.com.Styles, &text))
	m.planPreview = &savedPlanPreview{name: msg.Name, chat: preview, source: msg.Owner}
	m.dialog.CloseDialog(dialog.PlansID)
	return nil
}

func (m *UI) handlePlanPreviewInput(msg tea.Msg) (bool, tea.Cmd) {
	if m.planPreview == nil || m.dialog.HasDialogs() {
		return false, nil
	}
	preview := m.planPreview.chat
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "alt+esc":
			source := m.planPreview.source
			m.planPreview = nil
			m.dialog.OpenDialog(source)
		case "up", "k":
			return true, preview.ScrollBy(-1)
		case "down", "j":
			return true, preview.ScrollBy(1)
		case "pgup":
			return true, preview.ScrollBy(-max(1, preview.Height()-1))
		case "pgdown", "space":
			return true, preview.ScrollBy(max(1, preview.Height()-1))
		case "home", "g":
			return true, preview.ScrollToTop()
		case "end", "G":
			return true, preview.ScrollToBottom()
		}
		return true, nil
	case common.CoalescedWheelMsg:
		if image.Pt(msg.Mouse.X, msg.Mouse.Y).In(m.layout.main) {
			return true, preview.ScrollBy(int(msg.DeltaY))
		}
		return true, nil
	case tea.MouseWheelMsg:
		if image.Pt(msg.X, msg.Y).In(m.layout.main) {
			if msg.Button == tea.MouseWheelUp {
				return true, preview.ScrollBy(-3)
			}
			if msg.Button == tea.MouseWheelDown {
				return true, preview.ScrollBy(3)
			}
		}
		return true, nil
	case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg, tea.PasteMsg:
		return true, nil
	}
	return false, nil
}

func (m *UI) drawSavedPlan(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if m.state == uiChat && !m.isCompact {
		m.drawSidebar(scr, m.layout.sidebar)
	} else {
		m.drawHeader(scr, m.layout.header)
	}
	main := m.layout.main
	title := m.com.Styles.Resource.Heading.Render(ansi.Truncate(m.planPreview.name, max(0, main.Dx()), "…"))
	uv.NewStyledString(title).Draw(scr, main)
	main.Min.Y = min(main.Max.Y, main.Min.Y+2)
	m.planPreview.chat.SetSize(main.Dx(), main.Dy())
	m.planPreview.chat.Draw(scr, main)
	hint := m.com.Styles.Resource.AdditionalText.Render("Esc back to My plans · ↑/↓ scroll · PgUp/PgDn page")
	uv.NewStyledString(ansi.Truncate(hint, max(0, m.layout.editor.Dx()), "…")).Draw(scr, m.layout.editor)
	m.status.Draw(scr, m.layout.status)
	return m.dialog.Draw(scr, area)
}
