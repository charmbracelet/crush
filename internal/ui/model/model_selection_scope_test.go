package model

import (
	"context"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"charm.land/catwalk/pkg/catwalk"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func newModelScopeUI(t *testing.T) (*UI, *countingWorkspace) {
	t.Helper()
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("p", config.ProviderConfig{ID: "p", Models: []catwalk.Model{{ID: "other"}}})
	ws := &countingWorkspace{
		ready: true,
		cfg: &config.Config{
			Providers: providers,
			Models:    map[config.SelectedModelType]config.SelectedModel{},
		},
	}
	m := newBusyUI(ws)
	m.userThemeSelected = true // Skip the live theme swap, not under test.
	return m, ws
}

// A model picked in the TUI belongs to the project, not to the user's
// global config, so switching it in one worktree leaves the others alone.
func TestModelSelectionWritesWorkspaceScope(t *testing.T) {
	m, ws := newModelScopeUI(t)

	// No small model configured: both large and small get written.
	m.handleSelectModel(dialog.ActionSelectModel{
		Model:     config.SelectedModel{Provider: "p", Model: "big"},
		ModelType: config.SelectedModelTypeLarge,
	})
	require.Equal(t, []config.Scope{config.ScopeWorkspace, config.ScopeWorkspace}, ws.prefModelScopes)

	ws.prefModelScopes = nil
	cmd := m.restoreModelFromSession([]message.Message{{
		Role: message.Assistant, Provider: "p", Model: "other",
	}})
	require.NotNil(t, cmd)
	cmd() // Khôi phục ghi cấu hình trong Cmd, không trong Update.
	require.Equal(t, []config.Scope{config.ScopeWorkspace, config.ScopeWorkspace}, ws.prefModelScopes)
}

func (w *countingWorkspace) UpdateAgentModel(context.Context) error { return nil }

// actionDialog answers any message with a fixed action, so a test can push
// one dialog action through handleDialogMsg.
type actionDialog struct{ action dialog.Action }

func (*actionDialog) ID() string                               { return "action-dialog" }
func (d *actionDialog) HandleMsg(tea.Msg) dialog.Action        { return d.action }
func (*actionDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

// Thinking mode and reasoning effort edit the selected model, so they must
// land in the same project file as the model itself.
func TestModelTweaksWriteWorkspaceScope(t *testing.T) {
	for name, action := range map[string]dialog.Action{
		"thinking":         dialog.ActionToggleThinking{},
		"reasoning effort": dialog.ActionSelectReasoningEffort{Effort: "high"},
	} {
		t.Run(name, func(t *testing.T) {
			m, ws := newModelScopeUI(t)
			ws.cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{Provider: "p", Model: "a"}
			ws.cfg.Agents = map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			}
			m.dialog.OpenDialog(&actionDialog{action: action})

			runFirstOfSequence(m.handleDialogMsg(struct{}{}))

			require.Equal(t, []config.Scope{config.ScopeWorkspace}, ws.prefModelScopes)
		})
	}
}

// runFirstOfSequence runs the commands of a Batch/Sequence one level deep,
// which is how the toggle's write (the first step of a tea.Sequence) runs.
func runFirstOfSequence(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	v := reflect.ValueOf(cmd())
	if v.Kind() != reflect.Slice {
		return
	}
	for i := range v.Len() {
		if c, ok := v.Index(i).Interface().(tea.Cmd); ok {
			if inner := c(); inner != nil {
				if iv := reflect.ValueOf(inner); iv.Kind() == reflect.Slice && iv.Len() > 0 {
					if first, ok := iv.Index(0).Interface().(tea.Cmd); ok && first != nil {
						first()
					}
				}
			}
		}
	}
}
