package honcho

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// capBash is a shorthand for summarizing a shell command, which is by
// far the most exercised path in this file.
func capBash(cmd string) string {
	return SummarizeTool("bash", map[string]any{"command": cmd})
}

func TestRedactCommandRedactsCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		// exe must survive redaction so the memory still records
		// which program ran.
		exe string
		// absent are substrings that must not appear in the output.
		absent []string
	}{
		{
			name:   "authorization bearer header",
			cmd:    `curl -H "Authorization: Bearer sk-abc123" https://x.com`,
			exe:    "curl",
			absent: []string{"sk-abc123", "Bearer", "Authorization"},
		},
		{
			name:   "mysql short password flag",
			cmd:    "mysql -u root -p hunter2",
			exe:    "mysql",
			absent: []string{"hunter2", "root"},
		},
		{
			name:   "postgres url with inline credentials",
			cmd:    "psql postgres://user:pass@host/db",
			exe:    "psql",
			absent: []string{"pass", "user:pass", "host"},
		},
		{
			name:   "aws access key flag",
			cmd:    "aws --access-key AKIA123 s3 ls",
			exe:    "aws",
			absent: []string{"AKIA123", "--access-key"},
		},
		{
			name:   "exported api key",
			cmd:    "export API_KEY=sk-live-123",
			exe:    "export",
			absent: []string{"sk-live-123", "API_KEY"},
		},
		{
			name:   "gh auth token flag",
			cmd:    "gh auth login --with-token",
			exe:    "gh",
			absent: []string{"--with-token"},
		},
		{
			name:   "git clone url with token",
			cmd:    "git clone https://user:token@github.com/o/r",
			exe:    "git",
			absent: []string{"token", "github.com"},
		},
		{
			name:   "docker login password",
			cmd:    "docker login -u me -p secret",
			exe:    "docker",
			absent: []string{"secret", "login", "-u"},
		},
		{
			name:   "curl user colon password",
			cmd:    "curl --user admin:admin http://x",
			exe:    "curl",
			absent: []string{"admin"},
		},
		{
			name:   "uppercase long flag",
			cmd:    "deploy --API-KEY=XYZ123 --region us-east-1",
			exe:    "deploy",
			absent: []string{"XYZ123", "--API-KEY"},
		},
		{
			name:   "uppercase authorization header",
			cmd:    `curl -H "AUTHORIZATION: Bearer TOPSECRET" https://x`,
			exe:    "curl",
			absent: []string{"TOPSECRET", "AUTHORIZATION"},
		},
		{
			name:   "mixed case api key header",
			cmd:    `curl -H "X-Api-Key: abc123" https://x`,
			exe:    "curl",
			absent: []string{"abc123", "X-Api-Key"},
		},
		{
			name:   "password token anywhere in argument",
			cmd:    "myapp --db-password=swordfish run",
			exe:    "myapp",
			absent: []string{"swordfish"},
		},
		{
			name:   "cookie header",
			cmd:    `curl -b "Cookie: session=deadbeef" https://x`,
			exe:    "curl",
			absent: []string{"deadbeef"},
		},
		{
			name:   "private key path",
			cmd:    "ssh -i /home/me/.ssh/private_key host",
			exe:    "ssh",
			absent: []string{"private_key", "/home/me"},
		},
		{
			name:   "absolute executable path keeps only basename",
			cmd:    "/usr/local/bin/vault login --token=s.abc",
			exe:    "vault",
			absent: []string{"s.abc", "/usr/local/bin"},
		},
		{
			name:   "short user flag makes next argument secret",
			cmd:    "redis-cli -u redis://h -a hunter2",
			exe:    "redis-cli",
			absent: []string{"hunter2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RedactCommand(tt.cmd)
			require.Contains(t, got, tt.exe, "executable must survive redaction")
			require.Contains(t, got, redactedArgs, "arguments must be dropped")
			for _, secret := range tt.absent {
				require.NotContains(t, got, secret, "secret leaked into summary")
			}
			// The redacted form is exactly the executable plus the
			// marker, nothing more.
			require.Equal(t, strings.TrimSpace(tt.exe)+redactedArgs, got)
		})
	}
}

func TestRedactCommandLeavesOrdinaryCommandsAlone(t *testing.T) {
	t.Parallel()

	tests := []string{
		"go test ./...",
		"git status",
		"npm run build",
		`rg -n "foo" internal/`,
		"ls -la",
		"go build ./internal/honcho",
		"make lint",
		"gofumpt -w .",
		"git log --oneline -n 10",
		`sed -i '' 's/a/b/' file.go`,
		"curl https://example.com/health",
		"docker compose up -d",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, cmd, RedactCommand(cmd))
		})
	}
}

