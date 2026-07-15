package chat

import (
	"fmt"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestAssistantMessageItem_PlanCardHasUniformBackground(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "plan-card",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "# Plan\n\nPlain **bold** and *italic* with [link](https://example.com), `code`, and 👩‍💻.\n\n- first item\n- second item\n\n```go\nfmt.Println(\"hi\")\n```\n\n<!-- CRUSH_PLAN_READY -->"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 72
	first := item.RawRender(width)
	second := item.RawRender(width)
	require.Equal(t, first, second, "a cached plan render must be byte-stable")
	require.Equal(t, [2]int{73, 12}, [2]int{lipgloss.Width(first), lipgloss.Height(first)},
		"background composition must preserve the existing card geometry")

	scr := renderANSIToScreen(first)

	wantBackground := sty.Messages.PlanBox.GetBackground()
	var foundBold, foundItalic, foundLink, foundEmoji bool
	for y, line := range scr.Lines {
		for x, cell := range line {
			if cell.Width == 0 {
				continue
			}
			requireColorEqual(t, wantBackground, cell.Style.Bg,
				fmt.Sprintf("plan-card cell %q at (%d,%d) must share the card background", cell.Content, x, y))
			foundBold = foundBold || cell.Content == "b" && cell.Style.Attrs&uv.AttrBold != 0
			foundItalic = foundItalic || cell.Content == "i" && cell.Style.Attrs&uv.AttrItalic != 0
			foundLink = foundLink || cell.Link.URL == "https://example.com"
			foundEmoji = foundEmoji || cell.Content == "👩‍💻"
		}
	}

	require.True(t, foundBold, "bold Markdown styling must survive background composition")
	require.True(t, foundItalic, "italic Markdown styling must survive background composition")
	require.True(t, foundLink, "Markdown hyperlinks must survive background composition")
	require.True(t, foundEmoji, "Unicode grapheme clusters must survive background composition")
}

func TestAssistantMessageItem_NonPlanRepliesHaveNoPlanCard(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tests := []struct {
		name string
		text string
	}{
		{name: "ordinary reply", text: "Ordinary assistant reply."},
		{name: "marker mentioned in prose", text: "The marker <!-- CRUSH_PLAN_READY --> is not on its own line."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := &message.Message{
				ID:   tt.name,
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.TextContent{Text: tt.text},
					message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
				},
			}
			item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
			rendered := item.RawRender(72)

			require.Equal(t, 1, lipgloss.Height(rendered), "non-plan replies must not receive card padding")
			for _, line := range renderANSIToScreen(rendered).Lines {
				for _, cell := range line {
					if cell.Width > 0 && cell.Content != " " {
						require.False(t, colorsEqual(sty.Messages.PlanBox.GetBackground(), cell.Style.Bg),
							"non-plan reply text must not receive the plan-card background")
					}
				}
			}
		})
	}
}

func TestAssistantMessageItem_SelectionOverridesPlanBackground(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "selected-plan",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Plan text.\n\n<!-- CRUSH_PLAN_READY -->"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	base := renderANSIToScreen(item.RawRender(72))

	var targetX, targetY int
	found := false
	for y, line := range base.Lines {
		for x, cell := range line {
			if cell.Content == "P" {
				targetX, targetY, found = x, y, true
				break
			}
		}
		if found {
			break
		}
	}
	require.True(t, found)

	item.SetHighlight(
		targetY,
		targetX+MessageLeftPaddingTotal,
		targetY,
		targetX+MessageLeftPaddingTotal+1,
	)
	selected := renderANSIToScreen(item.RawRender(72))
	requireColorEqual(t, sty.TextSelection.GetBackground(), selected.CellAt(targetX, targetY).Style.Bg,
		"text selection must remain the final background layer")
}

func renderANSIToScreen(rendered string) uv.ScreenBuffer {
	width := lipgloss.Width(rendered)
	height := lipgloss.Height(rendered)
	scr := uv.NewScreenBuffer(width, height)
	uv.NewStyledString(rendered).Draw(scr, uv.Rect(0, 0, width, height))
	return scr
}

func requireColorEqual(t *testing.T, want, got color.Color, msg string) {
	t.Helper()
	require.NotNil(t, want)
	require.NotNil(t, got, msg)
	require.True(t, colorsEqual(want, got), msg)
}

func colorsEqual(left, right color.Color) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftR, leftG, leftB, leftA := left.RGBA()
	rightR, rightG, rightB, rightA := right.RGBA()
	return [4]uint32{leftR, leftG, leftB, leftA} == [4]uint32{rightR, rightG, rightB, rightA}
}
