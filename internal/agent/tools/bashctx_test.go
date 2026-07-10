package tools

import (
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyzeCommand_SingleCommands(t *testing.T) {
	wd := "/repo"
	tests := []struct {
		name       string
		cmd        string
		expect     []string
		expectOpaq bool // true if we expect an opaque command! token
	}{
		{
			name:   "simple command",
			cmd:    "ls",
			expect: []string{"command:ls"},
		},
		{
			name:   "command with subcommand",
			cmd:    "go test",
			expect: []string{"command:go test"},
		},
		{
			name:   "command with subcommand and path arg",
			cmd:    "go test ./...",
			expect: []string{"command:go test", "path:/repo/..."},
		},
		{
			name:   "git subcommand",
			cmd:    "git diff",
			expect: []string{"command:git diff"},
		},
		{
			name:   "cd with relative dir (plain word becomes subcommand)",
			cmd:    "cd build",
			expect: []string{"command:cd build"},
		},
		{
			name:   "cd with absolute dir",
			cmd:    "cd /tmp",
			expect: []string{"command:cd", "path:/tmp"},
		},
		{
			name:   "cd with dot",
			cmd:    "cd .",
			expect: []string{"command:cd", "path:/repo"},
		},
		{
			name:   "command with flag and path",
			cmd:    "head -n 10 /var/log/syslog",
			expect: []string{"command:head", "path:/var/log/syslog"},
		},
		{
			name:   "command with multiple flags and path",
			cmd:    `grep -rn 'foo' src/file.txt`,
			expect: []string{"command:grep", "path:/repo/src/file.txt"},
		},
		{
			name:   "tar command with flags and path",
			cmd:    "tar czf archive.tar.gz ./build",
			expect: []string{"command:tar czf", "path:/repo/build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			if tt.expectOpaq {
				require.Len(t, got, 1)
				require.True(t, len(got[0]) > 9 && got[0][:9] == "command!:",
					"expected opaque token, got %q", got[0])
				return
			}
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestAnalyzeCommand_ChainCommands(t *testing.T) {
	wd := "/repo"

	tests := []struct {
		name   string
		cmd    string
		expect []string
	}{
		{
			name:   "and chain",
			cmd:    "go test && git diff",
			expect: []string{"command:go test", "command:git diff"},
		},
		{
			name:   "or chain",
			cmd:    "ls || echo 'not found'",
			expect: []string{"command:ls", "command:echo"},
		},
		{
			name:   "semicolon chain",
			cmd:    "cd /tmp; pwd; cd -",
			expect: []string{"command:cd", "command:pwd", "path:/tmp"},
		},
		{
			name:   "mixed chains",
			cmd:    "cd /tmp && pwd; ls /var",
			expect: []string{"command:cd", "command:pwd", "command:ls", "path:/tmp", "path:/var"},
		},
		{
			name:   "chain with paths",
			cmd:    "cd /tmp && ls ./src",
			expect: []string{"command:cd", "command:ls", "path:/tmp", "path:/repo/src"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			require.ElementsMatch(t, tt.expect, got, "command: %s", tt.cmd)
		})
	}
}

func TestAnalyzeCommand_Pipeline(t *testing.T) {
	wd := "/repo"

	tests := []struct {
		name   string
		cmd    string
		expect []string
	}{
		{
			name:   "simple pipe",
			cmd:    "ls | grep foo",
			expect: []string{"command:ls", "command:grep foo"},
		},
		{
			name:   "double pipe",
			cmd:    "ls | grep foo | wc -l",
			expect: []string{"command:ls", "command:grep foo", "command:wc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			require.ElementsMatch(t, tt.expect, got, "command: %s", tt.cmd)
		})
	}
}

func TestAnalyzeCommand_FailClosed(t *testing.T) {
	wd := "/repo"

	tests := []struct {
		name string
		cmd  string
	}{
		{
			name: "command substitution",
			cmd:  "echo $(pwd)",
		},
		{
			name: "backtick substitution",
			cmd:  "echo `pwd`",
		},
		{
			name: "redirect",
			cmd:  "ls > file.txt",
		},
		{
			name: "input redirect",
			cmd:  "grep foo < input.txt",
		},
		{
			name: "double redirect",
			cmd:  "cat < input.txt > output.txt",
		},
		{
			name: "stderr redirect",
			cmd:  "ls 2>&1",
		},
		{
			name: "sh -c",
			cmd:  "sh -c 'echo hello'",
		},
		{
			name: "bash -c",
			cmd:  "bash -c 'echo hello'",
		},
		{
			name: "eval",
			cmd:  "eval echo hi",
		},
		{
			name: "function declaration",
			cmd:  "func() { ls; }",
		},
		{
			name: "subshell grouping",
			cmd:  "(ls)",
		},
		{
			name: "braces grouping",
			cmd:  "{ ls; }",
		},
		{
			name: "for loop",
			cmd:  "for i in 1 2 3; do echo $i; done",
		},
		{
			name: "if statement",
			cmd:  "if [ -f foo ]; then echo yes; fi",
		},
		{
			name: "process substitution",
			cmd:  "diff <(ls dir1) <(ls dir2)",
		},
		{
			name: "nested subshell in double-quoted",
			cmd:  "sh -c 'echo $(pwd)'",
		},
		{
			name: "appending redirect",
			cmd:  "echo hello >> logfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			require.Len(t, got, 1, "expected exactly one token for: %s", tt.cmd)
			require.True(t, len(got[0]) > 9 && got[0][:9] == "command!:",
				"expected opaque token, got %q for: %s", got[0], tt.cmd)
		})
	}
}

func TestAnalyzeCommand_PathExtraction(t *testing.T) {
	wd := "/repo/project"

	tests := []struct {
		name   string
		cmd    string
		expect []string
	}{
		{
			name:   "absolute path",
			cmd:    "cat /etc/hosts",
			expect: []string{"command:cat", "path:/etc/hosts"},
		},
		{
			name:   "relative path",
			cmd:    "cat src/main.go",
			expect: []string{"command:cat", "path:/repo/project/src/main.go"},
		},
		{
			name:   "relative dot path",
			cmd:    "cat ./src/main.go",
			expect: []string{"command:cat", "path:/repo/project/src/main.go"},
		},
		{
			name:   "dotdot relative path",
			cmd:    "cat ../other",
			expect: []string{"command:cat", "path:/repo/other"},
		},
		{
			name:   "tilde path",
			cmd:    "cat ~/config",
			expect: []string{"command:cat", "path:" + mustTildePath("~/config")},
		},
		{
			name:   "multiple paths",
			cmd:    "cp src/main.go dist/main.go",
			expect: []string{"command:cp", "path:/repo/project/src/main.go", "path:/repo/project/dist/main.go"},
		},
		{
			name:   "path dedup",
			cmd:    "diff /tmp/x /tmp/x",
			expect: []string{"command:diff", "path:/tmp/x"},
		},
		{
			name:   "command with path arg",
			cmd:    "ls /var/log",
			expect: []string{"command:ls", "path:/var/log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			require.ElementsMatch(t, tt.expect, got, "command: %s", tt.cmd)
		})
	}
}

func mustTildePath(s string) string {
	usr, err := user.Current()
	if err != nil {
		return s
	}
	if len(s) > 1 {
		return filepath.Join(usr.HomeDir, s[1:])
	}
	return usr.HomeDir
}

func TestAnalyzeCommand_Deduplication(t *testing.T) {
	wd := "/repo"

	tests := []struct {
		name   string
		cmd    string
		expect []string
	}{
		{
			name:   "duplicate command in chain",
			cmd:    "ls && ls",
			expect: []string{"command:ls"},
		},
		{
			name:   "duplicate path in same command",
			cmd:    "diff /tmp/x /tmp/x",
			expect: []string{"command:diff", "path:/tmp/x"},
		},
		{
			name:   "same command repeated in complex chain",
			cmd:    "go test ./a && go test ./b",
			expect: []string{"command:go test", "path:/repo/a", "path:/repo/b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			// Verify no duplicates
			seen := make(map[string]int)
			for _, tok := range got {
				seen[tok]++
			}
			for tok, count := range seen {
				require.Equal(t, 1, count, "duplicate token %q in %v", tok, got)
			}
			require.ElementsMatch(t, tt.expect, got)
		})
	}
}

func TestAnalyzeCommand_EdgeCases(t *testing.T) {
	wd := "/repo"

	tests := []struct {
		name   string
		cmd    string
		expect []string
	}{
		{
			name:   "empty command",
			cmd:    "",
			expect: []string{"command:"},
		},
		{
			name:   "only whitespace",
			cmd:    "   ",
			expect: []string{"command:"},
		},
		{
			name:   "quoted argument",
			cmd:    `echo "hello world"`,
			expect: []string{"command:echo"},
		},
		{
			name:   "single quoted arg",
			cmd:    `echo 'hello world'`,
			expect: []string{"command:echo"},
		},
		{
			name:   "double quoted path",
			cmd:    `cat "./src/main.go"`,
			expect: []string{"command:cat", "path:/repo/src/main.go"},
		},
		{
			name:   "cd with - (previous dir, no path token)",
			cmd:    "cd -",
			expect: []string{"command:cd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, wd)
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestAnalyzeCommand_WorkingDirResolution(t *testing.T) {
	tests := []struct {
		name       string
		workingDir string
		cmd        string
		expected   []string
	}{
		{
			name:       "relative path resolved",
			workingDir: "/a/b/c",
			cmd:        "cat ./foo.txt",
			expected:   []string{"command:cat", "path:/a/b/c/foo.txt"},
		},
		{
			name:       "absolute path unchanged",
			workingDir: "/a/b/c",
			cmd:        "cat /etc/passwd",
			expected:   []string{"command:cat", "path:/etc/passwd"},
		},
		{
			name:       "cd with relative target",
			workingDir: "/a/b/c",
			cmd:        "cd dist",
			expected:   []string{"command:cd dist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.cmd, tt.workingDir)
			require.ElementsMatch(t, tt.expected, got)
		})
	}
}

func TestMakeCommandToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"go", "command:go"},
		{"go test", "command:go test"},
		{"git diff", "command:git diff"},
		// Extra whitespace is normalised.
		{"  go  test  ", "command:go test"},
		{"kubectl  get", "command:kubectl get"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, MakeCommandToken(tt.input))
		})
	}
}

func TestMakePathToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"/tmp", "path:/tmp"},
		{"/tmp/subdir", "path:/tmp/subdir"},
		// filepath.Clean is applied.
		{"/tmp//subdir", "path:/tmp/subdir"},
		{"/tmp/subdir/..", "path:/tmp"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, MakePathToken(tt.input))
		})
	}
}