func TestRedactCommandEdgeCases(t *testing.T) {
	t.Parallel()

	require.Empty(t, RedactCommand(""))
	require.Empty(t, RedactCommand("   \t\n  "))
	require.Equal(t, "go test", RedactCommand("   go test   "))
}

// TestRedactCommandCatchesKeywordlessSecrets covers credentials that
// no keyword announces: one carried by a header flag, and one
// recognizable only by its issuer prefix.
func TestRedactCommandCatchesKeywordlessSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cmd    string
		secret string
	}{
		{
			name:   "header flag carries the secret",
			cmd:    `curl --header "X-Key: hunter2" https://x`,
			secret: "hunter2",
		},
		{
			name:   "issuer prefix with no keyword",
			cmd:    "deploy ghp_AbCdEf0123456789",
			secret: "ghp_AbCdEf0123456789",
		},
		{
			name:   "short header flag",
			cmd:    `curl -H "X-Secret-Value: swordfish" https://x`,
			secret: "swordfish",
		},
		{
			name:   "long random token with no prefix",
			cmd:    "deploy Xk9mQ2rTvB7nL4wP1zYc8sHd3JfG6aEu",
			secret: "Xk9mQ2rTvB7nL4wP1zYc8sHd3JfG6aEu",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RedactCommand(tc.cmd)
			require.NotContains(t, got, tc.secret)
			require.Contains(t, got, redactedArgs)
		})
	}
}

func TestSplitCommandRespectsQuotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{
			name: "separator inside double quotes",
			cmd:  `echo "a; b" && go test`,
			want: []string{`echo "a; b" `, " go test"},
		},
		{
			name: "separator inside single quotes",
			cmd:  `echo 'x && y' ; npm run build`,
			want: []string{`echo 'x && y' `, " npm run build"},
		},
		{
			name: "pipe inside double quotes",
			cmd:  `grep "a|b" file | wc -l`,
			want: []string{`grep "a|b" file `, " wc -l"},
		},
		{
			name: "escaped separator",
			cmd:  `echo a\; b && go vet`,
			want: []string{`echo a\; b `, " go vet"},
		},
		{
			name: "double pipe is one separator",
			cmd:  "a || b",
			want: []string{"a ", " b"},
		},
		{
			name: "single pipe splits",
			cmd:  "a | b",
			want: []string{"a ", " b"},
		},
		{
			name: "newline splits",
			cmd:  "go build\ngo test",
			want: []string{"go build", "go test"},
		},
		{
			name: "single ampersand does not split",
			cmd:  "sleep 1 & wait",
			want: []string{"sleep 1 & wait"},
		},
		{
			name: "no separator",
			cmd:  "go test ./...",
			want: []string{"go test ./..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, splitCommand(tt.cmd))
		})
	}
}

func TestSummarizeToolShellQuoteAwareness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cmd     string
		want    string
		notWant []string
	}{
		{
			name:    "quoted semicolon does not split",
			cmd:     `echo "a; b" && go test`,
			want:    "Ran: go test",
			notWant: []string{"echo"},
		},
		{
			name:    "quoted ampersands do not split",
			cmd:     `echo 'x && y' ; npm run build`,
			want:    "Ran: npm run build",
			notWant: []string{"echo"},
		},
		{
			name: "quoted pipe is not a separator",
			cmd:  `grep "a|b" file | wc -l`,
			want: `Ran: grep "a|b" file`,
		},
		{
			name:    "escaped separator stays in its segment",
			cmd:     `echo a\; b && go vet`,
			want:    "Ran: go vet",
			notWant: []string{"echo"},
		},
		{
			name:    "double pipe yields one separator",
			cmd:     "go build || go vet",
			want:    "Ran: go vet",
			notWant: []string{"|"},
		},
		{
			name: "last meaningful segment wins",
			cmd:  "cd /tmp && ls -la && go build ./...",
			want: "Ran: go build ./...",
		},
		{
			name: "falls back when every later segment is trivial",
			cmd:  "go test ./... | tail -n 20",
			want: "Ran: go test ./...",
		},
		{
			name: "redaction still applies inside a compound command",
			cmd:  "cd /tmp && curl -H \"Authorization: Bearer sk-1\" https://x",
			want: "Ran: curl" + redactedArgs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := capBash(tt.cmd)
			require.Equal(t, tt.want, got)
			for _, s := range tt.notWant {
				require.NotContains(t, got, s)
			}
		})
	}
}

