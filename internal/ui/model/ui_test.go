package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestCurrentModelSupportsImages(t *testing.T) {
	t.Parallel()

	t.Run("returns false when config is nil", func(t *testing.T) {
		t.Parallel()

		ui := newTestUIWithConfig(t, nil)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when coder agent is missing", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents:    map[string]config.Agent{},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when model is not found", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns true when current model supports images", func(t *testing.T) {
		t.Parallel()

		providers := csync.NewMap[string, config.ProviderConfig]()
		providers.Set("test-provider", config.ProviderConfig{
			ID: "test-provider",
			Models: []catwalk.Model{
				{ID: "test-model", SupportsImages: true},
			},
		})

		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {
					Provider: "test-provider",
					Model:    "test-model",
				},
			},
			Providers: providers,
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}

		ui := newTestUIWithConfig(t, cfg)
		require.True(t, ui.currentModelSupportsImages())
	})
}

func TestMouseMode(t *testing.T) {
	t.Parallel()

	t.Run("returns no mouse mode when disabled", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, tea.MouseModeNone, mouseMode(false, false))
		require.Equal(t, tea.MouseModeNone, mouseMode(false, true))
	})

	t.Run("returns cell motion when enabled and no inline editor is active", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, tea.MouseModeCellMotion, mouseMode(true, false))
	})

	t.Run("returns all motion when enabled and an inline editor is active", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, tea.MouseModeAllMotion, mouseMode(true, true))
	})
}

func TestImageKeyWarnsWhenModelDoesNotSupportImages(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"add image", tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}},
		{"paste image", tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u := newImageKeyTestUI(t, false)
			requireWarnMsg(t, u.handleKeyPressMsg(tc.key), "The current model does not support image attachments")
		})
	}
}

// newImageKeyTestUI builds a chat UI with the editor focused and a coder agent
// backed by a test model with the given image capability.
func newImageKeyTestUI(t *testing.T, supportsImages bool) *UI {
	t.Helper()

	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("test-provider", config.ProviderConfig{
		ID: "test-provider",
		Models: []catwalk.Model{
			{ID: "test-model", SupportsImages: supportsImages},
		},
	})
	cfg := &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "test-provider", Model: "test-model"},
		},
		Providers: providers,
		Agents: map[string]config.Agent{
			config.AgentCoder: {Model: config.SelectedModelTypeLarge},
		},
	}

	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: cfg}
	u.dialog = dialog.NewOverlay()
	u.keyMap = DefaultKeyMap()
	sty := u.com.Styles.Attachments
	u.attachments = attachments.New(
		attachments.NewRenderer(sty.Normal, sty.Deleting, sty.Image, sty.Text, sty.Skill, sty.Remove),
		attachments.Keymap{},
	)
	return u
}

// requireWarnMsg walks the commands produced by an update and fails unless one
// of them yields a warning [util.InfoMsg] containing want.
func requireWarnMsg(t *testing.T, cmd tea.Cmd, want string) {
	t.Helper()

	var found bool
	var walk func(tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil || found {
			return
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			for _, child := range msg {
				walk(child)
			}
		case util.InfoMsg:
			if msg.Type == util.InfoTypeWarn && strings.Contains(msg.Msg, want) {
				found = true
			}
		}
	}
	walk(cmd)
	require.True(t, found, "expected a warning containing %q", want)
}

func newTestUIWithConfig(t *testing.T, cfg *config.Config) *UI {
	t.Helper()

	return &UI{
		com: &common.Common{
			Workspace: &testWorkspace{cfg: cfg},
		},
	}
}

// testWorkspace is a minimal [workspace.Workspace] stub for unit tests.
type testWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *testWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *testWorkspace) WorkingDir() string {
	return "/tmp/crush-test"
}

func (w *testWorkspace) AgentIsReady() bool {
	return false
}
