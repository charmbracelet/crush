package model

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

type statusCoveringDialog struct{}

func (statusCoveringDialog) ID() string { return "status-covering" }

func (statusCoveringDialog) HandleMsg(tea.Msg) dialog.Action { return nil }

func (statusCoveringDialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	row := image.Rect(area.Min.X, area.Max.Y-1, area.Max.X, area.Max.Y)
	uv.NewStyledString(strings.Repeat("x", row.Dx())).Draw(scr, row)
	return nil
}

func TestDraw_OnboardingStatusRendersAboveDialog(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.state = uiOnboarding
	u.width = 60
	u.height = 20
	u.header = newHeader(u.com)
	u.dialog = dialog.NewOverlay(statusCoveringDialog{})
	u.status.SetInfoMsg(util.InfoMsg{
		Type: util.InfoTypeUpdate,
		Msg:  "Update ready",
	})

	canvas := uv.NewScreenBuffer(u.width, u.height)
	u.Draw(canvas, canvas.Bounds())
	require.Contains(t, canvas.Render(), "Update ready")
}
