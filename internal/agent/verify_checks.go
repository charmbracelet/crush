package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
)

// sourceFileExts are the file extensions treated as source code — used to
// scope the unverified-in-context suffix (a README edit has nothing to
// hedge about). Check selection itself is not ext-gated: declared verify
// commands gate every mutation.
var sourceFileExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".py": true, ".rs": true, ".c": true,
	".cc": true, ".cpp": true, ".h": true, ".hpp": true, ".java": true,
	".rb": true, ".php": true, ".cs": true, ".swift": true, ".kt": true,
	".kts": true, ".m": true, ".scala": true, ".ex": true, ".exs": true,
	".erl": true, ".hrl": true, ".hs": true, ".clj": true, ".lua": true,
}

// pendingChecksForEdit returns the gate-run checks a mutation to absPath
// selects: every configured verify command (a declared project check
// gates ANY file mutation — a dependency or manifest edit breaks builds
// as readily as source does), plus a same-package test for Go files in a
// tested directory. The decorator records the result as pending entries
// in the verification metadata.
func pendingChecksForEdit(cfg *config.Config, workingDir, absPath string) []message.VerificationCheck {
	if cfg == nil {
		return nil
	}
	var checks []message.VerificationCheck
	for _, v := range cfg.Verify {
		checks = append(checks, message.VerificationCheck{
			Check:   "verify:" + v.DisplayName(),
			State:   message.VerificationPending,
			Command: v.Command,
			Timeout: int(v.TimeoutDuration() / time.Second),
		})
	}
	if strings.EqualFold(filepath.Ext(absPath), ".go") {
		dir := filepath.Dir(absPath)
		rel, err := filepath.Rel(workingDir, dir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && dirHasGoTestFile(dir) {
			// Unquoted by default: observed-bash satisfaction is an exact
			// command match, and a model naturally runs `go test ./pkg` —
			// quoting would dead-end the check's biggest cost saver. Quote
			// only when the relative path actually needs it.
			target := "./" + rel
			if strings.ContainsAny(rel, " \t\"'") {
				target = fmt.Sprintf("%q", target)
			}
			checks = append(checks, message.VerificationCheck{
				Check:   "package-test:" + rel,
				State:   message.VerificationPending,
				Command: "go test " + target,
				Timeout: 120,
			})
		}
	}
	return checks
}

// dirHasGoTestFile reports whether dir contains a *_test.go file.
func dirHasGoTestFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
			return true
		}
	}
	return false
}
