package config

import (
	"cmp"
	"context"
	"log/slog"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
)

func (c *Config) importCopilot() (*oauth.Token, bool) {
	if testing.Testing() {
		return nil, false
	}

	if c.HasConfigField("providers.copilot.api_key") || c.HasConfigField("providers.copilot.oauth") {
		return nil, false
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	if !hasDiskToken {
		return nil, false
	}

	slog.Info("Found existing GitHub Copilot token on disk. Authenticating...")
	token, err := copilot.RefreshToken(context.TODO(), diskToken)
	if err != nil {
		slog.Error("Unable to import GitHub Copilot token", "error", err)
		return nil, false
	}

	if err := cmp.Or(
		c.SetConfigField("providers.copilot.api_key", token.AccessToken),
		c.SetConfigField("providers.copilot.oauth", token),
	); err != nil {
		slog.Error("Unable to save GitHub Copilot token to disk", "error", err)
	}

	slog.Info("GitHub Copilot successfully imported")
	return token, true
}
