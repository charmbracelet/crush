package honcho

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// tokenFileName holds the OAuth credential beside the shared config,
// but in its own file so a token refresh never rewrites settings that
// other harnesses own.
const tokenFileName = "crush-token.json"

// tokenMu serializes read-modify-write on the token file across the
// goroutines that may refresh concurrently.
var tokenMu sync.Mutex

// TokenPath returns where Crush stores its Honcho OAuth token.
func TokenPath() (string, error) {
	cfgPath, err := SharedConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgPath), tokenFileName), nil
}

// LoadToken reads the stored OAuth token. A missing file yields a nil
// token and no error: not being signed in is an ordinary state.
func LoadToken() (*OAuthToken, error) {
	path, err := TokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("honcho: read token: %w", err)
	}
	var tok OAuthToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("honcho: parse token: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, nil
	}
	return &tok, nil
}

// SaveToken persists an OAuth token with owner-only permissions.
func SaveToken(tok *OAuthToken) error {
	if tok == nil {
		return errors.New("honcho: no token to save")
	}
	path, err := TokenPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("honcho: encode token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("honcho: create config directory: %w", err)
	}
	// Write to a temporary file and rename, so a crash mid-write
	// leaves the previous credential intact rather than a truncated
	// file that reads as "signed out". The temp file is created with
	// owner-only permissions from the start, so the secret is never
	// briefly world-readable.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".crush-token-*")
	if err != nil {
		return fmt.Errorf("honcho: create temporary token file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("honcho: secure token file: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("honcho: write token: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("honcho: write token: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("honcho: install token: %w", err)
	}
	return nil
}

// DeleteToken removes the stored token, signing Crush out.
func DeleteToken() error {
	path, err := TokenPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("honcho: remove token: %w", err)
	}
	return nil
}

// AccessToken returns a usable bearer token for a deployment,
// refreshing it when it has expired.
//
// It returns an empty string with no error when Crush is not signed
// in, so callers can fall back to an API key without treating the
// absence of a token as a failure.
func AccessToken(ctx context.Context, baseURL string) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	tok, err := LoadToken()
	if err != nil {
		return "", err
	}
	if tok == nil {
		return "", nil
	}
	if !tok.Expired() {
		return tok.AccessToken, nil
	}

	fresh, err := RefreshToken(ctx, baseURL, tok)
	if err != nil {
		// Only discard the token when the server said the grant is
		// gone. A network failure, a timeout, or a 5xx must leave it
		// alone: signing the user out because their laptop was
		// offline would be a far worse bug than a failed turn.
		if errors.Is(err, ErrGrantRevoked) {
			slog.Warn("Honcho sign-in is no longer valid, sign in again", "error", err)
			if rmErr := DeleteToken(); rmErr != nil {
				slog.Debug("Failed to remove revoked Honcho token", "error", rmErr)
			}
			return "", err
		}
		slog.Debug("Honcho token refresh failed, keeping credential", "error", err)
		return "", err
	}
	if err := SaveToken(fresh); err != nil {
		slog.Warn("Failed to persist refreshed Honcho token", "error", err)
	}
	return fresh.AccessToken, nil
}

// SignedIn reports whether a stored OAuth token exists, without
// touching the network.
func SignedIn() bool {
	tok, err := LoadToken()
	return err == nil && tok != nil
}
