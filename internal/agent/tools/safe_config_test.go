package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// A configured safe command covers exactly what it says and no more. The
// value of the setting is that it is predictable: if it quietly widened to
// accept flags or operands, "go build" would also mean "go build -o
// /usr/local/bin/anything".
func TestUserSafeCommands(t *testing.T) {
	t.Parallel()

	us := userSafeCommands(&config.Permissions{
		SafeCommands: []string{"go build", "cargo check", "  go   vet  ", "", "   "},
	})

	safe := []string{
		"go build",
		"go vet",
		"cargo check",
		// Composing with each other and with built-in safe commands works,
		// because a configured entry joins the same list the rest use.
		"go build && cargo check",
		"go build; ls -la",
		"go build | go vet",
		// A wrapper resolves to the command it runs, as everywhere else.
		"nice go build",
		"timeout 5 go build",
	}
	for _, c := range safe {
		require.True(t, isSafeReadOnly(c, us), "%q should be safe", c)
	}

	unsafe := []string{
		"go build -o /tmp/x",  // a flag is not covered
		"go build ./...",      // nor an operand
		"go install",          // nor a different subcommand
		"go",                  // nor a prefix of the entry
		"go test",             // nor a sibling
		"go build > /tmp/x",   // nor a redirect
		"go build && curl x",  // nor an unsafe neighbour
		"GOFLAGS=-x go build", // nor an environment prefix
	}
	for _, c := range unsafe {
		require.False(t, isSafeReadOnly(c, us), "%q should not be safe", c)
	}

	// Nothing configured means nothing extra is safe.
	require.False(t, isSafeReadOnly("go build", nil))
	require.Empty(t, userSafeCommands(nil))
}

// Configured command names are matched the way the block list matches, so a
// spelling that differs only in case, path or padding still lines up. The
// failure this guards against is silent: the setting looks applied and does
// nothing.
func TestResolveBlockedCommandsNormalizesConfig(t *testing.T) {
	t.Parallel()

	t.Run("allowed_commands removes a default however it is spelled", func(t *testing.T) {
		t.Parallel()
		for _, spelling := range []string{"curl", "CURL", " curl ", "/usr/bin/curl", "curl.exe"} {
			blocked := resolveBlockedCommands(&config.Permissions{
				AllowedCommands: []string{spelling},
			})
			require.NotContains(t, blocked, "curl", "spelled %q", spelling)
			require.Contains(t, blocked, "sudo", "spelled %q: only curl should go", spelling)
		}
	})

	t.Run("blocked_commands adds however it is spelled", func(t *testing.T) {
		t.Parallel()
		for _, spelling := range []string{"kubectl", "KUBECTL", " kubectl ", "/usr/local/bin/kubectl"} {
			bf := blockFuncs(resolveBlockedCommands(&config.Permissions{
				BlockedCommands: []string{spelling},
			}))
			require.Equal(t, "kubectl", shell.CheckCommand("kubectl apply -f x", bf).Reason,
				"spelled %q", spelling)
		}
	})

	t.Run("allow wins over block for the same command", func(t *testing.T) {
		t.Parallel()
		blocked := resolveBlockedCommands(&config.Permissions{
			BlockedCommands: []string{"git"},
			AllowedCommands: []string{"GIT"},
		})
		require.NotContains(t, blocked, "git")
	})

	t.Run("empty entries are ignored", func(t *testing.T) {
		t.Parallel()
		require.Equal(t,
			resolveBlockedCommands(&config.Permissions{}),
			resolveBlockedCommands(&config.Permissions{BlockedCommands: []string{"", "  "}}))
	})
}
