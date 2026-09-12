package agent

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadStoreWithHooks writes a crush.json containing the given hooks JSON
// into an isolated temp dir and loads a real ConfigStore from it. This
// exercises the same config path production uses.
func loadStoreWithHooks(t *testing.T, hooksJSON string) (*config.ConfigStore, string) {
	t.Helper()
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	cfgJSON := `{"hooks":` + hooksJSON + `}`
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crush.json"), []byte(cfgJSON), 0o600))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	return store, workDir
}

// awaitOutcome reads permission notifications until a final decision
// (granted or denied) arrives, skipping the leading "requested" event.
func awaitOutcome(t *testing.T, ch <-chan pubsub.Event[permission.PermissionNotification]) permission.PermissionNotification {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			require.True(t, ok, "notification channel closed before an outcome arrived")
			if ev.Payload.Granted || ev.Payload.Denied {
				return ev.Payload
			}
		case <-deadline:
			t.Fatal("timed out waiting for a permission outcome notification")
		}
	}
}

// TestPrePermissionBridge_Deny is an end-to-end test of the full bridge:
// permission.Request -> dispatcher -> hooks.Runner -> real shell command
// -> aggregated decision -> back to the permission service. A PrePermission
// hook that denies must short-circuit the prompt and surface its reason in
// the notification.
func TestPrePermissionBridge_Deny(t *testing.T) {
	store, workDir := loadStoreWithHooks(t, `{
		"PrePermission": [
			{"matcher": "^bash$", "command": "echo '{\"decision\":\"deny\",\"reason\":\"blocked by policy\"}'"}
		]
	}`)

	svc := permission.NewPermissionService(workDir, false, nil)
	svc.SetPermissionHooks(newPermissionHookDispatcher(store, nil))

	notifications := svc.SubscribeNotifications(context.Background())
	granted, err := svc.Request(context.Background(), permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "c1",
		ToolName:   "bash",
		Action:     "execute",
		Path:       workDir,
		Params:     map[string]any{"command": "rm -rf /"},
	})
	require.NoError(t, err)
	assert.False(t, granted, "PrePermission deny must block without prompting")

	outcome := awaitOutcome(t, notifications)
	assert.Equal(t, "c1", outcome.ToolCallID)
	assert.True(t, outcome.Denied)
	assert.Equal(t, "blocked by policy", outcome.Reason)
}

// TestPrePermissionBridge_Allow verifies a PrePermission hook that allows
// grants the call and skips the prompt entirely.
func TestPrePermissionBridge_Allow(t *testing.T) {
	store, workDir := loadStoreWithHooks(t, `{
		"PrePermission": [
			{"matcher": "^bash$", "command": "echo '{\"decision\":\"allow\"}'"}
		]
	}`)

	svc := permission.NewPermissionService(workDir, false, nil)
	svc.SetPermissionHooks(newPermissionHookDispatcher(store, nil))

	notifications := svc.SubscribeNotifications(context.Background())
	granted, err := svc.Request(context.Background(), permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "c2",
		ToolName:   "bash",
		Action:     "execute",
		Path:       workDir,
		Params:     map[string]any{"command": "ls"},
	})
	require.NoError(t, err)
	assert.True(t, granted, "PrePermission allow must grant without prompting")

	outcome := awaitOutcome(t, notifications)
	assert.Equal(t, "c2", outcome.ToolCallID)
	assert.True(t, outcome.Granted)
}

// TestPrePermissionBridge_SilenceFallsThroughToPrompt verifies that a hook
// expressing no opinion lets the normal prompt proceed (and a subscriber
// can still grant it).
func TestPrePermissionBridge_SilenceFallsThroughToPrompt(t *testing.T) {
	store, workDir := loadStoreWithHooks(t, `{
		"PrePermission": [
			{"matcher": "^bash$", "command": "exit 0"}
		]
	}`)

	svc := permission.NewPermissionService(workDir, false, nil)
	svc.SetPermissionHooks(newPermissionHookDispatcher(store, nil))

	events := svc.Subscribe(context.Background())
	var (
		wg      sync.WaitGroup
		granted bool
		err     error
	)
	wg.Go(func() {
		granted, err = svc.Request(context.Background(), permission.CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "c3",
			ToolName:   "bash",
			Action:     "execute",
			Path:       workDir,
			Params:     map[string]any{"command": "make deploy"},
		})
	})

	event := <-events
	svc.Grant(event.Payload)
	wg.Wait()
	require.NoError(t, err)
	assert.True(t, granted, "silent hook must fall through to the prompt, which then grants")
}

// TestPrePermissionBridge_NoHooksConfigured verifies the dispatcher is a
// no-op when no PrePermission hooks exist, so the normal prompt runs.
func TestPrePermissionBridge_NoHooksConfigured(t *testing.T) {
	store, workDir := loadStoreWithHooks(t, `{}`)

	svc := permission.NewPermissionService(workDir, false, nil)
	svc.SetPermissionHooks(newPermissionHookDispatcher(store, nil))

	events := svc.Subscribe(context.Background())
	var (
		wg      sync.WaitGroup
		granted bool
		err     error
	)
	wg.Go(func() {
		granted, err = svc.Request(context.Background(), permission.CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "c4",
			ToolName:   "bash",
			Action:     "execute",
			Path:       workDir,
			Params:     map[string]any{"command": "ls"},
		})
	})

	event := <-events
	svc.Grant(event.Payload)
	wg.Wait()
	require.NoError(t, err)
	assert.True(t, granted)
}

// TestPermissionDeniedBridge verifies a PermissionDenied hook fires after
// a user denial, end to end through the dispatcher.
func TestPermissionDeniedBridge(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "denials.log")
	store, workDir := loadStoreWithHooks(t, `{
		"PermissionDenied": [
			{"command": "echo denied >> ` + logFile + `"}
		]
	}`)

	svc := permission.NewPermissionService(workDir, false, nil)
	svc.SetPermissionHooks(newPermissionHookDispatcher(store, nil))

	events := svc.Subscribe(context.Background())
	var (
		wg      sync.WaitGroup
		granted bool
		err     error
	)
	wg.Go(func() {
		granted, err = svc.Request(context.Background(), permission.CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "c5",
			ToolName:   "bash",
			Action:     "execute",
			Path:       workDir,
			Params:     map[string]any{"command": "rm -rf /"},
		})
	})

	event := <-events
	svc.Deny(event.Payload)
	wg.Wait()
	require.NoError(t, err)
	assert.False(t, granted)

	// The dispatch is async; poll for the hook's side effect to land.
	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(logFile)
		return readErr == nil && len(data) > 0
	}, 5*time.Second, 20*time.Millisecond, "PermissionDenied hook should have written the log file")

	data, readErr := os.ReadFile(logFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "denied")
}