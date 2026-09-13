package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

func benchStyles() *styles.Styles {
	return common.DefaultCommon(nil).Styles
}

// buildSession returns n alternating user/assistant items, each assistant
// carrying a reasoning trace and a text body, as a session that has already
// scrolled well past the viewport.
func buildSession(n int) []chat.MessageItem {
	sty := benchStyles()
	items := make([]chat.MessageItem, 0, n*2)
	for i := range n {
		um := &message.Message{ID: fmt.Sprintf("u%d", i), Role: message.User}
		um.AppendContent(fmt.Sprintf("Question %d: can you walk me through how the cache invalidation works here?", i))
		items = append(items, chat.NewUserMessageItem(sty, um, nil))

		am := &message.Message{ID: fmt.Sprintf("a%d", i), Role: message.Assistant}
		for j := range 20 {
			am.AppendReasoningContent(fmt.Sprintf("Considering angle %d of question %d in some detail.\n", j, i))
		}
		am.FinishThinking()
		am.AppendContent(strings.Repeat("Here is a paragraph of the answer body that wraps across lines. ", 12))
		am.AddFinish(message.FinishReasonEndTurn, "", "")
		items = append(items, chat.NewAssistantMessageItem(sty, am))
	}
	return items
}

func benchDraw(b *testing.B, items []chat.MessageItem, mutate func(i int)) {
	u := newTestUI()
	u.chat.SetMessages(items...)
	u.updateLayoutAndSize()
	const w, h = 140, 40
	scr := uv.NewScreenBuffer(w, h)
	area := uv.Rect(0, 0, w, h)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if mutate != nil {
			mutate(i)
		}
		u.chat.Draw(scr, area)
	}
}

// BenchmarkDrawIdle is the steady-state cost: nothing changed since the
// last frame. This is what a spinner tick costs.
func BenchmarkDrawIdle(b *testing.B) {
	for _, n := range []int{10, 100, 400} {
		b.Run(fmt.Sprintf("msgs=%d", n*2), func(b *testing.B) {
			benchDraw(b, buildSession(n), nil)
		})
	}
}

// BenchmarkDrawStreaming is a frame during an active turn: the last
// assistant message grows by one reasoning delta per frame. The two
// shapes matter because the stable-prefix cache in streamingMarkdown
// advances on blank lines, so a trace with paragraph breaks keeps a
// small trailing partial while one without grows its trail up to
// relaxBoundaryAfter.
func BenchmarkDrawStreaming(b *testing.B) {
	shapes := []struct {
		name       string
		blankEvery int
	}{
		{"no-blank-lines", 0},
		{"blank-every-8", 8},
	}
	for _, sh := range shapes {
		b.Run(sh.name, func(b *testing.B) {
			items := buildSession(10)
			live := &message.Message{ID: "live", Role: message.Assistant}
			item := chat.NewAssistantMessageItem(benchStyles(), live).(*chat.AssistantMessageItem)
			items = append(items, item)
			benchDraw(b, items, func(i int) {
				live.AppendReasoningContent(fmt.Sprintf("Delta %d of the reasoning trace here.\n", i))
				if sh.blankEvery > 0 && i%sh.blankEvery == sh.blankEvery-1 {
					live.AppendReasoningContent("\n")
				}
				item.SetMessage(live)
			})
		})
	}
}
