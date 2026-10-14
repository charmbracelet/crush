package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

func newTestAssistantInfoItem(msg *message.Message, lastUserMessageTime time.Time) *AssistantInfoItem {
	sty := styles.CharmtonePantera()
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	return NewAssistantInfoItem(&sty, msg, cfg, lastUserMessageTime).(*AssistantInfoItem)
}

func TestAssistantInfoItem_CompletionTimestamp(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		hour int
		min  int
		want string
	}{
		{name: "evening", hour: 23, min: 23, want: "11:23 PM"},
		{name: "morning", hour: 9, min: 7, want: "9:07 AM"},
		{name: "midnight", hour: 0, min: 0, want: "12:00 AM"},
		{name: "noon", hour: 12, min: 0, want: "12:00 PM"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			completedAt := time.Date(2026, time.January, 2, tt.hour, tt.min, 45, 0, time.Local)
			msg := &message.Message{
				ID:        "assistant",
				Role:      message.Assistant,
				Provider:  "test",
				CreatedAt: completedAt.Add(-time.Minute).Unix(),
				UpdatedAt: completedAt.Add(time.Hour).Unix(),
				Parts: []message.ContentPart{
					message.Finish{Reason: message.FinishReasonEndTurn, Time: completedAt.Unix()},
				},
			}
			item := newTestAssistantInfoItem(msg, completedAt.Add(-4*time.Second))
			out := ansi.Strip(item.Render(120))

			require.Contains(t, out, "Unknown Model via test in 4s • "+tt.want)
			require.Equal(t, out, ansi.Strip(item.Render(120)), "cached renders must retain the completion time")
		})
	}
}

func TestAssistantInfoItem_MissingCompletionTimestamp(t *testing.T) {
	t.Parallel()

	lastUserMessageTime := time.Date(2026, time.January, 2, 23, 22, 56, 0, time.Local)
	for _, timestamp := range []int64{0, -1} {
		msg := &message.Message{
			ID:       "assistant",
			Role:     message.Assistant,
			Provider: "test",
			Parts: []message.ContentPart{
				message.Finish{Reason: message.FinishReasonEndTurn, Time: timestamp},
			},
		}
		item := newTestAssistantInfoItem(msg, lastUserMessageTime)
		out := ansi.Strip(item.Render(120))

		require.Contains(t, out, "Unknown Model via test")
		require.NotContains(t, out, " in ", "missing timestamps must not display a duration")
		require.NotContains(t, out, "•", "missing timestamps must not display an epoch time")
	}
}

func TestAssistantInfoItem_TimestampConfigChanges(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, time.January, 2, 23, 23, 0, 0, time.Local)
	msg := &message.Message{
		ID:       "assistant",
		Role:     message.Assistant,
		Provider: "test",
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn, Time: completedAt.Unix()},
		},
	}
	item := newTestAssistantInfoItem(msg, completedAt.Add(-4*time.Second))
	require.Contains(t, ansi.Strip(item.Render(120)), "in 4s • 11:23 PM")

	for _, tt := range []struct {
		format config.TimestampFormat
		want   string
	}{
		{format: config.TimestampFormat24Hour, want: "in 4s • 23:23"},
		{format: config.TimestampFormatOff, want: "Unknown Model via test in 4s"},
		{format: config.TimestampFormat12Hour, want: "in 4s • 11:23 PM"},
	} {
		cfg := &config.Config{
			Providers: item.cfg.Providers,
			Options:   &config.Options{TUI: &config.TUIOptions{Timestamps: tt.format}},
		}
		version := item.Version()
		item.SetConfig(cfg)
		require.Greater(t, item.Version(), version, "a changed format must invalidate the list cache")
		out := ansi.Strip(item.Render(120))
		require.Contains(t, out, tt.want)
		if tt.format == config.TimestampFormatOff {
			require.NotContains(t, out, "•")
		}
		version = item.Version()
		item.SetConfig(cfg)
		require.Equal(t, version, item.Version(), "unchanged formats must retain cached output")
	}
}

func TestAssistantInfoItem_CompletionBeforePrompt(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, time.January, 2, 23, 23, 0, 0, time.Local)
	msg := &message.Message{
		ID:       "assistant",
		Role:     message.Assistant,
		Provider: "test",
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn, Time: completedAt.Unix()},
		},
	}
	item := newTestAssistantInfoItem(msg, completedAt.Add(4*time.Second))
	out := ansi.Strip(item.Render(120))

	require.Contains(t, out, "Unknown Model via test • 11:23 PM")
	require.NotContains(t, out, " in ", "completion before the prompt must not display a negative duration")
}

func TestAssistantInfoItem_WrapsCompletionFooter(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, time.January, 2, 23, 23, 0, 0, time.Local)
	savings := 13.0
	msg := &message.Message{
		ID:                      "assistant",
		Role:                    message.Assistant,
		Provider:                "test",
		PrismModelName:          "模型 " + strings.Repeat("long-model-name", 8),
		PrismHypercreditSavings: &savings,
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn, Time: completedAt.Unix()},
		},
	}
	item := newTestAssistantInfoItem(msg, completedAt.Add(-4*time.Second))

	for _, width := range []int{120, 80, 60, 40, 12, 4, 0, 1, 2, 120} {
		out := item.Render(width)
		if width <= MessageLeftPaddingTotal {
			require.Empty(t, out)
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			require.LessOrEqual(t, ansi.StringWidth(line), width, "footer exceeds width %d: %q", width, line)
		}
		text := strings.Join(strings.Fields(ansi.Strip(out)), "")
		require.Contains(t, text, "in4s•11:23PM")
		require.Contains(t, text, "◆13Saved")
		require.Contains(t, text, strings.ReplaceAll(msg.PrismModelName, " ", ""))
	}
}

func TestAssistantInfoItem_RestoredCompletionTimestamp(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	queries := db.New(conn)
	sess, err := session.NewService(queries, conn).Create(t.Context(), "test")
	require.NoError(t, err)
	messages := message.NewService(queries, message.WithDebounce(0))
	msg, err := messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:     message.Assistant,
		Provider: "test",
	})
	require.NoError(t, err)
	completedAt := time.Date(2024, time.January, 2, 23, 23, 0, 0, time.Local)
	msg.Parts = append(msg.Parts, message.Finish{Reason: message.FinishReasonEndTurn, Time: completedAt.Unix()})
	require.NoError(t, messages.Update(t.Context(), msg))

	restored, err := message.NewService(queries).Get(t.Context(), msg.ID)
	require.NoError(t, err)
	item := newTestAssistantInfoItem(&restored, completedAt.Add(-4*time.Second))

	require.Contains(t, ansi.Strip(item.Render(120)), "in 4s • 11:23 PM")
}
