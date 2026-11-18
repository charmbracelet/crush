package chat

import (
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestModelDisplayNameFallbacks(t *testing.T) {
	// modelDisplayName resolves the name shown in the assistant info
	// footer. Config wins (renames show immediately), the persisted
	// name covers renamed or removed models, the model ID is the last
	// identifiable fallback, and "Unknown Model" only appears when the
	// message carries no model at all.
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	cfg.Providers.Set("test-provider", config.ProviderConfig{
		ID: "test-provider",
		Models: []catwalk.Model{
			{ID: "model-id", Name: "Configured Name"},
		},
	})

	newItem := func(msg *message.Message) *AssistantInfoItem {
		item := NewAssistantInfoItem(nil, msg, cfg, time.Unix(0, 0))
		return item.(*AssistantInfoItem)
	}

	t.Run("config entry wins", func(t *testing.T) {
		msg := &message.Message{Provider: "test-provider", Model: "model-id", ModelName: "Persisted Name"}
		require.Equal(t, "Configured Name", newItem(msg).modelDisplayName())
	})

	t.Run("persisted name used when model missing from config", func(t *testing.T) {
		msg := &message.Message{Provider: "test-provider", Model: "removed-model", ModelName: "Persisted Name"}
		require.Equal(t, "Persisted Name", newItem(msg).modelDisplayName())
	})

	t.Run("model id used when no name persisted", func(t *testing.T) {
		msg := &message.Message{Provider: "test-provider", Model: "removed-model"}
		require.Equal(t, "removed-model", newItem(msg).modelDisplayName())
	})

	t.Run("unknown only without any model", func(t *testing.T) {
		msg := &message.Message{Provider: "test-provider"}
		require.Equal(t, "Unknown Model", newItem(msg).modelDisplayName())
	})
}
