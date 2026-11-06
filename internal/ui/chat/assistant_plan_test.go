package chat

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// The plan card paints no background of its own: ordinary cells expose the
// terminal behind the card, while intentional backgrounds — the inline-code
// chip and the H1 badge above all — survive the composition.
func TestAssistantMessageItem_PlanCardKeepsIntentionalBackgrounds(t *testing.T) {
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
	require.LessOrEqual(t, lipgloss.Width(first), cappedMessageWidth(width),
		"plan card must fit the available message content width")
	require.LessOrEqual(t, lipgloss.Width(item.Render(width)), width,
		"prefixed plan card must fit the available item width")

	scr := renderANSIToScreen(first)

	require.NotNil(t, sty.PlanMarkdown.Code.BackgroundColor, "inline code must declare a chip background")
	codeBackground := lipgloss.Color(*sty.PlanMarkdown.Code.BackgroundColor)
	require.NotNil(t, sty.PlanMarkdown.H1.BackgroundColor, "H1 must declare a badge background")
	h1Background := lipgloss.Color(*sty.PlanMarkdown.H1.BackgroundColor)
	var foundBold, foundItalic, foundLink, foundEmoji, foundCodeChip bool
	for y, line := range scr.Lines {
		for x, cell := range line {
			if cell.Width == 0 {
				continue
			}
			isCodeChip := colorsEqual(codeBackground, cell.Style.Bg)
			isH1Badge := colorsEqual(h1Background, cell.Style.Bg)
			if !isCodeChip && !isH1Badge {
				require.Nil(t, cell.Style.Bg,
					fmt.Sprintf("plan-card cell %q at (%d,%d) must carry no background or an intentional one", cell.Content, x, y))
			}
			foundCodeChip = foundCodeChip || isCodeChip
			foundBold = foundBold || cell.Content == "b" && cell.Style.Attrs&uv.AttrBold != 0
			foundItalic = foundItalic || cell.Content == "i" && cell.Style.Attrs&uv.AttrItalic != 0
			foundLink = foundLink || cell.Link.URL == "https://example.com"
			foundEmoji = foundEmoji || cell.Content == "👩‍💻"
		}
	}

	require.True(t, foundCodeChip, "inline code must keep its chip background inside the plan card")
	require.True(t, foundBold, "bold Markdown styling must survive background composition")
	require.True(t, foundItalic, "italic Markdown styling must survive background composition")
	require.True(t, foundLink, "Markdown hyperlinks must survive background composition")
	require.True(t, foundEmoji, "Unicode grapheme clusters must survive background composition")
}

// The plan box must be sized so its content area matches the width glamour
// wrapped at. In lipgloss v2 Width is the total box width (border and
// padding live inside it); sizing the box to the inner width makes lipgloss
// re-wrap every glamour line narrower, spilling a word or two per line.
func TestAssistantMessageItem_PlanCardDoesNotReWrapContent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	paragraph := strings.TrimSpace(strings.Repeat("lorem ipsum dolor sit amet ", 30))
	msg := &message.Message{
		ID:   "plan-wrap",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: paragraph + "\n\n<!-- CRUSH_PLAN_READY -->"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 72
	_, innerWidth := planBoxLayout(sty.Messages.PlanBox, cappedMessageWidth(width))

	renderer := common.PlanMarkdownRenderer(&sty, innerWidth)
	mu := common.LockMarkdownRenderer(renderer)
	mu.Lock()
	glamourOut, err := renderer.Render(paragraph)
	mu.Unlock()
	require.NoError(t, err)
	var expected []string
	for line := range strings.Lines(strings.TrimSpace(ansi.Strip(glamourOut))) {
		line = strings.TrimSuffix(line, "\n")
		expected = append(expected, strings.TrimRight(line, " "))
	}

	var actual []string
	for line := range strings.Lines(ansi.Strip(item.RawRender(width))) {
		line = strings.TrimSuffix(line, "\n")
		content, ok := strings.CutPrefix(line, "│  ")
		if !ok {
			continue
		}
		content, ok = strings.CutSuffix(content, "  │")
		if !ok {
			continue
		}
		if content = strings.TrimRight(content, " "); content != "" {
			actual = append(actual, content)
		}
	}

	require.Equal(t, expected, actual,
		"plan card content must match glamour's wrapping exactly; a mismatch means the box re-wrapped it")
}

func TestAssistantMessageItem_PlanCardFitsAvailableWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "responsive-plan-card",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "# Plan\n\nA deliberately long plan line with `code`, a [link](https://example.com), and Unicode 👩‍💻.\n\n<!-- CRUSH_PLAN_READY -->"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	for _, width := range []int{24, 72, 140} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			t.Parallel()
			item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
			raw := item.RawRender(width)
			rendered := item.Render(width)

			require.LessOrEqual(t, lipgloss.Width(raw), cappedMessageWidth(width))
			require.LessOrEqual(t, lipgloss.Width(rendered), width)
		})
	}
}

func TestAssistantMessageItem_PlanStreamingCardIsOpenUntilMarker(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	mk := func(text string, finished bool) *message.Message {
		parts := []message.ContentPart{message.TextContent{Text: text}}
		if finished {
			parts = append(parts, message.Finish{Reason: message.FinishReasonEndTurn, Time: 1})
		}
		return &message.Message{
			ID:    "plan-streaming-" + fmt.Sprintf("%t", finished),
			Role:  message.Assistant,
			Parts: parts,
		}
	}

	streaming := NewAssistantMessageItem(&sty, mk("<!-- CRUSH_PLAN_START -->\n\n# Plan\n\nIn progress", false)).(*AssistantMessageItem)
	streaming.SetPlanAgent(true)
	got := streaming.RawRender(72)
	require.Contains(t, got, "╭", "streaming plan must draw the top border")
	require.Contains(t, got, "│", "streaming plan must draw the side borders")
	require.NotContains(t, got, "╰", "streaming plan must withhold the bottom border")
	require.NotContains(t, got, "CRUSH_PLAN_START", "the start marker must be stripped from the card")

	done := NewAssistantMessageItem(&sty, mk("<!-- CRUSH_PLAN_START -->\n\n# Plan\n\nDone\n\n<!-- CRUSH_PLAN_READY -->", true)).(*AssistantMessageItem)
	done.SetPlanAgent(true)
	closed := done.RawRender(72)
	require.Contains(t, closed, "╭", "finished plan must draw the top border")
	require.Contains(t, closed, "╰", "finished plan must close the bottom border")
	require.NotContains(t, closed, "CRUSH_PLAN_START", "the start marker must be stripped from the card")

	intermediate := NewAssistantMessageItem(&sty, mk("Let me look around.", false)).(*AssistantMessageItem)
	intermediate.SetPlanAgent(true)
	plain := intermediate.RawRender(72)
	require.NotContains(t, plain, "╭", "an intermediate plan-mode reply must not render a card")
	require.NotContains(t, plain, "│", "an intermediate plan-mode reply must not draw side borders")

	interrupted := NewAssistantMessageItem(&sty, mk("<!-- CRUSH_PLAN_START -->\n\nHalf a plan", true)).(*AssistantMessageItem)
	interrupted.SetPlanAgent(true)
	cut := interrupted.RawRender(72)
	require.NotContains(t, cut, "╭", "a plan that never reached the ready marker must not render a card")
	require.NotContains(t, cut, "CRUSH_PLAN_START", "the start marker must not leak into rendered output")
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
			require.NotContains(t, rendered, "╭", "non-plan replies must not receive the plan-card border")
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
