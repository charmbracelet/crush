package config

import (
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestMCPConfig_ResolvedURL(t *testing.T) {
	t.Parallel()

	t.Run("empty url short-circuits without calling resolver", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Type: MCPHttp}
		got, err := m.ResolvedURL(stubResolver{err: errors.New("should not be called")})
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("literal url passes through unchanged", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Type: MCPHttp, URL: "https://mcp.example.com/api"}
		got, err := m.ResolvedURL(NewShellVariableResolver(env.NewFromMap(nil)))
		require.NoError(t, err)
		require.Equal(t, "https://mcp.example.com/api", got)
	})

	t.Run("expands $VAR with shell resolver", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Type: MCPHttp, URL: "https://$MCP_HOST/api"}
		r := NewShellVariableResolver(env.NewFromMap(map[string]string{"MCP_HOST": "mcp.example.com"}))
		got, err := m.ResolvedURL(r)
		require.NoError(t, err)
		require.Equal(t, "https://mcp.example.com/api", got)
	})

	t.Run("expands $(cmd) with shell resolver", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Type: MCPSSE, URL: "https://$(echo mcp.example.com)/events"}
		got, err := m.ResolvedURL(NewShellVariableResolver(env.NewFromMap(nil)))
		require.NoError(t, err)
		require.Equal(t, "https://mcp.example.com/events", got)
	})

	t.Run("unset var is an error wrapping the template", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Type: MCPHttp, URL: "https://$MCP_MISSING_HOST/api"}
		_, err := m.ResolvedURL(NewShellVariableResolver(env.NewFromMap(nil)))
		require.Error(t, err)
		require.Contains(t, err.Error(), "url:")
		require.Contains(t, err.Error(), "$MCP_MISSING_HOST")
		require.Contains(t, err.Error(), "unbound")
	})

	t.Run("failing command substitution is an error", func(t *testing.T) {
		t.Parallel()
		m := MCPConfig{Type: MCPHttp, URL: "https://$(false)/api"}
		_, err := m.ResolvedURL(NewShellVariableResolver(env.NewFromMap(nil)))
		require.Error(t, err)
		require.Contains(t, err.Error(), "url:")
		require.Contains(t, err.Error(), "$(false)")
	})

	t.Run("identity resolver round-trips template verbatim", func(t *testing.T) {
		t.Parallel()
		// In client mode expansion happens server-side; the client must
		// forward the template without touching it and without erroring
		// on unset vars.
		tmpl := "https://$MCP_HOST/$(vault read -f url)"
		m := MCPConfig{Type: MCPHttp, URL: tmpl}
		got, err := m.ResolvedURL(IdentityResolver())
		require.NoError(t, err)
		require.Equal(t, tmpl, got)
	})
}

// stubResolver returns ("", err) for every call. Paired with a non-nil
// err the empty-URL test asserts ResolvedURL short-circuits before
// reaching ResolveValue: if it didn't, the test would fail with err.
type stubResolver struct {
	err error
}

func (s stubResolver) ResolveValue(v string) (string, error) {
	return "", s.err
}
