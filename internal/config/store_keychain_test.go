package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/keyring"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	zkeyring "github.com/zalando/go-keyring"
)

func newKeychainTestStore(t *testing.T) *ConfigStore {
	t.Helper()

	return &ConfigStore{
		config: &Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"testprovider": {ID: "testprovider", Name: "Test Provider"},
			}),
		},
		globalDataPath: filepath.Join(t.TempDir(), "global.json"),
	}
}

func configOnDisk(t *testing.T, store *ConfigStore) string {
	t.Helper()

	data, err := os.ReadFile(store.globalDataPath)
	require.NoError(t, err)
	return string(data)
}

func TestSetProviderAPIKeyStoresSecretInKeyring(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()

	store := newKeychainTestStore(t)

	require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "testprovider", "sk-secret"))

	disk := configOnDisk(t, store)
	require.Contains(t, disk, keyring.Ref("testprovider"))
	require.NotContains(t, disk, "sk-secret")

	stored, err := keyring.Get("testprovider")
	require.NoError(t, err)
	require.Equal(t, "sk-secret", stored)

	provider, ok := store.Config().Providers.Get("testprovider")
	require.True(t, ok)
	require.Equal(t, "sk-secret", provider.APIKey, "in-memory config keeps the resolved secret")
}

func TestSetProviderAPIKeyFallsBackToPlaintextWithoutKeyring(t *testing.T) {
	keyring.MockInitWithError(zkeyring.ErrUnsupportedPlatform)
	keyring.ResetAvailableCache()

	store := newKeychainTestStore(t)

	require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "testprovider", "sk-secret"))

	disk := configOnDisk(t, store)
	require.Contains(t, disk, "sk-secret")
	require.NotContains(t, disk, "keychain://")
}

func TestSetProviderAPIKeyFallsBackToPlaintextOnKeyringFailure(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()

	store := newKeychainTestStore(t)

	require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "testprovider", "sk-secret"))
	require.Contains(t, configOnDisk(t, store), keyring.Ref("testprovider"))

	keyring.MockInitWithError(errors.New("keyring daemon died"))
	require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "testprovider", "sk-rotated"))

	disk := configOnDisk(t, store)
	require.Contains(t, disk, "sk-rotated", "a failed keyring write must not lose the key")
}

func TestSetProviderAPIKeyStoresOAuthAccessTokenInKeyring(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()

	store := newKeychainTestStore(t)
	token := &oauth.Token{AccessToken: "access-123", RefreshToken: "refresh-456"}

	require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "testprovider", token))

	disk := configOnDisk(t, store)
	apiKeyValue := gjson.Get(disk, "providers.testprovider.api_key").String()
	require.Equal(t, keyring.Ref("testprovider"), apiKeyValue)
	require.Contains(t, disk, "refresh-456", "the refresh token stays in the config for OAuth rotation")

	stored, err := keyring.Get("testprovider")
	require.NoError(t, err)
	require.Equal(t, "access-123", stored)
}

func TestRefreshedSecretValueUpdatesExistingEntry(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()

	store := newKeychainTestStore(t)

	require.Equal(t, "access-123", store.refreshedSecretValue("testprovider", "access-123"),
		"providers without a keyring entry keep plaintext storage")

	require.NoError(t, keyring.Set("testprovider", "access-old"))

	require.Equal(t, keyring.Ref("testprovider"), store.refreshedSecretValue("testprovider", "access-123"))

	stored, err := keyring.Get("testprovider")
	require.NoError(t, err)
	require.Equal(t, "access-123", stored)
}
