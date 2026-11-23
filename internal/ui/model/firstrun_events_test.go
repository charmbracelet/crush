package model

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func TestAuthKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pc   config.ProviderConfig
		want string
	}{
		{"oauth", config.ProviderConfig{OAuthToken: &oauth.Token{}}, "oauth"},
		{"env var", config.ProviderConfig{APIKey: "$ANTHROPIC_API_KEY"}, "env var"},
		{"api key", config.ProviderConfig{APIKey: "sk-secret"}, "api key"},
		{"nothing", config.ProviderConfig{}, "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, authKind(tt.pc))
		})
	}
}

func TestOnProviderChosenRemembersThePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		choice dialog.ProviderChoice
		via    string
		method string
	}{
		{dialog.ProviderChoice{ID: "hyper"}, onboardingViaCard, "sign in"},
		{dialog.ProviderChoice{ID: "anthropic", NeedsKey: true}, onboardingViaCardKey, "api key"},
		{dialog.ProviderChoice{ID: dialog.ChoiceMoreProviders}, onboardingViaMoreProviders, "more providers"},
	}
	for _, tt := range tests {
		t.Run(tt.choice.ID, func(t *testing.T) {
			t.Parallel()
			m := &UI{}
			m.onProviderChosen(tt.choice)
			require.Equal(t, tt.via, m.firstRun.via)
			require.Equal(t, tt.method, providerChoiceMethod(tt.choice))
		})
	}
}

func TestFirstTurnIsFollowedOnce(t *testing.T) {
	t.Parallel()

	m := &UI{}
	m.firstRun.firstTurn = &firstTurn{completedAt: time.Now(), held: true}

	m.trackFirstPromptSent("s1")
	require.Equal(t, "s1", m.firstRun.firstTurn.sessionID)

	// A later prompt does not replace the first one.
	m.trackFirstPromptSent("s2")
	require.Equal(t, "s1", m.firstRun.firstTurn.sessionID)

	// Runs in other sessions are ignored.
	m.trackFirstTurnFinished(pubsub.Event[notify.RunComplete]{Payload: notify.RunComplete{SessionID: "s2"}})
	require.NotNil(t, m.firstRun.firstTurn)

	m.trackFirstTurnFinished(pubsub.Event[notify.RunComplete]{Payload: notify.RunComplete{SessionID: "s1"}})
	require.Nil(t, m.firstRun.firstTurn, "only the first turn is reported")
}

func TestFirstTurnIsNotFollowedWithoutOnboarding(t *testing.T) {
	t.Parallel()

	m := &UI{}
	m.trackFirstPromptSent("s1")
	m.trackFirstTurnFinished(pubsub.Event[notify.RunComplete]{Payload: notify.RunComplete{SessionID: "s1"}})
	require.Nil(t, m.firstRun.firstTurn)
}

func TestSetupSource(t *testing.T) {
	t.Parallel()

	require.Equal(t, "none", setupSource(nil))

	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	require.Equal(t, "none", setupSource(cfg))

	cfg.Providers.Set("openai", config.ProviderConfig{ID: "openai", APIKey: "$OPENAI_API_KEY"})
	require.Equal(t, "env var", setupSource(cfg), "falls back to the enabled provider")

	cfg.Providers.Set("hyper", config.ProviderConfig{ID: "hyper", OAuthToken: &oauth.Token{}})
	cfg.Models = map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "hyper", Model: "m"},
	}
	require.Equal(t, "oauth", setupSource(cfg), "follows the large model")
}

func TestErrorCategory(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"POST https://api.anthropic.com/v1/messages: 401 Unauthorized": "auth",
		"invalid x-api-key": "auth",
		"You exceeded your current quota (insufficient_quota) 429": "billing",
		"429 Too Many Requests: rate limit reached":                "rate limit",
		"model_not_found: The model `gpt-9` does not exist":        "model",
		"prompt is too long: 140301 tokens > 140000 maximum":       "context",
		"dial tcp: lookup api.openai.com: no such host":            "network",
		"context deadline exceeded":                                "network",
		"502 Bad Gateway":                                          "server",
		"something unexpected happened":                            "other",
		"request used 4013 tokens":                                 "other",
	}
	for msg, want := range tests {
		t.Run(msg, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, errorCategory(msg))
		})
	}
}
