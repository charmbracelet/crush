package mcp

import (
	"context"
	"maps"
	"os"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// shellResolverWithPath builds a shell resolver whose env carries PATH
// plus any caller-supplied overrides. Without PATH, $(cat), $(echo),
// etc. can't find their binaries in a test process where the shell env
// is otherwise empty.
func shellResolverWithPath(t *testing.T, overrides map[string]string) config.VariableResolver {
	t.Helper()
	m := map[string]string{"PATH": os.Getenv("PATH")}
	maps.Copy(m, overrides)
	return config.NewShellVariableResolver(env.NewFromMap(m))
}

func TestMCPSession_CancelOnClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	ctx, cancel := context.WithCancel(context.Background())

	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	sess := &ClientSession{clientSession, cancel}

	// Verify the context is not cancelled before close.
	require.NoError(t, ctx.Err())

	err = sess.Close()
	require.NoError(t, err)

	// After Close, the context must be cancelled.
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

// TestCreateTransport_URLResolution pins that m.URL goes through the
// same resolver seam as command, args, env, and headers. Covers both
// the HTTP and SSE branches, success and failure, so a regression in
// ResolvedURL wiring is caught at the transport layer rather than only
// at the config layer.
func TestCreateTransport_URLResolution(t *testing.T) {
	t.Parallel()

	shell := config.NewShellVariableResolver(env.NewFromMap(map[string]string{
		"MCP_HOST": "mcp.example.com",
	}))

	t.Run("http success expands $VAR", func(t *testing.T) {
		t.Parallel()
		m := config.MCPConfig{
			Type: config.MCPHttp,
			URL:  "https://$MCP_HOST/api",
		}
		tr, err := createTransport(t.Context(), m, shell)
		require.NoError(t, err)
		require.NotNil(t, tr)
		sct, ok := tr.(*mcp.StreamableClientTransport)
		require.True(t, ok, "expected StreamableClientTransport, got %T", tr)
		require.Equal(t, "https://mcp.example.com/api", sct.Endpoint)
	})

	t.Run("sse success expands $(cmd)", func(t *testing.T) {
		t.Parallel()
		m := config.MCPConfig{
			Type: config.MCPSSE,
			URL:  "https://$(echo mcp.example.com)/events",
		}
		tr, err := createTransport(t.Context(), m, shell)
		require.NoError(t, err)
		sse, ok := tr.(*mcp.SSEClientTransport)
		require.True(t, ok, "expected SSEClientTransport, got %T", tr)
		require.Equal(t, "https://mcp.example.com/events", sse.Endpoint)
	})

	t.Run("http failing $(cmd) surfaces error, no transport created", func(t *testing.T) {
		t.Parallel()
		// Under lenient nounset, unset $VAR expands to "" silently,
		// so the only way a URL resolution *errors* is a failing
		// $(cmd). Mirror the SSE subtest so both transports share
		// coverage for the url-resolve-failure path.
		m := config.MCPConfig{
			Type: config.MCPHttp,
			URL:  "https://$(false)/api",
		}
		tr, err := createTransport(t.Context(), m, shellResolverWithPath(t, nil))
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "url:")
		require.Contains(t, err.Error(), "$(false)")
	})

	t.Run("http unset var expands empty", func(t *testing.T) {
		t.Parallel()
		// Pinning test for the new lenient-nounset default: an
		// unset bare $VAR in the URL is *not* an error. It
		// expands to "" and, here, leaves a syntactically weird
		// but non-empty URL that the existing non-empty guard
		// still lets through. Guards against a future regression
		// that flips strict-by-default back on.
		m := config.MCPConfig{
			Type: config.MCPHttp,
			URL:  "https://$MCP_MISSING_HOST/api",
		}
		tr, err := createTransport(t.Context(), m, shell)
		require.NoError(t, err)
		sct, ok := tr.(*mcp.StreamableClientTransport)
		require.True(t, ok)
		require.Equal(t, "https:///api", sct.Endpoint)
	})

	t.Run("sse failing $(cmd) surfaces error, no transport created", func(t *testing.T) {
		t.Parallel()
		m := config.MCPConfig{
			Type: config.MCPSSE,
			URL:  "https://$(false)/events",
		}
		tr, err := createTransport(t.Context(), m, shell)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "url:")
		require.Contains(t, err.Error(), "$(false)")
	})

	t.Run("http empty-after-resolve still fails the non-empty guard", func(t *testing.T) {
		t.Parallel()
		// ${MCP_EMPTY:-} resolves to the empty string (no error),
		// then the existing TrimSpace guard in createTransport must
		// reject it so we never spawn a transport against "".
		m := config.MCPConfig{
			Type: config.MCPHttp,
			URL:  "${MCP_EMPTY:-}",
		}
		tr, err := createTransport(t.Context(), m, shell)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "non-empty 'url'")
	})

	t.Run("identity resolver round-trips template verbatim", func(t *testing.T) {
		t.Parallel()
		// Client mode forwards the template to the server; no local
		// expansion, no error on unset vars.
		tmpl := "https://$MCP_MISSING_HOST/api"
		m := config.MCPConfig{Type: config.MCPHttp, URL: tmpl}
		tr, err := createTransport(t.Context(), m, config.IdentityResolver())
		require.NoError(t, err)
		sct, ok := tr.(*mcp.StreamableClientTransport)
		require.True(t, ok)
		require.Equal(t, tmpl, sct.Endpoint)
	})
}

