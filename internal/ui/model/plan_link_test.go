package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestPlanFileDelayedClick(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"click", "outside", "double", "drag", "holding", "scrolled"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			u, _ := newPlanUI(t, "sess-1")
			m := u.chat
			m.SetSize(72, 50)
			msg := message.Message{ID: "p", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "# Plan\n\n" + strings.Repeat("step\n\n", 12) + "<!-- CRUSH_PLAN_READY -->"}, message.Finish{Reason: message.FinishReasonEndTurn, Time: 1}}}
			item := chat.ExtractMessageItems(u.com.Styles, &msg, nil, "/tmp/project")[0]
			item.(*chat.AssistantMessageItem).SetPlanFileLink(".crush/plans/a.md", "/tmp/project/.crush/plans/a.md")
			m.SetMessages(item)
			if scenario == "scrolled" {
				m.SetSize(72, 8)
				m.ScrollToBottom()
			}
			rendered := strings.Split(ansi.Strip(m.list.Render()), "\n")
			x, y := -1, -1
			for row, line := range rendered {
				if at := strings.Index(line, "Saved to"); at >= 0 {
					x, y = at, row
					break
				}
			}
			require.GreaterOrEqual(t, x, 0)
			if scenario == "outside" {
				x = 0
			}
			handled, cmd := m.HandleMouseDown(x, y)
			require.True(t, handled)
			require.NotNil(t, cmd)
			pending := cmd().(DelayedClickMsg)
			switch scenario {
			case "double":
				m.HandleMouseDown(x, y)
			case "drag":
				m.HandleMouseDrag(x+4, y)
			}
			if scenario != "holding" {
				m.HandleMouseUp(x, y)
			}
			handled, open := m.HandleDelayedClick(pending)
			if scenario == "click" || scenario == "scrolled" {
				require.True(t, handled)
				require.NotNil(t, open)
				_, again := m.HandleDelayedClick(pending)
				require.Nil(t, again)
			} else {
				require.Nil(t, open)
			}
		})
	}
}

func TestOpenPlanFileCommand(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		called := false
		cmd := openPlanFile("/tmp/план with spaces.md", func(path string) error {
			called = true
			require.Equal(t, "/tmp/план with spaces.md", path)
			if fail {
				return errors.New("missing file")
			}
			return nil
		})
		require.False(t, called)
		msg := cmd()
		require.True(t, called)
		if fail {
			require.IsType(t, util.InfoMsg{}, msg)
			require.Contains(t, msg.(util.InfoMsg).Msg, "missing file")
		} else {
			require.Nil(t, msg)
		}
	}
}
