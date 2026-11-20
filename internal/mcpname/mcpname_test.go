package mcpname

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// allowed is the character set providers commonly accept in a tool name.
var allowed = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// requireValid checks the shared wire rules: allowed characters only and at
// most 64 of them (SEP-986), with the server/tool delimiter intact.
func requireValid(t *testing.T, name string) {
	t.Helper()
	require.Regexp(t, allowed, name)
	require.LessOrEqual(t, len(name), MaxLen)
	require.Contains(t, strings.TrimPrefix(name, "mcp__"), "__", "%q lost its delimiter", name)
}

// A name that needs no cleaning is passed through exactly, with the doubled
// underscore between server and tool.
func TestToolLeavesCleanNamesAlone(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ server, tool string }{
		{"ssh", "ssh_connect"},
		{"docker", "list"},
		{"ok-name", "fine_tool"},
		{"a", "b"},
	} {
		require.Equal(t, "mcp__"+tt.server+"__"+tt.tool, Tool(tt.server, tt.tool))
	}
}

// Existing code identifies MCP tools with a single-underscore prefix check;
// the doubled name must still satisfy it.
func TestToolKeepsLegacyPrefixCheck(t *testing.T) {
	t.Parallel()

	require.Regexp(t, `^mcp_`, Tool("ssh", "connect"))
}

// Providers commonly reject a tool name with anything outside
// [a-zA-Z0-9_-], and reject the whole request over one bad name. An MCP
// server is free to call itself "docker.io" or "my server", so the
// characters are scrubbed, and a digest marks the name as changed.
func TestToolSanitizesDisallowedCharacters(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ server, tool, prefix string }{
		{"docker.io", "list", "mcp__docker_io__list__"},
		{"my server", "run cmd", "mcp__my_server__run_cmd__"},
		{"a/b", "c:d", "mcp__a_b__c_d__"},
	} {
		got := Tool(tt.server, tt.tool)
		require.True(t, strings.HasPrefix(got, tt.prefix), "%q should start with %q", got, tt.prefix)
		require.Len(t, got, len(tt.prefix)+hashLen)
		requireValid(t, got)
	}
}

// Cleaning can map different originals onto the same text. The digest over
// the original server and tool keeps them apart.
func TestToolKeepsCleanedCollisionsDistinct(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, Tool("a.b", "list"), Tool("a_b", "list"))
	require.NotEqual(t, Tool("a__b", "tool"), Tool("a...b", "tool"))
	require.NotEqual(t, Tool("a__b", "tool"), Tool("a_b", "tool"))
	require.NotEqual(t, Tool("srv", "a.b"), Tool("srv", "a_b"))
	require.NotEqual(t, Tool("srv", "a__b"), Tool("srv", "a_b"))
	// Moving a character across the boundary must not collide either.
	require.NotEqual(t, Tool("a.", "b"), Tool("a", ".b"))

	// A clean name must never equal a hashed one, even when the clean
	// tool's own name looks like a digest suffix.
	hashed := Tool("docker.io", "list")
	server, tool, ok := strings.Cut(strings.TrimPrefix(hashed, "mcp__"), "__")
	require.True(t, ok)
	require.NotEqual(t, hashed, Tool(server, tool))
	require.NotEqual(t, hashed, Tool("docker_io", strings.ReplaceAll(tool, "__", "_")))
	require.NotEqual(t, hashed, Tool("docker_io", "list_"+hashed[len(hashed)-hashLen:]))
}

// A clean name has two delimiters and a hashed one three, and Parse says
// which it saw.
func TestToolHashedNamesHaveTheirOwnDelimiter(t *testing.T) {
	t.Parallel()

	clean := Tool("docker_io", "list")
	require.Equal(t, 2, strings.Count(clean, "__"))
	p, ok := Parse(clean)
	require.True(t, ok)
	require.False(t, p.Hashed)

	hashed := Tool("docker.io", "list")
	require.Equal(t, 3, strings.Count(hashed, "__"))
	require.Regexp(t, `^mcp__docker_io__list__[0-9a-f]{6}$`, hashed)
	p, ok = Parse(hashed)
	require.True(t, ok)
	require.True(t, p.Hashed)
	require.Equal(t, Parts{Server: "docker_io", Tool: "list", Hashed: true}, p)
}

// Only an unhashed name can be keyed on its server: "docker." cleans to
// "docker" but must not pass for it.
func TestIsServer(t *testing.T) {
	t.Parallel()

	require.True(t, IsServer(Tool("docker", "mcp-add"), "docker"))
	require.True(t, IsServer("mcp_docker_mcp-add", "docker"), "legacy spelling")
	require.False(t, IsServer(Tool("docker.", "mcp-add"), "docker"))
	require.False(t, IsServer(Tool("docker", "mcp add"), "docker"), "a hashed name never qualifies")
	require.False(t, IsServer(Tool("dockerx", "mcp-add"), "docker"))
	require.False(t, IsServer("bash", "docker"))
}

// No segment may contain "__" or touch the delimiter, or the name could not
// be split back apart.
func TestToolSegmentsNeverContainDelimiter(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ server, tool string }{
		{"a__b", "tool"},
		{"srv", "deep__tool"},
		{"_a_", "_b_"},
		{"a_", "run"},
		{"a", "_run"},
		{"...", "..."},
	} {
		name := Tool(tt.server, tt.tool)
		requireValid(t, name)
		p, ok := Parse(name)
		require.True(t, ok, name)
		for _, seg := range []string{p.Server, p.Tool} {
			require.NotContains(t, seg, "__", name)
			require.False(t, strings.HasPrefix(seg, "_") || strings.HasSuffix(seg, "_"), name)
		}
	}
}

