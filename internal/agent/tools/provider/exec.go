package provider

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/herdr"
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
	cmd.Env = childEnv(os.Environ())
	// CLI con hay bật tiến trình cháu (dev server, test watcher) giữ pipe
	// stdout: huỷ/hết giờ thì dừng cả nhóm, và sau waitDelay đóng pipe để
	// Run trả về thay vì treo quá hạn.
	setProcessGroup(cmd)
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("running %s: %w: %s", name, err, tailStderr(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// waitDelay là thời gian chờ CLI con thoát sau khi bị huỷ, trước khi Go đóng
// pipe và giết hẳn tiến trình.
const waitDelay = 5 * time.Second

// sessionLinkEnv là các biến Claude Code đặt cho tiến trình con của MỘT phiên
// của nó (đánh dấu lồng phiên, socket/token nhắn tin, ID phiên). agent-cli mở
// từ trong một phiên Claude Code sẽ thừa kế chúng; CLI con được giao việc phải
// chạy như mở ở terminal thường, không gắn vào phiên kia. Biến cấu hình của
// người dùng (khoá, provider, model...) được giữ nguyên.
var sessionLinkEnv = []string{
	"CLAUDECODE",
	"CLAUDE_CODE_ENTRYPOINT",
	"CLAUDE_CODE_SESSION_ID",
	"CLAUDE_CODE_CHILD_SESSION",
	"CLAUDE_CODE_MESSAGING_SOCKET",
	"CLAUDE_CODE_MESSAGING_TOKEN",
	"CLAUDE_CODE_SESSION_ATTENDED",
	"CLAUDE_CODE_EXECPATH",
	"CLAUDE_CODE_SSE_PORT",
	"CLAUDE_PID",
	"CLAUDE_AGENT_SDK_VERSION",
}

// childEnv trả env cho CLI con: env của agent-cli bỏ đi sessionLinkEnv và các
// biến HERDR_* (CLI con tự báo trạng thái bằng hook của nó sẽ đè trạng thái
// của agent-cli trong cùng pane herdr).
func childEnv(env []string) []string {
	return slices.DeleteFunc(herdr.StripEnv(env), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(sessionLinkEnv, name)
	})
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
