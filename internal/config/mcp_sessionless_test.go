package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestMCPConfig_IsSessionless(t *testing.T) {
	t.Parallel()

	resolver := NewShellVariableResolver(env.NewFromMap(map[string]string{
		"GH_MCP_HOST": "api.githubcopilot.com",
	}))
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name string
		cfg  MCPConfig
		want bool
	}{
		{
			name: "explicit true wins over unknown url",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://mcp.example.com/", Sessionless: boolPtr(true)},
			want: true,
		},
		{
			name: "explicit false overrides known sessionless url",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://api.githubcopilot.com/mcp/", Sessionless: boolPtr(false)},
			want: false,
		},
		{
			name: "known githubcopilot url auto-detected",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://api.githubcopilot.com/mcp/"},
			want: true,
		},
		{
			name: "known github url auto-detected",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://api.github.com/mcp/"},
			want: true,
		},
		{
			name: "known url without trailing slash auto-detected",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://api.githubcopilot.com/mcp"},
			want: true,
		},
		{
			name: "known url reached via $VAR expansion",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://$GH_MCP_HOST/mcp/"},
			want: true,
		},
		{
			name: "unknown url defaults to false",
			cfg:  MCPConfig{Type: MCPHttp, URL: "https://mcp.example.com/mcp"},
			want: false,
		},
		{
			name: "empty url defaults to false",
			cfg:  MCPConfig{Type: MCPHttp},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.cfg.IsSessionless(resolver))
		})
	}
}
