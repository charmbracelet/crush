package shellconfig

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/version"
)

// loadTimeout bounds a single crushrc execution. Config loading runs on the
// startup and reload critical paths while the config store's write lock is
// held, so a script that blocks (a hung command substitution, a stray loop)
// must not be able to wedge the whole store. The interpreter honors context
// cancellation, so this deadline reliably interrupts a runaway script.
const loadTimeout = 30 * time.Second

// LoadShellConfig executes a crushrc script and returns its config as a
// single JSON object. The script uses config builtins (provider, model, mcp,
// etc.) that mutate a ConfigBuilder in execution order; the builder is then
// marshaled to JSON, which the config loader merges with any other config
// files.
//
// The script runs with the same shell interpreter used by the bash tool and
// hooks, so source, $VAR, $(cmd), and other shell constructs all work.
//
// Execution is bounded by loadTimeout on top of any deadline already present
// on ctx, so a misbehaving script cannot block config loading indefinitely.
//
// Stderr của script được thu lại thay vì bỏ đi: khi script thất bại,
// maxStderrTailBytes byte cuối của nó được nối vào lỗi trả về, ví dụ
//
//	executing shell config .../crushrc: exit status 1: AGENT_CLI_BASE_URL: đặt ...
//
// Không có bước này thì chốt chặn kiểu `: "${VAR:?thông báo}"` chỉ để lại
// "exit status 1", và ở chế độ client/server lý do mất hẳn (stderr của server
// là tệp log không ai xem). Chuyển hướng tới stderr thật của tiến trình
// (`2>/dev/stderr`) cũng được thu, xem redirectDevStderr. Ở đây không in gì ra
// terminal: lỗi mang theo lý do.
func LoadShellConfig(ctx context.Context, path string, src []byte) ([]byte, error) {
	slog.Info("Loading shell config", "path", path)

	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()

	builder := newConfigBuilder()
	runCtx := withConfigBuilder(ctx, builder)

	cwd := filepath.Dir(path)

	// Expose the running Crush version so scripts can feature-detect, e.g.
	// [[ "$CRUSH_VERSION" == "devel" ]] or branch on the release.
	env := append(os.Environ(), "CRUSH_VERSION="+version.Version)

	stderr := newStderrTail(maxStderrTailBytes)
	err := shell.Run(runCtx, shell.RunOptions{
		Command: redirectDevStderr(string(src)),
		Cwd:     cwd,
		Env:     env,
		Stderr:  stderr,
	})
	stderrSummary := stderr.summary()
	if err != nil {
		logAttrs := []any{"path", path, "error", err}
		if stderrSummary != "" {
			logAttrs = append(logAttrs, "stderr", stderrSummary)
		}
		suffix := errorSuffix(err, stderrSummary)
		if shell.IsInterrupt(err) {
			slog.Error("Shell config execution timed out or was cancelled", logAttrs...)
			return nil, fmt.Errorf("shell config %s: execution timed out or was cancelled: %w%s", path, err, suffix)
		}
		slog.Error("Shell config execution failed", logAttrs...)
		return nil, fmt.Errorf("executing shell config %s: %w%s", path, err, suffix)
	}
	// Script thành công nhưng có in ra stderr (cảnh báo của chính script): ghi
	// vào log thay vì terminal, để không làm rối TUI khi nạp lại cấu hình.
	if stderrSummary != "" {
		slog.Warn("Shell config wrote to stderr", "path", path, "stderr", stderrSummary)
	}

	if builder.empty() {
		slog.Warn("Shell config produced no config", "path", path)
		return nil, nil
	}

	data, err := builder.JSON()
	if err != nil {
		slog.Error("Failed to marshal shell config", "path", path, "error", err)
		return nil, fmt.Errorf("marshaling shell config %s: %w", path, err)
	}

	slog.Info("Shell config loaded successfully", "path", path, "bytes", len(data))
	return data, nil
}
