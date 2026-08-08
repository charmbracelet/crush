package tools

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
)

// These tests exercise the real production block set (blockFuncs()) rather than
// a hand-rebuilt copy, so they stay honest about what the deny list actually
// does.
//
// Block assertions route the command string through the real shell execution
// path (NewShell(...).Exec); a blocked command is denied before the base
// handler, so it never runs and there is no side effect. Allow assertions check
// the resolved argv against the production block set directly (no execution),
// which avoids invoking package managers during the test while still matching
// what blockHandler passes to each BlockFunc for these metacharacter-free
// command lines.

func aliasExecBlocked(t *testing.T, command string) bool {
	t.Helper()
	sh := shell.NewShell(&shell.Options{
		WorkingDir: t.TempDir(),
		BlockFuncs: blockFuncs(),
	})
	_, _, err := sh.Exec(t.Context(), command)
	return err != nil && strings.Contains(err.Error(), "not allowed for security reasons")
}

func aliasArgvBlocked(argv []string) bool {
	for _, f := range blockFuncs() {
		if f(argv) {
			return true
		}
	}
	return false
}

// npm's install subcommand has documented aliases (i, in, ins, ..., add); gem's
// install alias is `i`. The canonical-spelling rules missed all of these, so a
// global install reached the shell unblocked through an alias.
func TestBlockFuncs_InstallSubcommandAliasesBlocked(t *testing.T) {
	blocked := []string{
		// npm install aliases with the global flag (short and long form).
		"npm i -g typescript",
		"npm in -g typescript",
		"npm ins -g typescript",
		"npm inst -g typescript",
		"npm insta -g typescript",
		"npm instal -g typescript",
		"npm isnt -g typescript",
		"npm isnta -g typescript",
		"npm isntal -g typescript",
		"npm isntall -g typescript",
		"npm add -g typescript",
		"npm i --global typescript",
		"npm add --global typescript",
		// Flag in a trailing position is still caught (ArgumentsBlocker scans).
		"npm i typescript -g",
		// gem install alias + unambiguous prefixes (RubyGems prefix resolution);
		// the gem install rule is unconditional.
		"gem i rubygems-update",
		"gem ins rubygems-update",
		"gem inst rubygems-update",
		"gem insta rubygems-update",
		"gem instal rubygems-update",
	}
	for _, cmd := range blocked {
		t.Run(cmd, func(t *testing.T) {
			if !aliasExecBlocked(t, cmd) {
				t.Errorf("expected %q to be blocked by the production block set, but it was allowed", cmd)
			}
		})
	}
}

// Regression: the canonical spellings the aliases mirror must still be blocked.
func TestBlockFuncs_CanonicalInstallStillBlocked(t *testing.T) {
	blocked := []string{
		"npm install -g typescript",
		"npm install --global typescript",
		"gem install rubygems-update",
		"pip install --user requests",
		"pnpm add -g left-pad",
		"yarn global add left-pad",
	}
	for _, cmd := range blocked {
		t.Run(cmd, func(t *testing.T) {
			if !aliasExecBlocked(t, cmd) {
				t.Errorf("expected canonical %q to stay blocked, but it was allowed", cmd)
			}
		})
	}
}

// The fix must not over-block legitimate workflows. The npm aliases keep the
// -g/--global requirement, so local installs and unrelated subcommands stay
// allowed; gem's `i` is install-only so it mirrors the existing gem ban.
func TestBlockFuncs_LegitNotOverBlocked(t *testing.T) {
	allowed := [][]string{
		// Local installs (no global flag) — the common developer case.
		{"npm", "i"},
		{"npm", "i", "typescript"},
		{"npm", "i", "--save-dev", "typescript"},
		{"npm", "in", "lodash"},
		{"npm", "add", "left-pad"},
		{"npm", "install", "typescript"},
		{"pnpm", "i"},
		{"pnpm", "install"},
		// Unrelated npm subcommands that merely start with the alias letters.
		{"npm", "init", "-y"},
		{"npm", "info", "react"},
		{"npm", "run", "build"},
		{"npm", "ci"},
		// gem read-only / non-install subcommands. `gem in` is an ambiguous
		// prefix (install vs info) that RubyGems rejects, so it is deliberately
		// not in the alias set and must stay un-blocked.
		{"gem", "list"},
		{"gem", "info", "rails"},
		{"gem", "in", "rails"},
		{"gem", "which", "json"},
		// yarn add is local (only `yarn global add` is banned).
		{"yarn", "add", "left-pad"},
		// Other interpreters and builds are untouched.
		{"python3", "-m", "venv", ".venv"},
		{"python3", "-m", "pytest", "tests/"},
		{"go", "build", "./..."},
		{"cat", "README.md"},
	}
	for _, argv := range allowed {
		t.Run(strings.Join(argv, "_"), func(t *testing.T) {
			if aliasArgvBlocked(argv) {
				t.Errorf("legitimate command %v was unexpectedly blocked", argv)
			}
		})
	}
}

// Known limitation, asserted so it is not silently "fixed" later: the alias set
// is npm/gem specific (sourced from their own command tables). It does not, and
// is not claimed to, cover interpreter indirection (python -m pip install
// --user), tools absent from the deny list (uv, pipx), or aliases of other
// package managers. Those are documented in the PR as out of scope.
func TestBlockFuncs_KnownAliasLimitations(t *testing.T) {
	notCovered := [][]string{
		{"python3", "-m", "pip", "install", "--user", "requests"}, // module-runner indirection
		{"uv", "pip", "install", "--user", "requests"},            // tool not in deny list
		{"pipx", "install", "some-cli"},                           // tool not in deny list
	}
	for _, argv := range notCovered {
		t.Run(strings.Join(argv, "_"), func(t *testing.T) {
			if aliasArgvBlocked(argv) {
				t.Errorf("documented out-of-scope command %v is now blocked; update the PR's scope claims", argv)
			}
		})
	}
}
