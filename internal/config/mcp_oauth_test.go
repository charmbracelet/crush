package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestSetMCPOAuthConfig_PersistsAndClones(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.MCP["notion"] = MCPConfig{Type: MCPHttp, URL: "https://example.com/mcp"}
	store := testStoreWithPath(cfg, dir)

	oauthCfg := &MCPOAuthConfig{
		Enabled:     true,
		ClientName:  "Crush",
		RedirectURL: "http://127.0.0.1:8080/callback",
		Token: &oauth.Token{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		Registration: &MCPOAuthRegistration{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
		AuthServer: &MCPOAuthAuthServer{
			Issuer:                "https://auth.example.com",
			AuthorizationEndpoint: "https://auth.example.com/authorize",
			TokenEndpoint:         "https://auth.example.com/token",
		},
		Resource: "https://example.com/mcp",
		Scopes:   []string{"read", "write"},
	}

	require.NoError(t, store.SetMCPOAuthConfig(ScopeGlobal, "notion", oauthCfg))

	oauthCfg.Token.AccessToken = "mutated"
	oauthCfg.Registration.ClientID = "mutated"
	oauthCfg.AuthServer.Issuer = "https://mutated.example.com"
	oauthCfg.Scopes[0] = "mutated"

	stored := store.Config().MCP["notion"].OAuth
	require.NotNil(t, stored)
	require.Equal(t, "access-token", stored.Token.AccessToken)
	require.Equal(t, "client-id", stored.Registration.ClientID)
	require.Equal(t, "https://auth.example.com", stored.AuthServer.Issuer)
	require.Equal(t, []string{"read", "write"}, stored.Scopes)

	json := readConfigJSON(t, store.globalDataPath)
	mcpSection := json["mcp"].(map[string]any)
	notion := mcpSection["notion"].(map[string]any)
	oauthJSON := notion["oauth"].(map[string]any)
	require.Equal(t, true, oauthJSON["enabled"])
	require.Equal(t, "Crush", oauthJSON["client_name"])
	tokenJSON := oauthJSON["token"].(map[string]any)
	require.Equal(t, "access-token", tokenJSON["access_token"])
}

func TestSetMCPOAuthToken_InitializesAndClears(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.MCP["notion"] = MCPConfig{Type: MCPHttp, URL: "https://example.com/mcp"}
	store := testStoreWithPath(cfg, dir)

	token := &oauth.Token{AccessToken: "token-a", RefreshToken: "token-b"}
	require.NoError(t, store.SetMCPOAuthToken(ScopeGlobal, "notion", token))
	require.NotNil(t, store.Config().MCP["notion"].OAuth)
	require.Equal(t, "token-a", store.Config().MCP["notion"].OAuth.Token.AccessToken)

	require.NoError(t, store.SetMCPOAuthToken(ScopeGlobal, "notion", nil))
	require.NotNil(t, store.Config().MCP["notion"].OAuth)
	require.Nil(t, store.Config().MCP["notion"].OAuth.Token)

	json := readConfigJSON(t, store.globalDataPath)
	mcpSection := json["mcp"].(map[string]any)
	notion := mcpSection["notion"].(map[string]any)
	oauthJSON := notion["oauth"].(map[string]any)
	_, hasToken := oauthJSON["token"]
	require.False(t, hasToken)
}