// Whatever Tool produces must parse back into its cleaned segments, or the
// UI cannot label the tools the model is calling.
func TestToolRoundTrips(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 90)
	for _, tt := range []struct {
		name, server, tool string
		wantServer         string
		wantToolPrefix     string
		hashed             bool
	}{
		{"clean", "prickly", "javascript_eval", "prickly", "javascript_eval", false},
		{"trailing underscore on server", "a_", "run", "a", "run", true},
		{"leading underscore on tool", "a", "_run", "a", "run", true},
		{"both empty", "", "", placeholder, placeholder, true},
		{"long tool", "server", long, "server", "xxxx", true},
		{"long server", strings.Repeat("s", 80), "tool", strings.Repeat("s", maxServerLen), "tool", true},
		{"unicode", "café", "naïve", "caf", "na_ve", true},
		{"all unicode", "日本", "ツール", placeholder, placeholder, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			name := Tool(tt.server, tt.tool)
			requireValid(t, name)
			p, ok := Parse(name)
			require.True(t, ok, name)
			require.Equal(t, tt.hashed, p.Hashed, name)
			require.Equal(t, tt.wantServer, p.Server)
			if !tt.hashed {
				require.Equal(t, tt.wantToolPrefix, p.Tool)
				return
			}
			require.True(t, strings.HasPrefix(p.Tool, tt.wantToolPrefix), "tool half %q", p.Tool)
			require.False(t, strings.HasSuffix(p.Tool, "_"), "truncation must not leave a trailing _ before the digest")
			require.Regexp(t, `__[0-9a-f]{6}$`, name)
		})
	}
}

// A name too long for the portable limit is shortened, and two long names
// that share a prefix must not collapse onto each other.
func TestToolCaps(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 90)
	a := Tool("server", long+"a")
	b := Tool("server", long+"b")
	requireValid(t, a)
	requireValid(t, b)
	require.NotEqual(t, a, b, "distinct tools must keep distinct names")
	require.True(t, strings.HasPrefix(a, "mcp__server__"))

	huge := Tool(strings.Repeat("long-server", 6), strings.Repeat("tool", 20))
	requireValid(t, huge)

	x := Tool(strings.Repeat("s", 80)+"x", "tool")
	y := Tool(strings.Repeat("s", 80)+"y", "tool")
	require.NotEqual(t, x, y)

	// Truncation landing on an underscore must not leave "__" before the
	// digest.
	under := Tool("server", strings.Repeat("ab_", 30))
	requireValid(t, under)
	p, ok := Parse(under)
	require.True(t, ok)
	require.True(t, p.Hashed)
	require.False(t, strings.HasSuffix(p.Tool, "_"))
}

func TestParse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in string
		want     Parts
		ok       bool
	}{
		{"current spelling", "mcp__prickly__javascript_eval", Parts{Server: "prickly", Tool: "javascript_eval"}, true},
		{"tool half keeps its underscores", "mcp__ssh__ssh_connect", Parts{Server: "ssh", Tool: "ssh_connect"}, true},
		{"hashed", "mcp__docker_io__list__abc123", Parts{Server: "docker_io", Tool: "list", Hashed: true}, true},
		{"legacy spelling still resolves", "mcp_ssh_ssh_connect", Parts{Server: "ssh", Tool: "ssh_connect"}, true},
		{"legacy with disallowed characters", "mcp_docker.io_list", Parts{Server: "docker.io", Tool: "list"}, true},
		{"third segment that is not a digest", "mcp__a__b__c", Parts{}, false},
		{"not an mcp tool", "bash", Parts{}, false},
		{"prefix only", "mcp__", Parts{}, false},
		{"no tool half", "mcp__server", Parts{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Parse(tc.in)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

// History must be wire-valid; valid names, MCP or not, pass through.
func TestWireSafe(t *testing.T) {
	t.Parallel()

	wire := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	for _, name := range []string{"bash", "mcp_ssh_connect", Tool("docker.io", "list")} {
		require.Equal(t, name, WireSafe(name))
	}
	for _, in := range []string{
		"mcp_docker.io_list",
		"mcp__docker.io_list",
		"",
		"mcp_ünïcödé",
		"mcp_" + strings.Repeat("$", 200),
		strings.Repeat("a", 65),
	} {
		got := WireSafe(in)
		require.Regexp(t, wire, got, "WireSafe(%q)", in)
		require.Equal(t, got, WireSafe(got), "WireSafe(%q) is not stable", in)
	}
	require.NotEqual(t, WireSafe("mcp_a.b"), WireSafe("mcp_a:b"))
}

// Only unhashed current names have an old spelling to alias.
func TestLegacyOf(t *testing.T) {
	t.Parallel()

	got, ok := LegacyOf("mcp__github__get_me")
	require.True(t, ok)
	require.Equal(t, "mcp_github_get_me", got)
	for _, name := range []string{Tool("docker.io", "list"), Tool("_a_", "run"), "mcp_github_get_me", "bash"} {
		_, ok := LegacyOf(name)
		require.False(t, ok, name)
	}
}

func TestUpgradeAllowList(t *testing.T) {
	t.Parallel()

	in := []string{"bash", "mcp_github_create_issue", "mcp_docker.io_list:execute", "mcp_a_b_run", "mcp_unknown_x"}
	got := UpgradeAllowList(in, []string{"github", "docker.io", "a", "a_b"})
	require.Equal(t, in, got[:len(in)], "original entries are kept")
	require.ElementsMatch(t, []string{
		"mcp__github__create_issue",
		Tool("docker.io", "list") + ":execute",
		"mcp__a_b__run",
	}, got[len(in):], "longest server prefix wins")
}
