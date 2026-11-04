package model

import (
	"errors"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestSavedPlanPreviewAndBack(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.chat.SetSize(70, 8)
	original := chat.NewAssistantMessageItem(u.com.Styles, &message.Message{
		ID: "original", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("Conversation\n\n", 30)}, message.Finish{Reason: message.FinishReasonEndTurn, Time: 1}},
	})
	u.chat.SetMessages(original)
	u.chat.ScrollBy(4)
	idx, line := u.chat.ScrollPosition()
	u.textarea.SetValue("Unsent draft")
	source := dialog.NewPlans(u.com)
	u.dialog.OpenDialog(source)
	require.Nil(t, u.showSavedPlan(dialog.PlanReadMsg{Owner: source, Name: "план.md", Content: "# Saved plan\n\n" + strings.Repeat("A step\n\n", 30)}))
	require.False(t, u.dialog.HasDialogs())
	require.NotNil(t, u.planPreview)
	require.Same(t, original, u.chat.MessageItem("original"))
	require.Contains(t, ansi.Strip(u.planPreview.chat.MessageItem("saved-plan-preview").RawRender(70)), "A step")
	u.planPreview.chat.SetSize(70, 8)
	handled, _ := u.handlePlanPreviewInput(tea.KeyPressMsg{Code: tea.KeyPgDown})
	require.True(t, handled)
	previewIdx, previewLine := u.planPreview.chat.ScrollPosition()
	require.True(t, previewIdx > 0 || previewLine > 0)
	handled, _ = u.handlePlanPreviewInput(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.True(t, handled)
	require.Equal(t, "Unsent draft", u.textarea.Value())
	handled, _ = u.handlePlanPreviewInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.True(t, handled)
	require.Nil(t, u.planPreview)
	require.Same(t, source, u.dialog.Dialog(dialog.PlansID))
	gotIdx, gotLine := u.chat.ScrollPosition()
	require.Equal(t, idx, gotIdx)
	require.Equal(t, line, gotLine)
}

func TestSavedPlanPreviewErrorsAndStaleReads(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "")
	source := dialog.NewPlans(u.com)
	require.Nil(t, u.showSavedPlan(dialog.PlanReadMsg{Owner: source, Content: "stale"}))
	require.Nil(t, u.planPreview)
	u.dialog.OpenDialog(source)
	cmd := u.showSavedPlan(dialog.PlanReadMsg{Owner: source, Err: errors.New("missing plan")})
	require.Contains(t, cmd().(util.InfoMsg).Msg, "missing plan")
	require.Nil(t, u.planPreview)
	require.Same(t, source, u.dialog.Dialog(dialog.PlansID))
}

func TestSavedPlanPreviewDrawAndResize(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "")
	u.isCompact = true
	u.status = NewStatus(u.com, u)
	u.header = newHeader(u.com)
	source := dialog.NewPlans(u.com)
	u.dialog.OpenDialog(source)
	u.showSavedPlan(dialog.PlanReadMsg{Owner: source, Name: "2026-план.md", Content: "# Preview\n\nSaved text"})
	for _, size := range []image.Point{{80, 30}, {32, 15}, {12, 6}} {
		u.layout.main = image.Rect(0, 0, size.X, max(0, size.Y-3))
		u.layout.editor = image.Rect(0, max(0, size.Y-3), size.X, max(0, size.Y-1))
		u.layout.status = image.Rect(0, max(0, size.Y-1), size.X, size.Y)
		scr := uv.NewScreenBuffer(size.X, size.Y)
		require.NotPanics(t, func() { u.drawSavedPlan(scr, image.Rect(0, 0, size.X, size.Y)) })
		if size.X == 80 {
			rendered := ansi.Strip(scr.Render())
			require.Contains(t, rendered, "Saved text")
			require.Contains(t, rendered, "Esc back to My plans")
		}
	}
}
