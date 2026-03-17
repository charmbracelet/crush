package hooks_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

func helperBinary(t *testing.T, name, src string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found, skipping")
	}

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0o644))

	binName := name
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)
	out, err := exec.CommandContext(t.Context(), "go", "build", "-o", binPath, srcFile).CombinedOutput()
	require.NoError(t, err, "build helper binary: %s", out)
	return binPath
}

func TestCommandHandler_Passthrough_Supported(t *testing.T) {
	t.Parallel()

	src := `package main
import (
	"fmt"
	"os"
)
func main() {
	if len(os.Args) < 2 { os.Exit(1) }
	fmt.Print("wrapped:" + os.Args[1])
}`
	bin := helperBinary(t, "helper", src)

	enabled := true
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "test-passthrough",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{
				Command:     bin,
				Passthrough: true,
			},
		},
	})
	require.NoError(t, err)

	out, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "git status",
	}, "sess-1")
	require.NoError(t, err)
	require.Equal(t, hooks.DecisionModify, out.Decision)
	require.Equal(t, "wrapped:git status", out.ModifiedInput["command"])
}

func TestCommandHandler_Passthrough_Unsupported(t *testing.T) {
	t.Parallel()

	src := `package main
import "os"
func main() { os.Exit(1) }`
	bin := helperBinary(t, "helper-fail", src)

	enabled := true
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "test-fail",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{
				Command:     bin,
				Passthrough: true,
			},
		},
	})
	require.NoError(t, err)

	out, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "htop",
	}, "sess-2")
	require.NoError(t, err)
	require.Equal(t, hooks.DecisionAllow, out.Decision)
}

func TestCommandHandler_JSON_Deny(t *testing.T) {
	t.Parallel()

	src := `package main
import (
	"fmt"
	"os"
)
func main() {
	fmt.Fprint(os.Stdout, "{\"decision\":\"deny\",\"reason\":\"not allowed\"}")
}`
	bin := helperBinary(t, "deny-hook", src)

	enabled := true
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "deny-hook",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{
				Command: bin,
			},
		},
	})
	require.NoError(t, err)

	result, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "rm -rf /",
	}, "sess-3")
	require.NoError(t, err)
	require.Equal(t, hooks.DecisionDeny, result.Decision)
	require.Equal(t, "not allowed", result.Reason)
}

func TestManager_Timeout(t *testing.T) {
	t.Parallel()

	src := `package main
import "time"
func main() { time.Sleep(10 * time.Second) }`
	bin := helperBinary(t, "sleep-hook", src)

	enabled := true
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:      "sleep-hook",
			Enabled:   &enabled,
			Events:    []hooks.Event{hooks.EventPreToolUse},
			Type:      hooks.HandlerTypeCommand,
			TimeoutMs: 200,
			Command: &hooks.CommandConfig{
				Command:     bin,
				Passthrough: true,
			},
		},
	})
	require.NoError(t, err)

	start := time.Now()
	out, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "something",
	}, "sess-4")
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, hooks.DecisionAllow, out.Decision)
	require.Less(t, elapsed, 2*time.Second)
}

func TestManager_ChainedHooks(t *testing.T) {
	t.Parallel()

	makeSrc := func(suffix string) string {
		return `package main
import (
	"encoding/json"
	"fmt"
	"os"
)
func main() {
	var in struct {
		ToolInput map[string]any ` + "`json:\"tool_input\"`" + `
	}
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	cmd, _ := in.ToolInput["command"].(string)
	fmt.Fprintf(os.Stdout, "{\"decision\":\"modify\",\"modified_input\":{\"command\":%q}}", cmd + "` + suffix + `")
}`
	}

	bin1 := helperBinary(t, "hook1", makeSrc("-A"))
	bin2 := helperBinary(t, "hook2", makeSrc("-B"))

	enabled := true
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "hook1",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{Command: bin1},
		},
		{
			Name:    "hook2",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{Command: bin2},
		},
	})
	require.NoError(t, err)

	out, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "git status",
	}, "sess-5")
	require.NoError(t, err)
	require.Equal(t, hooks.DecisionModify, out.Decision)
	require.Equal(t, "git status-A-B", out.ModifiedInput["command"])
}

func TestHTTPHandler_Basic(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in hooks.HookInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&in))
		require.Equal(t, "bash", in.ToolName)

		cmd, _ := in.ToolInput["command"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hooks.HookOutput{
			Decision: hooks.DecisionModify,
			ModifiedInput: map[string]any{
				"command": "rtk " + cmd,
			},
		})
	}))
	defer srv.Close()

	enabled := true
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "http-hook",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeHTTP,
			HTTP: &hooks.HTTPConfig{
				URL: srv.URL + "/hook",
			},
		},
	})
	require.NoError(t, err)

	out, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "git log",
	}, "sess-6")
	require.NoError(t, err)
	require.Equal(t, hooks.DecisionModify, out.Decision)
	require.Equal(t, "rtk git log", out.ModifiedInput["command"])
}

func TestManager_DisabledHook(t *testing.T) {
	t.Parallel()

	disabled := false
	mgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "disabled",
			Enabled: &disabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{
				Command: "this-binary-does-not-exist",
			},
		},
	})
	require.NoError(t, err)

	out, err := mgr.RunPreToolUse(context.Background(), "bash", map[string]any{
		"command": "ls",
	}, "sess-7")
	require.NoError(t, err)
	require.Equal(t, hooks.DecisionAllow, out.Decision)
}
