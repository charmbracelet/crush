package agent

import (
	"context"
	"net/http"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// Every provider that signs in by OAuth asks for a new sign-in once its
// credential is rejected for good, not only Hyper.
func TestRejectedOAuthCredentialAsksForSignIn(t *testing.T) {
	const providerID = "copilot"
	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
	broker := pubsub.NewBroker[notify.Notification]()
	t.Cleanup(broker.Shutdown)
	coord.notify = broker
	events := broker.Subscribe(t.Context())

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	agent := newMockAgent(providerID, 128, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		return nil, &fantasy.ProviderError{StatusCode: http.StatusUnauthorized}
	})
	agent.model.OAuth = true

	_, err = coord.runSubAgent(t.Context(), subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "message",
		ToolCallID:     "call",
		Prompt:         "go",
		SessionTitle:   "sub",
	})
	require.NoError(t, err)

	select {
	case event := <-events:
		require.Equal(t, notify.TypeReAuthenticate, event.Payload.Type)
		require.Equal(t, providerID, event.Payload.ProviderID)
	case <-time.After(time.Second):
		t.Fatal("no sign-in was requested")
	}
}

func TestSignInAgainMessageNamesProviderAndCommand(t *testing.T) {
	t.Parallel()

	m := Model{ModelCfg: config.SelectedModel{Provider: "copilot"}, ProviderName: "GitHub Copilot"}
	title, detail := reauthFinish(m)
	require.Equal(t, "Unauthorized", title)
	require.Equal(t, `Please sign in to GitHub Copilot again. You can also run "crush login copilot --force".`, detail)

	m.ProviderName = ""
	_, detail = reauthFinish(m)
	require.Contains(t, detail, "sign in to Copilot again")
}
