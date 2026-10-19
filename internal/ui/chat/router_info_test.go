package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestAssistantInfoShowsRouter(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	msg := &message.Message{
		ID:         "m",
		Role:       message.Assistant,
		Model:      "claude-opus-5-5",
		Provider:   "anthropic",
		RouterName: "complexity",
		RouteName:  "complex",
	}
	msg.AddFinish(message.FinishReasonToolUse, "", "")

	// Intermediate turns get a footer too, since the model can change.
	require.True(t, ShouldShowAssistantInfo(msg))

	out := ansi.Strip(NewAssistantInfoItem(&sty, msg, cfg, time.Now()).Render(120))
	require.Contains(t, out, "⇄ complexity/complex → ")

	msg.RouterName, msg.RouteName = "", ""
	out = ansi.Strip(NewAssistantInfoItem(&sty, msg, cfg, time.Now()).Render(120))
	require.NotContains(t, out, "⇄")
	require.False(t, ShouldShowAssistantInfo(msg))
}
