package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestAssistantInfoItem_CriticVerdictRendering(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles()
	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}

	msg := &message.Message{
		ID:       "msg-1",
		Role:     message.Assistant,
		Provider: "test",
		Model:    "test-model",
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()},
		},
	}

	t.Run("approve badge", func(t *testing.T) {
		t.Parallel()
		item := NewAssistantInfoItem(&sty, msg, cfg, time.Now().Add(-time.Second)).(*AssistantInfoItem)
		item.SetCriticVerdict("approve")
		rendered := item.renderContent(80)
		require.Contains(t, rendered, styles.CriticApproveIcon)
		require.NotContains(t, rendered, styles.CriticReviseIcon)
		require.NotContains(t, rendered, styles.CriticHaltIcon)
	})

	t.Run("revise badge", func(t *testing.T) {
		t.Parallel()
		item := NewAssistantInfoItem(&sty, msg, cfg, time.Now().Add(-time.Second)).(*AssistantInfoItem)
		item.SetCriticVerdict("revise")
		rendered := item.renderContent(80)
		require.Contains(t, rendered, styles.CriticReviseIcon)
		require.NotContains(t, rendered, styles.CriticApproveIcon)
		require.NotContains(t, rendered, styles.CriticHaltIcon)
	})

	t.Run("halt badge", func(t *testing.T) {
		t.Parallel()
		item := NewAssistantInfoItem(&sty, msg, cfg, time.Now().Add(-time.Second)).(*AssistantInfoItem)
		item.SetCriticVerdict("halt")
		rendered := item.renderContent(80)
		require.Contains(t, rendered, styles.CriticHaltIcon)
		require.NotContains(t, rendered, styles.CriticApproveIcon)
		require.NotContains(t, rendered, styles.CriticReviseIcon)
	})

	t.Run("no verdict", func(t *testing.T) {
		t.Parallel()
		item := NewAssistantInfoItem(&sty, msg, cfg, time.Now().Add(-time.Second)).(*AssistantInfoItem)
		item.SetCriticVerdict("")
		rendered := item.renderContent(80)
		require.NotContains(t, rendered, styles.CriticApproveIcon)
		require.NotContains(t, rendered, styles.CriticReviseIcon)
		require.NotContains(t, rendered, styles.CriticHaltIcon)
	})

	t.Run("unknown verdict ignored", func(t *testing.T) {
		t.Parallel()
		item := NewAssistantInfoItem(&sty, msg, cfg, time.Now().Add(-time.Second)).(*AssistantInfoItem)
		item.SetCriticVerdict("unknown")
		rendered := item.renderContent(80)
		require.NotContains(t, rendered, styles.CriticApproveIcon)
		require.NotContains(t, rendered, styles.CriticReviseIcon)
		require.NotContains(t, rendered, styles.CriticHaltIcon)
	})
}
