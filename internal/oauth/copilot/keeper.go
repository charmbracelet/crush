package copilot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// minRefreshInterval is the minimum time between token refresh attempts.
const minRefreshInterval = 5 * time.Minute

// TokenKeeper manages Copilot token lifecycle with rate-limited refresh.
type TokenKeeper struct {
	mu           sync.RWMutex
	githubToken  string
	copilotToken *CopilotToken
	lastRefresh  time.Time
}

// NewTokenKeeper creates a new TokenKeeper with the given GitHub OAuth token.
func NewTokenKeeper(githubToken string) *TokenKeeper {
	return &TokenKeeper{
		githubToken: githubToken,
	}
}

// GetToken returns a valid Copilot token, refreshing if necessary.
// Refresh is rate-limited to at most once per 5 minutes.
func (k *TokenKeeper) GetToken(ctx context.Context) (string, error) {
	// Fast path: check if we have a valid cached token.
	k.mu.RLock()
	if k.copilotToken != nil && !k.copilotToken.IsExpired() {
		token := k.copilotToken.Token
		k.mu.RUnlock()
		return token, nil
	}
	k.mu.RUnlock()

	// Slow path: need to refresh.
	k.mu.Lock()
	defer k.mu.Unlock()

	// Double-check after acquiring write lock.
	if k.copilotToken != nil && !k.copilotToken.IsExpired() {
		return k.copilotToken.Token, nil
	}

	// Check rate limit.
	if time.Since(k.lastRefresh) < minRefreshInterval && k.copilotToken != nil {
		slog.Debug("copilot: token refresh rate-limited, using potentially expired token")
		return k.copilotToken.Token, nil
	}

	// Refresh the token.
	slog.Debug("copilot: refreshing token")
	token, err := GetCopilotToken(ctx, k.githubToken)
	if err != nil {
		// If we have an old token, return it with a warning.
		if k.copilotToken != nil {
			slog.Warn("copilot: failed to refresh token, using cached token", "error", err)
			return k.copilotToken.Token, nil
		}
		return "", fmt.Errorf("failed to get copilot token: %w", err)
	}

	k.copilotToken = token
	k.lastRefresh = time.Now()
	slog.Debug("copilot: token refreshed", "expires_at", token.ExpiresAt)

	return token.Token, nil
}

// GetHeaders returns Copilot API headers with a fresh token.
func (k *TokenKeeper) GetHeaders(ctx context.Context) (map[string]string, error) {
	token, err := k.GetToken(ctx)
	if err != nil {
		return nil, err
	}
	return CopilotHeaders(token), nil
}

// HasValidToken checks if the keeper has a non-expired token without refreshing.
func (k *TokenKeeper) HasValidToken() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.copilotToken != nil && !k.copilotToken.IsExpired()
}
