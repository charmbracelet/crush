package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/automode"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHooks is a minimal PermissionHooks for composite-forwarding tests.
type stubHooks struct{}

func (stubHooks) PrePermission(context.Context, permission.PermissionRequest) permission.PreHookResult {
	return permission.PreHookResult{}
}

func (stubHooks) PermissionDenied(context.Context, permission.PermissionRequest) {}

// The permission service reaches runtime togglers through a type
// assertion on the installed hooks. If compositeHooks ever loses the
// method, SetAutoMode silently stops reaching the native classifier and
// auto mode's real state diverges from the reported one.
var _ permission.AutoModeToggler = compositeHooks{}

// TestAutoModeToggleReachesNativeClassifier guards against auto mode
// being active while it is not enabled (and inert when it is): the
// native classifier defaults to on at construction, and only the
// composite's SetAutoModeEnabled forwarding can honor the toggle.
func TestAutoModeToggleReachesNativeClassifier(t *testing.T) {
	t.Parallel()

	const dangerousCmd = "chmod 777 file" // matches a static dangerous pattern

	requestDangerous := func(svc permission.Service) (bool, error) {
		return svc.Request(context.Background(), permission.CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "c1",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/work",
			Params:     map[string]any{"command": dangerousCmd},
		})
	}

	newSvc := func(autoEnabled bool) (permission.Service, <-chan pubsub.Event[permission.PermissionRequest]) {
		svc := permission.NewPermissionService("/work", false, nil)
		svc.SetPermissionHooks(compositeHooks{
			primary:   automode.New(automode.Options{}),
			secondary: stubHooks{},
		})
		svc.SetAutoMode(autoEnabled)
		return svc, svc.Subscribe(context.Background())
	}

	t.Run("disabled defers to the human prompt", func(t *testing.T) {
		t.Parallel()
		svc, events := newSvc(false)
		var (
			wg      sync.WaitGroup
			granted bool
			err     error
		)
		wg.Go(func() { granted, err = requestDangerous(svc) })

		select {
		case ev := <-events:
			svc.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("auto mode is disabled, so the request must surface a prompt instead of a classifier decision")
		}
		wg.Wait()
		require.NoError(t, err)
		assert.False(t, granted)
	})

	t.Run("enabled blocks without a prompt", func(t *testing.T) {
		t.Parallel()
		svc, events := newSvc(true)
		granted, err := requestDangerous(svc)
		require.NoError(t, err)
		assert.False(t, granted, "the native classifier must block the dangerous command")
		assert.Contains(t, svc.DenialReason("c1"), "[auto-mode]")
		select {
		case ev := <-events:
			t.Fatalf("no prompt should surface once the classifier denies, got %+v", ev.Payload)
		case <-time.After(200 * time.Millisecond):
		}
	})

	// Turning the toggle back off must take effect immediately: the same
	// service with a fresh subscriber flow defers to the human again.
	t.Run("re-disabled defers again", func(t *testing.T) {
		t.Parallel()
		svc, events := newSvc(true)
		svc.SetAutoMode(false)
		var (
			wg      sync.WaitGroup
			granted bool
			err     error
		)
		wg.Go(func() { granted, err = requestDangerous(svc) })
		select {
		case ev := <-events:
			svc.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("re-disabling auto mode must return control to the human prompt")
		}
		wg.Wait()
		require.NoError(t, err)
		assert.False(t, granted)
	})
}

func TestClassifierModelSelection(t *testing.T) {
	t.Parallel()

	explicit := &config.AutoModeConfig{
		Classifier: &config.AutoModeClassifier{
			Provider: "local-llama",
			Model:    "gemma4-e4b",
		},
	}

	t.Run("explicit classifier wins", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			AutoMode: explicit,
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeSmall: {Provider: "p", Model: "small-model"},
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "local-llama", sel.Provider)
		assert.Equal(t, "gemma4-e4b", sel.Model)
	})

	t.Run("falls back to small model", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeSmall: {Provider: "p", Model: "small-model"},
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "small-model", sel.Model)
	})

	t.Run("falls back to large model when small unset", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "large-model", sel.Model)
	})

	t.Run("nil auto mode section falls through to slots", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeSmall: {Provider: "p", Model: "small-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "small-model", sel.Model)
	})

	t.Run("empty classifier fields fall through to slots", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			AutoMode: &config.AutoModeConfig{
				Classifier: &config.AutoModeClassifier{Provider: "", Model: ""},
			},
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "large-model", sel.Model)
	})

	t.Run("no models at all yields no selection", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		_, ok := classifierModelSelection(cfg)
		assert.False(t, ok, "rules-only mode applies when nothing is configured")
	})
}
