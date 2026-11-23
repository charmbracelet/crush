package log

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimSourcePath(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ in, want string }{
		{"/Users/christian/go/pkg/mod/github.com/charmbracelet/crush@v0.1.0/internal/agent/agent.go", "github.com/charmbracelet/crush@v0.1.0/internal/agent/agent.go"},
		{"/Users/christian/go/pkg/mod/charm.land/fantasy@v2.0.1/provider.go", "charm.land/fantasy@v2.0.1/provider.go"},
		{"/Users/christian/src/crush-log-source/internal/agent/agent.go", "internal/agent/agent.go"},
		{"C:/ci/crush/main.go", "main.go"},
		{"agent.go", "agent.go"},
	} {
		require.Equal(t, tt.want, trimSourcePath(tt.in), "trimming %q", tt.in)
	}
}

// TestSetupSourceHandling checks that the source field is only logged when
// debug logging is on, and that it is trimmed then. Logging is set up once
// per process, so each case runs in a child process.
func TestSetupSourceHandling(t *testing.T) {
	if os.Getenv("CRUSH_LOG_TEST_CHILD") == "1" {
		setupChild(t)
		return
	}

	for name, debug := range map[string]bool{"debug": true, "info": false} {
		t.Run(name, func(t *testing.T) {
			exe, err := os.Executable()
			require.NoError(t, err)
			c := exec.CommandContext(t.Context(), exe, "-test.run=^TestSetupSourceHandling$")
			c.Env = append(os.Environ(),
				"CRUSH_LOG_TEST_CHILD=1",
				"CRUSH_LOG_TEST_DEBUG="+boolString(debug),
			)
			out, err := c.CombinedOutput()
			require.NoError(t, err, "child test failed:\n%s", out)
		})
	}
}

func setupChild(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "crush.log")
	Setup(logFile, os.Getenv("CRUSH_LOG_TEST_DEBUG") == "true")

	// Close the log file before TempDir cleanup removes it: on Windows,
	// open files cannot be deleted.
	t.Cleanup(func() { _ = file.Close() })

	slog.Info("ModelProvider called")

	got, err := os.ReadFile(logFile)
	require.NoError(t, err)
	line := string(got)
	require.Contains(t, line, `"msg":"ModelProvider called"`)
	if os.Getenv("CRUSH_LOG_TEST_DEBUG") == "true" {
		require.Contains(t, line, `"source"`)
		require.Contains(t, line, `"file":"internal/log/log_test.go"`)
		require.NotContains(t, line, "src/crush-log-source", "the source path must be trimmed")
	} else {
		require.NotContains(t, line, `"source"`, "source must only be logged in debug mode")
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
