package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth/antigravity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The subscription's OAuth client secret cannot be shipped, so it is supplied
// either in the provider block or through the environment, and the explicit
// configuration has to win when both exist.
func TestGoogleSubscriptionClientSecretPrecedence(t *testing.T) {
	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set(antigravity.ProviderID, ProviderConfig{
		ID:                antigravity.ProviderID,
		OAuthClientSecret: "from-config",
	})
	cfg := &Config{Providers: providers}

	t.Setenv(antigravity.ClientSecretEnv, "from-environment")
	assert.Equal(t, "from-config", cfg.GoogleSubscriptionClientSecret())

	providers.Set(antigravity.ProviderID, ProviderConfig{ID: antigravity.ProviderID})
	assert.Equal(t, "from-environment", cfg.GoogleSubscriptionClientSecret())

	t.Setenv(antigravity.ClientSecretEnv, "")
	assert.Empty(t, cfg.GoogleSubscriptionClientSecret())

	// A configured account with no login yet, and no config at all: neither
	// may panic, both mean "ask the user for it".
	empty := &Config{Providers: csync.NewMap[string, ProviderConfig]()}
	assert.Empty(t, empty.GoogleSubscriptionClientSecret())

	require.NotPanics(t, func() {
		(*Config)(nil).GoogleSubscriptionClientSecret()
	})
}
