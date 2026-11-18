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

func TestAssistantMessageItem_PlanCardBordersAlign(t *testing.T) {
	// Glamour registers code-block styles globally, so keep this matrix serial.

	for name, content := range map[string]string{
		"prose":            strings.Repeat("A plan with `inline code`, **bold**, and [a link](https://example.com). ", 8),
		"long_code":        "```python\n" + strings.Repeat("long_argument_", 20) + "\n```",
		"tabs":             "```python\n\tprint(\"hello\")\n\t\tprint(\"world\")\n```",
		"arrow":            strings.Repeat("User flow: count → prefix prompt. ", 8),
		"dash":             strings.Repeat("User flow: count — prefix prompt. ", 8),
		"cjk":              strings.Repeat("User flow: count 中文 prefix prompt. ", 8),
		"emoji":            "👩‍💻",
		"cancelled_widths": "```text\n👩‍💻©️©️Z\n```",
		"cancelled_faces":  "```text\n👩‍💻☺️☺️Z\n```",
		"empty":            "",
		"oversize_only":    "👨‍👩‍👧‍👦",
		"long_link":        "[L👩‍💻 L👩‍💻 L👩‍💻](https://example.com)",
		"family_emoji":     "👨‍👩‍👧‍👦Z",
		"combining":        "e\u0301Z",
		"narrow_emoji":     "123👩‍💻Z",
		"boundary_emoji":   "1234567890123👩‍💻Z",
	} {
		for _, streaming := range []bool{false, true} {
			for _, width := range []int{8, 12, 13, 16, 20, 24, 72, 128} {
				t.Run(fmt.Sprintf("%s_streaming_%t_width_%d", name, streaming, width), func(t *testing.T) {
					sty := styles.CharmtonePantera()
					msg := &message.Message{
						ID:   "aligned-plan",
						Role: message.Assistant,
						Parts: []message.ContentPart{
							message.TextContent{Text: "<!-- CRUSH_PLAN_START -->\n\n" + content},
						},
					}
					if !streaming {
						msg.Parts[0] = message.TextContent{Text: content + "\n\n<!-- CRUSH_PLAN_READY -->"}
						msg.Parts = append(msg.Parts, message.Finish{Reason: message.FinishReasonEndTurn, Time: 1})
					}
					item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
					item.SetPlanAgent(true)
					rendered := item.RawRender(width)
					if name == "family_emoji" || name == "oversize_only" {
						_, inner := planBoxLayout(sty.Messages.PlanBox, cappedMessageWidth(width))
						if inner >= 8 {
							require.Contains(t, rendered, "👨‍👩‍👧‍👦")
						} else {
							require.NotContains(t, rendered, "👨‍👩‍👧‍👦")
						}
					}
					if name == "boundary_emoji" && width >= 24 {
						require.Contains(t, rendered, "Z")
						require.Contains(t, rendered, "👩‍💻")
					}
					scr := uv.NewScreenBuffer(cappedMessageWidth(width), lipgloss.Height(rendered))
					uv.NewStyledString(rendered).Draw(scr, scr.Bounds())
					if name == "family_emoji" || name == "combining" || name == "narrow_emoji" {
						var screenText strings.Builder
						for _, row := range scr.Lines {
							for _, cell := range row {
								screenText.WriteString(cell.Content)
							}
						}
						require.Contains(t, screenText.String(), "Z")
					}
					linkedLabels := 0
					for row, line := range scr.Lines {
						for _, cell := range line {
							if name == "long_link" && cell.Content == "L" {
								require.Equal(t, "https://example.com", cell.Link.URL)
								linkedLabels++
							}
						}

						want := "│"
						if row == 0 {
							want = "╮"
						}
						if row == len(scr.Lines)-1 && !streaming {
							want = "╯"
						}
						require.Equal(t, want, line[len(line)-1].Content,
							"right border must stay in the last column at row %d", row)
						require.Empty(t, line[len(line)-1].Link.URL, "links must not extend onto the border")
					}
					if name == "long_link" {
						require.Equal(t, 3, linkedLabels)
					}
					for row, line := range strings.Split(rendered, "\n") {
						require.Equal(t, cappedMessageWidth(width), ansi.WcWidth.StringWidth(line),
							"plan row %d must fit the UI screen's width method", row)
					}
				})
			}
		}
	}
}

// A line that only overflows by the UI's cell widths is re-wrapped the way
// glamour wraps: whole words move to the next row, which keeps the line's
// indent.
func TestAssistantMessageItem_PlanCardRewrapKeepsWordsAndIndent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	nested := "nested 👩‍💻 item with more than enough words to wrap a few times"
	msg := &message.Message{
		ID:   "plan-rewrap",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "- top\n  - " + nested + "\n\n<!-- CRUSH_PLAN_READY -->"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	for width := 30; width <= 72; width++ {
		var rows []string
		for line := range strings.Lines(ansi.Strip(NewAssistantMessageItem(&sty, msg).RawRender(width))) {
			row, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "│  ")
			row = strings.TrimRight(strings.TrimSuffix(row, "│"), " ")
			if ok && (len(rows) > 0 || strings.Contains(row, "nested")) && row != "" {
				rows = append(rows, row)
			}
		}
		for _, row := range rows {
			require.True(t, strings.HasPrefix(row, "  "),
				"width %d: row %q must keep the nested item's indent", width, row)
		}
		require.Equal(t, append([]string{"•"}, strings.Fields(nested)...),
			strings.Fields(strings.Join(rows, " ")), "width %d: words must not be split", width)
	}
}

// BenchmarkPlanCardStreaming measures one streaming flush of a long plan. One
// line whose cell width differs from its grapheme width sends the whole card
// through the cell layout.
func BenchmarkPlanCardStreaming(b *testing.B) {
	for _, tc := range []struct{ name, emoji string }{{"plain", ""}, {"emoji", " 👩‍💻"}} {
		b.Run(tc.name, func(b *testing.B) {
			var plan strings.Builder
			plan.WriteString("<!-- CRUSH_PLAN_START -->\n\n# Plan\n\n")
			for i := range 150 {
				fmt.Fprintf(&plan, "## Step %d\n\n- Update the renderer%s so the border stays put across terminals\n"+
					"- Add a test with `inline code` and a [link](https://example.com)\n\n"+
					"Prose explaining why this matters, long enough to wrap in the card.\n\n", i+1, tc.emoji)
			}
			sty := styles.CharmtonePantera()
			msg := &message.Message{
				ID:    "plan-bench",
				Role:  message.Assistant,
				Parts: []message.ContentPart{message.TextContent{Text: plan.String()}},
			}
			item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
			item.SetPlanAgent(true)
			_ = item.RawRender(120)
			for i := 0; b.Loop(); i++ {
				msg.Parts[0] = message.TextContent{Text: plan.String() + strings.Repeat("x", i%8+1)}
				_ = item.RawRender(120)
			}
		})
	}
}
