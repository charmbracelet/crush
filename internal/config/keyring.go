package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/zalando/go-keyring"
)

const (
	// keyringService is the service name used in the OS keychain.
	keyringService = "crush"

	// KeyringMarker is the sentinel value stored in the JSON config file to
	// indicate that the real credential lives in the OS keychain.
	KeyringMarker = "__keyring__"
)

// KeyringStore provides secure credential storage via the OS keychain
// (macOS Keychain, Windows Credential Manager, Linux Secret Service / D-Bus).
// When the keychain is unavailable (headless servers, CI, etc.) it reports
// itself as unavailable so callers can fall back to file-based storage.
type KeyringStore struct {
	available bool
}

// NewKeyringStore creates a KeyringStore and probes the system keychain for
// availability. Set CRUSH_DISABLE_KEYRING=1 to force file-based storage.
func NewKeyringStore() *KeyringStore {
	ks := &KeyringStore{}

	if v := os.Getenv("CRUSH_DISABLE_KEYRING"); v == "1" || strings.EqualFold(v, "true") {
		slog.Debug("OS keychain disabled via CRUSH_DISABLE_KEYRING")
		return ks
	}

	// Probe the keyring with a harmless set/delete cycle.
	const probe = "crush-keyring-probe"
	if err := keyring.Set(keyringService, probe, "probe"); err != nil {
		slog.Warn("OS keychain is not available, credentials will be stored in config file", "error", err)
		return ks
	}
	_ = keyring.Delete(keyringService, probe)
	ks.available = true
	slog.Debug("OS keychain is available")
	return ks
}

// Available reports whether the OS keychain is accessible.
func (ks *KeyringStore) Available() bool {
	return ks != nil && ks.available
}

// --- API key helpers ---

func apiKeyName(providerID string) string {
	return providerID + "/api_key"
}

// SetAPIKey stores an API key in the OS keychain.
func (ks *KeyringStore) SetAPIKey(providerID, apiKey string) error {
	if !ks.Available() {
		return fmt.Errorf("keyring not available")
	}
	return keyring.Set(keyringService, apiKeyName(providerID), apiKey)
}

// GetAPIKey retrieves an API key from the OS keychain.
func (ks *KeyringStore) GetAPIKey(providerID string) (string, error) {
	if !ks.Available() {
		return "", fmt.Errorf("keyring not available")
	}
	return keyring.Get(keyringService, apiKeyName(providerID))
}

// DeleteAPIKey removes an API key from the OS keychain.
func (ks *KeyringStore) DeleteAPIKey(providerID string) error {
	if !ks.Available() {
		return fmt.Errorf("keyring not available")
	}
	return keyring.Delete(keyringService, apiKeyName(providerID))
}

// --- OAuth token helpers ---

func oauthKeyName(providerID string) string {
	return providerID + "/oauth"
}

// SetOAuthToken stores a serialized OAuth token in the OS keychain.
func (ks *KeyringStore) SetOAuthToken(providerID string, token *oauth.Token) error {
	if !ks.Available() {
		return fmt.Errorf("keyring not available")
	}
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal OAuth token: %w", err)
	}
	return keyring.Set(keyringService, oauthKeyName(providerID), string(data))
}

// GetOAuthToken retrieves an OAuth token from the OS keychain.
func (ks *KeyringStore) GetOAuthToken(providerID string) (*oauth.Token, error) {
	if !ks.Available() {
		return nil, fmt.Errorf("keyring not available")
	}
	data, err := keyring.Get(keyringService, oauthKeyName(providerID))
	if err != nil {
		return nil, err
	}
	var token oauth.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OAuth token from keyring: %w", err)
	}
	return &token, nil
}

// DeleteOAuthToken removes an OAuth token from the OS keychain.
func (ks *KeyringStore) DeleteOAuthToken(providerID string) error {
	if !ks.Available() {
		return fmt.Errorf("keyring not available")
	}
	return keyring.Delete(keyringService, oauthKeyName(providerID))
}

// --- Helpers ---

// IsKeyringMarker reports whether a config value is the keyring placeholder.
func IsKeyringMarker(value string) bool {
	return value == KeyringMarker
}

// isEnvVarReference reports whether a value looks like an environment variable
// reference (e.g. $OPENAI_API_KEY or ${OPENAI_API_KEY}).
func isEnvVarReference(value string) bool {
	return strings.HasPrefix(value, "$")
}

// isCommandSubstitution reports whether a value looks like a command
// substitution (e.g. $(some-command)).
func isCommandSubstitution(value string) bool {
	return strings.HasPrefix(value, "$(") && strings.HasSuffix(value, ")")
}

// isLiteralSecret reports whether a value is a plaintext literal secret
// (not a marker, not an env-var reference, not a command substitution, and not empty).
func isLiteralSecret(value string) bool {
	return value != "" &&
		!IsKeyringMarker(value) &&
		!isEnvVarReference(value) &&
		!isCommandSubstitution(value)
}
