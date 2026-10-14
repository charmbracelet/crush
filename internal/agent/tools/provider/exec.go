package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// maxStderrBytes bounds the stderr tail surfaced in run errors so a chatty
// failure cannot flood the model's context.
const maxStderrBytes = 512

// runCommand executes name with args in dir and returns captured stdout.
// Errors carry a bounded, control-scrubbed stderr tail so the caller can
// surface a readable failure without exposing the full stream. Stdout is
// returned even on failure because codex reports structured errors there
// alongside a nonzero exit.
func runCommand(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("%s CLI %q not found in PATH: install it and sign in once", name, name)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("running %s: %w: %s", name, err, tailStderr(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// tailStderr returns the last chunk of stderr for error reporting, with NUL
// and other control bytes replaced so the message stays readable.
func tailStderr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "no stderr output"
	}
	if len(s) > maxStderrBytes {
		s = s[len(s)-maxStderrBytes:]
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\t' || r == '\n' || (r >= 0x20 && r < 0x7f) {
			out = append(out, r)
			continue
		}
		out = append(out, '?')
	}
	return string(out)
}
