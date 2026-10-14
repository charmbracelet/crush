package model

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/chat"
)

func newTimestampTestUI(t *testing.T) (*UI, *testWorkspace, chat.MessageItem) {
	t.Helper()

	u := newFrameTestUI(t)
	ws := u.com.Workspace.(*testWorkspace)
	ws.cfg = &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	ws.cfg.NormalizeOptions()
	completedAt := time.Date(2026, time.January, 2, 23, 23, 0, 0, time.Local)
	msg := &message.Message{
		ID:       "assistant",
		Role:     message.Assistant,
		Provider: "test",
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn, Time: completedAt.Unix()},
		},
	}
	item := chat.NewAssistantInfoItem(u.com.Styles, msg, ws.cfg, completedAt.Add(-4*time.Second))
	u.chat.SetMessages(item)
	u.chat.list.SetSize(120, 5)
	return u, ws, item
}

func TestTimestampConfigRefreshesCachedFooters(t *testing.T) {
	t.Parallel()

	u, ws, item := newTimestampTestUI(t)
	u.chat.ScrollToTop()
	require.Contains(t, ansi.Strip(u.chat.list.Render()), "in 4s • 11:23 PM")
	before := u.chat.RenderState()
	oldConfig := ws.cfg

	for _, tt := range []struct {
		format config.TimestampFormat
		want   string
	}{
		{format: config.TimestampFormat24Hour, want: "in 4s • 23:23"},
		{format: config.TimestampFormatOff, want: "Unknown Model via test in 4s"},
		{format: config.TimestampFormat12Hour, want: "in 4s • 11:23 PM"},
	} {
		cfg := &config.Config{
			Providers: ws.cfg.Providers,
			Options:   &config.Options{TUI: &config.TUIOptions{Timestamps: tt.format}},
		}
		cfg.NormalizeOptions()
		ws.cfg = cfg
		u.Update(pubsub.Event[proto.ConfigChanged]{})
		out := ansi.Strip(u.chat.list.Render())
		require.Contains(t, out, tt.want)
		if tt.format == config.TimestampFormatOff {
			require.NotContains(t, out, "•")
		}
		require.Same(t, item, u.chat.MessageItem(item.ID()))
		require.Equal(t, before.OffsetIdx, u.chat.RenderState().OffsetIdx)
		require.Equal(t, before.OffsetLine, u.chat.RenderState().OffsetLine)
		require.False(t, u.chat.Follow(), "reloading must preserve scrolled-back mode")
	}
	require.Equal(t, config.TimestampFormat12Hour, oldConfig.TimestampFormat(), "reloads must retain immutable old snapshots")
}

func TestTimestampConfigRefreshKeepsFollowAtBottom(t *testing.T) {
	t.Parallel()

	u, ws, _ := newTimestampTestUI(t)
	u.chat.list.SetSize(36, 1)
	u.chat.ScrollToBottom()
	require.Greater(t, u.chat.RenderState().OffsetLine, 0, "the footer must wrap beyond the viewport")
	require.Contains(t, ansi.Strip(u.chat.list.Render()), "11:23 PM")

	ws.cfg = &config.Config{
		Providers: ws.cfg.Providers,
		Options:   &config.Options{TUI: &config.TUIOptions{Timestamps: config.TimestampFormatOff}},
	}
	u.Update(pubsub.Event[proto.ConfigChanged]{})

	require.Contains(t, ansi.Strip(u.chat.list.Render()), "Unknown Model via test in 4s")
	require.Zero(t, u.chat.RenderState().OffsetLine)
	require.True(t, u.chat.Follow())
}
