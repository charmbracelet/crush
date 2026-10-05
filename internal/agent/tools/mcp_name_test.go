package tools

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Anthropic bills a request carrying a tool named "mcp_" plus a single
// segment against extra usage instead of a Claude subscription, and rejects it
// with "You're out of extra usage" once that balance is empty. The doubled
// underscore Claude Code uses is billed to the subscription normally. This is
// the pattern that must never match a name we send.
var billedToExtraUsage = regexp.MustCompile(`^mcp_[^_]`)

func TestMCPToolNameIsNotBilledToExtraUsage(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ server, tool string }{
		{"ssh", "ssh_connect"},
		{"docker", "list"},
		{"a", "b"},
	} {
		name := MCPToolName(tt.server, tt.tool)
		require.NotRegexp(t, billedToExtraUsage, name, "%q is billed to extra usage", name)
		require.Equal(t, "mcp__"+tt.server+"__"+tt.tool, name)
	}
}

// The prefix is what the name is built from, so callers stripping it back off
// stay in step with the producer.
func TestMCPToolPrefixMatchesName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "mcp__ssh__", MCPToolPrefix("ssh"))
	require.True(t, len(MCPToolName("ssh", "connect")) > len(MCPToolPrefix("ssh")))
	require.Equal(t, "connect", MCPToolName("ssh", "connect")[len(MCPToolPrefix("ssh")):])
}

// Existing code identifies MCP tools with a single-underscore prefix check;
// the doubled name must still satisfy it.
func TestMCPToolNameKeepsLegacyPrefixCheck(t *testing.T) {
	t.Parallel()

	require.Regexp(t, `^mcp_`, MCPToolName("ssh", "connect"))
}

// Anthropic rejects a tool name containing anything outside [a-zA-Z0-9_-],
// and rejects the whole request over one bad name. An MCP server is free to
// call itself "docker.io" or "my server", so the characters have to be
// scrubbed before they reach the wire.
func TestMCPToolNameSanitizesDisallowedCharacters(t *testing.T) {
	t.Parallel()

	allowed := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	for _, tt := range []struct{ server, tool, want string }{
		{"docker.io", "list", "mcp__docker_io__list"},
		{"my server", "run cmd", "mcp__my_server__run_cmd"},
		{"a/b", "c:d", "mcp__a_b__c_d"},
		{"ok-name", "fine_tool", "mcp__ok-name__fine_tool"},
	} {
		got := MCPToolName(tt.server, tt.tool)
		require.Equal(t, tt.want, got)
		require.Regexp(t, allowed, got, "%q must satisfy Anthropic's tool name pattern", got)
	}
}

// "__" is the delimiter, so a server name that contains one would make the
// boundary unreadable. Tool names keep their underscores: everything past the
// delimiter belongs to the tool regardless.
func TestMCPToolNameCollapsesUnderscoresInServerOnly(t *testing.T) {
	t.Parallel()

	require.Equal(t, "mcp__a_b__tool", MCPToolName("a__b", "tool"))
	require.Equal(t, "mcp__a_b__tool", MCPToolName("a...b", "tool"))
	require.Equal(t, "mcp__srv__deep__tool", MCPToolName("srv", "deep__tool"))
}

// A name too long for the portable limit is shortened, and two long names that
// share a prefix must not collapse onto each other.
func TestMCPToolNameCaps(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 90)
	a := MCPToolName("server", long+"a")
	b := MCPToolName("server", long+"b")

	require.LessOrEqual(t, len(a), maxMCPToolNameLen)
	require.LessOrEqual(t, len(b), maxMCPToolNameLen)
	require.NotEqual(t, a, b, "distinct tools must keep distinct names")
	require.Regexp(t, regexp.MustCompile(`^[a-zA-Z0-9_-]+$`), a)
	require.True(t, strings.HasPrefix(a, "mcp__server__"))

	// Even a server name long enough to fill the budget on its own leaves the
	// delimiter in place, so the name still says where it came from.
	huge := MCPToolName(strings.Repeat("long-server", 6), strings.Repeat("tool", 20))
	require.LessOrEqual(t, len(huge), maxMCPToolNameLen)
	require.Contains(t, strings.TrimPrefix(huge, "mcp__"), "__", "the server/tool boundary must survive truncation")
	require.Regexp(t, regexp.MustCompile(`^[a-zA-Z0-9_-]+$`), huge)

	// Two pathological servers stay distinguishable from each other.
	x := MCPToolName(strings.Repeat("s", 80)+"x", "tool")
	y := MCPToolName(strings.Repeat("s", 80)+"y", "tool")
	require.NotEqual(t, x, y)

	// A name that already fits is left exactly as it is.
	short := MCPToolName("ssh", "ssh_connect")
	require.Equal(t, "mcp__ssh__ssh_connect", short)
}