// TestCreateTransport_StdioResolution pins that command, args, and env
// for stdio MCPs go through the same resolver seam as the other
// transports. Covers both success (expansion produced the expected
// exec.Cmd) and failure (any one field erroring prevents transport
// creation).
func TestCreateTransport_StdioResolution(t *testing.T) {
	t.Parallel()

	t.Run("success expands command, args, and env", func(t *testing.T) {
		t.Parallel()
		r := shellResolverWithPath(t, map[string]string{
			"MY_TOKEN": "hunter2",
		})
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "forgejo-mcp",
			Args:    []string{"--token", "$MY_TOKEN", "--host", "$(echo example.com)"},
			Env: map[string]string{
				"SECRET":    "$(echo shh)",
				"PLAIN":     "literal",
				"REFERENCE": "$MY_TOKEN",
			},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.NoError(t, err)
		require.NotNil(t, tr)

		ct, ok := tr.(*mcp.CommandTransport)
		require.True(t, ok, "expected CommandTransport, got %T", tr)

		// exec.Cmd.Args[0] is the command name; the rest are positional
		// args as passed.
		require.Equal(t, []string{"forgejo-mcp", "--token", "hunter2", "--host", "example.com"}, ct.Command.Args)

		// Env is os.Environ() + resolved entries (sorted). Check the
		// resolved entries are present with their expanded values.
		require.Contains(t, ct.Command.Env, "SECRET=shh")
		require.Contains(t, ct.Command.Env, "PLAIN=literal")
		require.Contains(t, ct.Command.Env, "REFERENCE=hunter2")
	})

	t.Run("env resolution failure surfaces error, no transport created", func(t *testing.T) {
		t.Parallel()
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "forgejo-mcp",
			Env:     map[string]string{"TOKEN": "$(false)"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "env TOKEN")
	})

	t.Run("failing env command is a hard error", func(t *testing.T) {
		t.Parallel()
		// Under lenient nounset a bare $UNSET expands to ""
		// silently — see the pinning subtest below. The remaining
		// failure mode for env resolution is a $(cmd) that exits
		// non-zero, which must still error out and prevent exec so
		// we never hand a broken credential to the child process.
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "forgejo-mcp",
			Env:     map[string]string{"FORGEJO_ACCESS_TOKEN": "$(exit 5)"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "env FORGEJO_ACCESS_TOKEN")
	})

	t.Run("unset env var expands empty", func(t *testing.T) {
		t.Parallel()
		// Pinning test for the lenient-nounset default: a bare
		// $UNSET in an env value expands to "" without error, and
		// the empty entry is kept on the resulting exec.Cmd (env
		// entries, unlike headers, are not dropped — see design
		// decision #18). Guards against a regression that flips
		// strict-by-default back on and silently breaks users
		// with configs like FORGEJO_ACCESS_TOKEN=$FORGEJO_TOKEN.
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "forgejo-mcp",
			Env:     map[string]string{"FORGEJO_ACCESS_TOKEN": "$FORGEJO_TOKEN_UNSET"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.NoError(t, err)
		ct, ok := tr.(*mcp.CommandTransport)
		require.True(t, ok)
		require.Contains(t, ct.Command.Env, "FORGEJO_ACCESS_TOKEN=")
	})

	t.Run("args resolution failure surfaces error, no transport created", func(t *testing.T) {
		t.Parallel()
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "forgejo-mcp",
			Args:    []string{"--token", "$(false)"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "arg 1")
	})

	t.Run("command resolution failure surfaces error, no transport created", func(t *testing.T) {
		t.Parallel()
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "$(false)",
		}
		tr, err := createTransport(t.Context(), m, r)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "invalid mcp command")
	})

	t.Run("identity resolver round-trips templates verbatim", func(t *testing.T) {
		t.Parallel()
		// Client mode: no local expansion, no error on unset vars.
		m := config.MCPConfig{
			Type:    config.MCPStdio,
			Command: "forgejo-mcp",
			Args:    []string{"--token", "$MCP_MISSING"},
			Env:     map[string]string{"TOKEN": "$(vault read -f token)"},
		}
		tr, err := createTransport(t.Context(), m, config.IdentityResolver())
		require.NoError(t, err)
		ct, ok := tr.(*mcp.CommandTransport)
		require.True(t, ok)
		require.Equal(t, []string{"forgejo-mcp", "--token", "$MCP_MISSING"}, ct.Command.Args)
		require.Contains(t, ct.Command.Env, "TOKEN=$(vault read -f token)")
	})
}

