package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func useRealRg(t *testing.T) {
	t.Helper()
	p, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("no rg")
	}
	prev := getRg
	getRg = func() string { return p }
	t.Cleanup(func() { getRg = prev })
}

// R1: hostile patterns / include values / paths through the real rg argv.
func TestGrepHostileArgsWithRealRg(t *testing.T) {
	useRealRg(t)
	project, outside := outsideFixture(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	script := filepath.Join(project, "x.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\ncat \"$1\"\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, "flags.txt"), []byte("a line with --pre=x and -v and --files inside\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "docs")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "env"), filepath.Join(project, "envlink.txt")))

	type tc struct {
		name, pattern, include, path string
		literal                      bool
	}
	cases := []tc{
		{"pre flag", "--pre=" + script, "", "", false},
		{"pre relative", "--pre=./x.sh", "", "", false},
		{"dash v", "-v", "", "", false},
		{"files flag", "--files", "", "", false},
		{"version flag", "--version", "", "", false},
		{"help", "--help", "", "", false},
		{"include flag-like", "line", "--pre=" + script, "", false},
		{"include dash x", "line", "-x", "", false},
		{"include files", "line", "--files", "", false},
		{"include negation", "line", "!*.go", "", false},
		{"include dotdot", "API_KEY", "../**", "", false},
		{"pattern secret via symlink file", "AGENT_CLI_API_KEY", "", "", false},
		{"path symlink to outside", "API_KEY", "", "docs", false},
		{"path dash", "line", "", "-x", false},
	}
	for _, c := range cases {
		perms := newRecordingPermissions(false)
		resp := runTool(t, NewGrepTool(perms, project, config.ToolGrep{}), GrepToolName, GrepParams{Pattern: c.pattern, Include: c.include, Path: c.path, LiteralText: c.literal})
		_, err := os.Stat(marker)
		leaked := strings.Contains(resp.Content, "secret-value")
		t.Logf("%-32s isErr=%-5v asked=%d marker=%v leaked=%v out=%.70q", c.name, resp.IsError, len(perms.seen()), err == nil, leaked, resp.Content)
		require.Error(t, err, "%s ran a program", c.name)
		require.False(t, leaked, c.name)
	}
}

// R2: glob root-after-prefix gate with real rg.
func TestGlobPrefixBypassAttempts(t *testing.T) {
	useRealRg(t)
	project, outside := outsideFixture(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "docs")))
	require.NoError(t, os.MkdirAll(filepath.Join(project, "sub", "deep"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "sub", "deep", "lnk")))
	rel, _ := filepath.Rel(project, outside)
	rel = filepath.ToSlash(rel)

	type tc struct {
		name, pattern, path string
		wantAsk             bool
	}
	cases := []tc{
		{"dotdot prefix", rel + "/*", "", true},
		{"dot-slash dotdot", "./" + rel + "/*", "", true},
		{"symlink prefix", "docs/*", "", true},
		{"symlink deep prefix", "sub/deep/lnk/*", "", true},
		{"dotdot after literal", "sub/../" + rel + "/*", "", true},
		{"dotdot through symlink", "docs/../*", "", false}, // lexical clean => project root
		{"wildcard first then dotdot", "*/../../*", "", false},
		{"absolute-looking prefix", outside + "/*", "", false},
		{"leading dash", "-x*", "", false},
		{"double slash", "docs//*", "", true},
		{"path outside", "*", outside, true},
		{"path symlink", "*", "docs", true},
		{"file pattern literal symlink", "docs/env", "", true},
	}
	for _, c := range cases {
		perms := newRecordingPermissions(false)
		resp := runTool(t, NewGlobTool(perms, project, config.ToolGlob{}), GlobToolName, GlobParams{Pattern: c.pattern, Path: c.path})
		asked := len(perms.seen()) > 0
		leaked := strings.Contains(resp.Content, "env") && !strings.Contains(resp.Content, "No files") && !resp.IsError
		t.Logf("%-28s asked=%-5v (want %-5v) isErr=%-5v leakedNames=%v out=%.60q", c.name, asked, c.wantAsk, resp.IsError, leaked, resp.Content)
		if !c.wantAsk {
			require.False(t, leaked, "%s leaked outside names without asking", c.name)
		} else {
			require.True(t, asked, c.name)
		}
	}
}