func TestSummarizeToolSkipsTrivialCommands(t *testing.T) {
	t.Parallel()

	trivial := []string{
		"ls -la",
		"ls",
		"pwd",
		"cd /tmp",
		"echo hi",
		"cat file.go",
		"head -n 5 file.go",
		"tail -f log",
		"wc -l file",
		"which go",
		"whoami",
		"date",
		"true",
		"clear",
		"sleep 1",
		"/bin/ls -la",
		"cd /tmp && pwd",
		"FOO=bar echo hi",
		"   ",
		"",
	}

	for _, cmd := range trivial {
		t.Run("trivial/"+cmd, func(t *testing.T) {
			t.Parallel()

			require.Empty(t, capBash(cmd))
		})
	}

	meaningful := map[string]string{
		"cd /tmp && go build":  "Ran: go build",
		"echo hi && go test":   "Ran: go test",
		"GOFLAGS=-v go vet":    "Ran: GOFLAGS=-v go vet",
		"./scripts/deploy.sh":  "Ran: ./scripts/deploy.sh",
		"pwd; ls; make server": "Ran: make server",
	}
	for cmd, want := range meaningful {
		t.Run("meaningful/"+cmd, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, want, capBash(cmd))
		})
	}
}

func TestSummarizeToolSilentTools(t *testing.T) {
	t.Parallel()

	silent := []string{
		"view", "read", "glob", "grep", "ls", "diagnostics",
		"lsp_definition", "lsp_references", "lsp_symbols",
		"lsp_call_hierarchy", "crush_info", "crush_logs", "job_output",
		"list_mcp_resources", "read_mcp_resource", "sourcegraph", "fetch",
		"honcho_search", "honcho_remember", "honcho_", "HONCHO_STATUS",
		"View", "GREP", "  Ls  ", "", "   ",
	}

	for _, tool := range silent {
		t.Run("tool/"+tool, func(t *testing.T) {
			t.Parallel()

			require.Empty(t, SummarizeTool(tool, map[string]any{
				"file_path": "/a/b/c/d.go",
				"command":   "go test ./...",
				"symbol":    "Foo",
			}))
		})
	}
}

func TestSummarizeToolShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tool   string
		params map[string]any
		want   string
	}{
		{
			name:   "edit with path",
			tool:   "edit",
			params: map[string]any{"file_path": "/a/b/internal/honcho/capture.go"},
			want:   "Edited .../internal/honcho/capture.go",
		},
		{
			name:   "multiedit with path",
			tool:   "multiedit",
			params: map[string]any{"file_path": "main.go"},
			want:   "Edited main.go",
		},
		{
			name:   "edit without path",
			tool:   "edit",
			params: map[string]any{},
			want:   "Edited a file",
		},
		{
			name:   "write with path alias",
			tool:   "write",
			params: map[string]any{"path": "docs/readme.md"},
			want:   "Wrote docs/readme.md",
		},
		{
			name:   "write without path",
			tool:   "write",
			params: map[string]any{},
			want:   "Wrote a file",
		},
		{
			name:   "download with path",
			tool:   "download",
			params: map[string]any{"file_path": "/tmp/out.bin"},
			want:   "Downloaded to /tmp/out.bin",
		},
		{
			name:   "download without path",
			tool:   "download",
			params: map[string]any{},
			want:   "Downloaded a file",
		},
		{
			name:   "rename with both names",
			tool:   "lsp_rename",
			params: map[string]any{"symbol": "Foo", "new_name": "Bar"},
			want:   "Renamed Foo to Bar",
		},
		{
			name:   "rename missing target",
			tool:   "lsp_rename",
			params: map[string]any{"symbol": "Foo"},
			want:   "Renamed a symbol",
		},
		{
			name:   "rename missing source",
			tool:   "lsp_rename",
			params: map[string]any{"new_name": "Bar"},
			want:   "Renamed a symbol",
		},
		{
			name:   "replace symbol",
			tool:   "lsp_replace_symbol",
			params: map[string]any{"symbol": "Service.Close"},
			want:   "Replaced Service.Close",
		},
		{
			name:   "replace symbol without name",
			tool:   "lsp_replace_symbol",
			params: map[string]any{},
			want:   "Replaced a symbol",
		},
		{
			name:   "agent with prompt",
			tool:   "agent",
			params: map[string]any{"prompt": "find the bug\nand fix it"},
			want:   "Delegated: find the bug",
		},
		{
			name:   "task with description",
			tool:   "task",
			params: map[string]any{"description": "  \n audit the redactor \n"},
			want:   "Delegated: audit the redactor",
		},
		{
			name:   "agent without prompt",
			tool:   "agent",
			params: map[string]any{},
			want:   "Delegated a task",
		},
		{
			name:   "unknown tool",
			tool:   "weather",
			params: map[string]any{},
			want:   "Used weather",
		},
		{
			name:   "unknown tool normalizes case",
			tool:   "  Weather  ",
			params: map[string]any{},
			want:   "Used weather",
		},
		{
			name:   "shell alias behaves like bash",
			tool:   "shell",
			params: map[string]any{"cmd": "go test ./..."},
			want:   "Ran: go test ./...",
		},
		{
			name:   "exec alias reads script param",
			tool:   "exec",
			params: map[string]any{"script": "make release"},
			want:   "Ran: make release",
		},
		{
			name:   "run alias with no command",
			tool:   "run",
			params: map[string]any{},
			want:   "",
		},
		{
			name:   "non string param is ignored",
			tool:   "bash",
			params: map[string]any{"command": 123},
			want:   "",
		},
		{
			name:   "blank string param is ignored",
			tool:   "edit",
			params: map[string]any{"file_path": "   "},
			want:   "Edited a file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, SummarizeTool(tt.tool, tt.params))
		})
	}
}

func TestShortenPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"/Users/kierank/code/charm/crush/internal/honcho/capture.go", ".../internal/honcho/capture.go"},
		{"/a/b/c/d", ".../b/c/d"},
		{"a/b/c.go", "a/b/c.go"},
		{"/a/b/c", "/a/b/c"},
		{"main.go", "main.go"},
		{"/main.go", "/main.go"},
		{"", ""},
		{"/one/two/three/four/five", ".../three/four/five"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, shortenPath(tt.in))
		})
	}
}

func TestSummarizeToolClampsAndStaysValidUTF8(t *testing.T) {
	t.Parallel()

	// Multibyte input so a naive byte cut would split a rune.
	long := strings.Repeat("日", 400)

	cases := map[string]string{
		"agent":  SummarizeTool("agent", map[string]any{"prompt": long}),
		"bash":   SummarizeTool("bash", map[string]any{"command": "deploy " + long}),
		"write":  SummarizeTool("write", map[string]any{"file_path": long + ".go"}),
		"rename": SummarizeTool("lsp_rename", map[string]any{"symbol": long, "new_name": long}),
	}

	for name, got := range cases {
		require.NotEmpty(t, got, name)
		require.LessOrEqual(t, len(got), maxSummaryLen, name)
		require.True(t, utf8.ValidString(got), "%s produced invalid UTF-8", name)
	}
}

func TestSummarizeToolHandlesMissingParams(t *testing.T) {
	t.Parallel()

	tools := []string{
		"bash", "shell", "exec", "run", "edit", "multiedit", "write",
		"download", "lsp_rename", "lsp_replace_symbol", "agent", "task",
		"mystery_tool", "view", "honcho_search", "",
	}

	for _, tool := range tools {
		t.Run("empty/"+tool, func(t *testing.T) {
			t.Parallel()

			require.NotPanics(t, func() {
				SummarizeTool(tool, map[string]any{})
			})
			require.NotPanics(t, func() {
				SummarizeTool(tool, nil)
			})
			// Wrong-typed params must be ignored rather than assert.
			require.NotPanics(t, func() {
				SummarizeTool(tool, map[string]any{
					"command":   []string{"go", "test"},
					"file_path": 42,
					"symbol":    nil,
					"new_name":  struct{}{},
					"prompt":    map[string]any{},
				})
			})
		})
	}
}

func TestIsTrivialCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		seg  string
		want bool
	}{
		{"", true},
		{"   ", true},
		{"ls", true},
		{"ls -la", true},
		{"/usr/bin/ls -la", true},
		{"FOO=bar echo hi", true},
		{"FOO=bar go build", false},
		{"go test", false},
		{"make", false},
		{"-flag only", false},
	}

	for _, tt := range tests {
		t.Run(tt.seg, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, isTrivialCommand(tt.seg))
		})
	}
}

func TestHasInlineCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmd  string
		want bool
	}{
		{"psql postgres://user:pass@host/db", true},
		{"git clone https://a:b@github.com/o/r", true},
		{"curl https://example.com/path", false},
		{"curl https://user@example.com/path", false},
		{"go test ./...", false},
		{"ssh host:/path", false},
		{"curl https://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, hasInlineCredentials(tt.cmd))
		})
	}
}

func TestFirstLine(t *testing.T) {
	t.Parallel()

	require.Equal(t, "hello", firstLine("hello\nworld"))
	require.Equal(t, "hello", firstLine("\n\n  hello  \nworld"))
	require.Empty(t, firstLine(""))
	require.Empty(t, firstLine("\n \n"))
}