// TestCreateTransport_HeadersResolution pins that a single failing
// header aborts HTTP/SSE transport creation and that the successful
// resolver passes every expanded header through to the round tripper.
func TestCreateTransport_HeadersResolution(t *testing.T) {
	t.Parallel()

	t.Run("http headers success expands $(cmd)", func(t *testing.T) {
		t.Parallel()
		r := shellResolverWithPath(t, map[string]string{
			"GITHUB_TOKEN": "gh-secret",
		})
		m := config.MCPConfig{
			Type: config.MCPHttp,
			URL:  "https://mcp.example.com/api",
			Headers: map[string]string{
				"Authorization": "$(echo Bearer $GITHUB_TOKEN)",
				"X-Static":      "kept",
			},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.NoError(t, err)

		sct, ok := tr.(*mcp.StreamableClientTransport)
		require.True(t, ok)
		rt, ok := sct.HTTPClient.Transport.(*headerRoundTripper)
		require.True(t, ok, "expected headerRoundTripper, got %T", sct.HTTPClient.Transport)
		require.Equal(t, map[string]string{
			"Authorization": "Bearer gh-secret",
			"X-Static":      "kept",
		}, rt.headers)
	})

	t.Run("http failing header surfaces error, no transport", func(t *testing.T) {
		t.Parallel()
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPHttp,
			URL:     "https://mcp.example.com/api",
			Headers: map[string]string{"Authorization": "$(false)"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "header Authorization")
	})

	t.Run("sse failing header surfaces error, no transport", func(t *testing.T) {
		t.Parallel()
		// Under lenient nounset a bare $MISSING expands to "",
		// which ResolvedHeaders drops — no error. The failing
		// $(cmd) path is the remaining way this can fail loudly;
		// cover it on the SSE branch to mirror the HTTP subtest.
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPSSE,
			URL:     "https://mcp.example.com/events",
			Headers: map[string]string{"Authorization": "$(false)"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.Error(t, err)
		require.Nil(t, tr)
		require.Contains(t, err.Error(), "header Authorization")
	})

	t.Run("sse unset var header drops silently", func(t *testing.T) {
		t.Parallel()
		// Pinning test for design decision #18 + lenient nounset:
		// a header whose value resolves to "" (here because the
		// bare $VAR is unset) is omitted from the round tripper
		// rather than sent as "X-Header:". Guards against a
		// regression that either re-introduces strict-by-default
		// or stops dropping empty headers.
		r := shellResolverWithPath(t, nil)
		m := config.MCPConfig{
			Type:    config.MCPSSE,
			URL:     "https://mcp.example.com/events",
			Headers: map[string]string{"Authorization": "$MISSING_TOKEN"},
		}
		tr, err := createTransport(t.Context(), m, r)
		require.NoError(t, err)
		sse, ok := tr.(*mcp.SSEClientTransport)
		require.True(t, ok)
		rt, ok := sse.HTTPClient.Transport.(*headerRoundTripper)
		require.True(t, ok)
		require.NotContains(t, rt.headers, "Authorization")
	})
}

// TestCreateSession_ResolutionFailureUpdatesState pins the user-visible
// half of the regression fix: when any of command/args/env/headers/url
// fails to resolve, createSession must publish StateError to the state
// map so crush_info and the TUI's MCP status card can render a real
// error instead of the MCP silently sitting in "starting" or being
// spawned with an empty credential.
//
// These subtests cannot run in parallel: `states` is a package-level
// csync.Map and each assertion reads the entry written by the call
// under test. They do use unique MCP names per subtest to keep them
// independent regardless of ordering.
func TestCreateSession_ResolutionFailureUpdatesState(t *testing.T) {
	r := shellResolverWithPath(t, nil)

	tests := []struct {
		name            string
		mcpName         string
		cfg             config.MCPConfig
		wantErrContains string
	}{
		{
			name:    "stdio env failure",
			mcpName: "test-stdio-env-fail",
			cfg: config.MCPConfig{
				Type:    config.MCPStdio,
				Command: "echo",
				Env:     map[string]string{"FORGEJO_ACCESS_TOKEN": "$(false)"},
			},
			wantErrContains: "env FORGEJO_ACCESS_TOKEN",
		},
		{
			// Args that reference an unset bare $VAR no longer
			// error out under lenient nounset; the only remaining
			// failure mode for arg resolution is a failing $(cmd).
			name:    "stdio args failure",
			mcpName: "test-stdio-args-fail",
			cfg: config.MCPConfig{
				Type:    config.MCPStdio,
				Command: "echo",
				Args:    []string{"--token", "$(false)"},
			},
			wantErrContains: "arg 1",
		},
		{
			// Likewise for URL: bare $UNSET expands to ""
			// silently, so we need a failing $(cmd) to exercise
			// the "url:" wrap from ResolvedURL.
			name:    "http url failure",
			mcpName: "test-http-url-fail",
			cfg: config.MCPConfig{
				Type: config.MCPHttp,
				URL:  "https://$(false)/api",
			},
			wantErrContains: "url:",
		},
		{
			// A URL whose shell expansion yields the empty
			// string (here via ${VAR:-}) is not a ResolvedURL
			// error, but the non-empty guard in createTransport
			// must still reject it so the state card renders an
			// error instead of spawning a transport against "".
			name:    "http empty-resolved url",
			mcpName: "test-http-url-empty",
			cfg: config.MCPConfig{
				Type: config.MCPHttp,
				URL:  "${MCP_URL_EMPTY:-}",
			},
			wantErrContains: "non-empty 'url'",
		},
		{
			name:    "http header failure",
			mcpName: "test-http-header-fail",
			cfg: config.MCPConfig{
				Type:    config.MCPHttp,
				URL:     "https://mcp.example.com/api",
				Headers: map[string]string{"Authorization": "$(false)"},
			},
			wantErrContains: "header Authorization",
		},
		{
			name:    "sse url failure",
			mcpName: "test-sse-url-fail",
			cfg: config.MCPConfig{
				Type: config.MCPSSE,
				URL:  "https://$(false)/events",
			},
			wantErrContains: "url:",
		},
		{
			// Bare $MISSING in a header resolves to "" silently
			// and is then dropped (design decision #18). The
			// "header Authorization" wrap only surfaces on a
			// $(cmd) failure; that is what this subtest now
			// pins for the SSE path.
			name:    "sse header failure",
			mcpName: "test-sse-header-fail",
			cfg: config.MCPConfig{
				Type:    config.MCPSSE,
				URL:     "https://mcp.example.com/events",
				Headers: map[string]string{"Authorization": "$(false)"},
			},
			wantErrContains: "header Authorization",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Guarantee a clean slate on the shared state map so a
			// stale entry from another test can't satisfy the
			// assertion.
			states.Del(tc.mcpName)
			t.Cleanup(func() { states.Del(tc.mcpName) })

			sess, err := createSession(t.Context(), tc.mcpName, tc.cfg, r)
			require.Error(t, err)
			require.Nil(t, sess)
			require.Contains(t, err.Error(), tc.wantErrContains)

			info, ok := GetState(tc.mcpName)
			require.True(t, ok, "state entry must be written for %q", tc.mcpName)
			require.Equal(t, StateError, info.State, "expected StateError, got %s", info.State)
			require.Error(t, info.Error, "state must carry the failure error")
			require.Contains(t, info.Error.Error(), tc.wantErrContains)
			require.Nil(t, info.Client, "no client session on failure")
		})
	}
}
