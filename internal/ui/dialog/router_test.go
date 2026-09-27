// internal/ui/dialog/router_test.go
package dialog

import (
	"errors"
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// routerTestWorkspace is a minimal workspace.Workspace fake that serves a
// fixed config and records every SetConfigField call, mirroring the
// pattern already used by sessionMouseWorkspace in sessions_mouse_test.go.
type routerSetCall struct {
	key   string
	value any
}

type routerTestWorkspace struct {
	workspace.Workspace
	cfg      *config.Config
	setCalls []routerSetCall
	// setErr, when non-nil, makes SetConfigField fail while still recording
	// the call, so tests can verify a failed write never updates the row's
	// displayed value.
	setErr error
}

func (w *routerTestWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *routerTestWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	w.setCalls = append(w.setCalls, routerSetCall{key, value})
	if w.setErr != nil {
		return w.setErr
	}
	return nil
}

func newTestRouterDialog(t *testing.T, cfg *config.Config) (*Router, *routerTestWorkspace) {
	t.Helper()
	s := styles.CharmtonePantera()
	ws := &routerTestWorkspace{cfg: cfg}
	return NewRouter(&common.Common{Workspace: ws, Styles: &s}), ws
}

func TestRouter_ConstructsWithDefaultsWhenRouterOptionsNil(t *testing.T) {
	t.Parallel()

	dialog, _ := newTestRouterDialog(t, &config.Config{Options: &config.Options{}})
	require.Equal(t, "false", dialog.selectedFieldValue(t, "enabled"))
	require.Equal(t, "openrouter", dialog.selectedFieldValue(t, "provider"))
}

func TestRouter_ToggleEnabledPersistsBool(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{Enabled: false},
	}})
	// "enabled" is the first row.
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Len(t, ws.setCalls, 1)
	require.Equal(t, "options.router.enabled", ws.setCalls[0].key)
	require.Equal(t, true, ws.setCalls[0].value)
}

func TestRouter_CycleProviderPersistsString(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{Provider: "openrouter"},
	}})
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown}) // move to "provider" row
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Switching backend also clears base_url and model, so the previous
	// backend's URL or model id is never sent to the new one. The provider
	// enum is derived from the backend table, so cycle to whatever follows
	// openrouter there rather than pinning a specific backend.
	nextProvider := router.ProviderNames()[1]
	require.Equal(t, []routerSetCall{
		{key: "options.router.provider", value: nextProvider},
		{key: "options.router.base_url", value: ""},
		{key: "options.router.model", value: ""},
	}, ws.setCalls)
}

func TestRouter_EditBaseURLPersistsOnConfirm(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{BaseURL: "http://old"},
	}})
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown}) // provider
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown}) // base_url
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	item := dialog.selectedItem()
	require.NotNil(t, item)
	require.True(t, item.Editing())
	// Same-package test: set the edit buffer directly rather than
	// simulating character-by-character key presses.
	item.input.SetValue("http://new:8021")

	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Len(t, ws.setCalls, 1)
	require.Equal(t, "options.router.base_url", ws.setCalls[0].key)
	require.Equal(t, "http://new:8021", ws.setCalls[0].value)
}

func TestRouter_CancelEditDoesNotPersist(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{BaseURL: "http://old"},
	}})
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	item := dialog.selectedItem()
	item.input.SetValue("http://should-not-persist")
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})

	require.Empty(t, ws.setCalls)
	require.False(t, item.Editing())
	require.Equal(t, "http://old", item.Value())
}

func TestRouter_InvalidConfidenceThresholdRejected(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{ConfidenceThreshold: 0.7},
	}})
	for range 5 { // enabled, provider, base_url, api_key, model -> confidence_threshold
		dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	item := dialog.selectedItem()
	require.Equal(t, "confidence_threshold", item.spec.key)
	item.input.SetValue("2")
	action := dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Empty(t, ws.setCalls, "an out-of-range threshold must not be persisted")
	require.IsType(t, ActionCmd{}, action)
}

func TestRouter_InvalidTimeoutRejected(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{TimeoutMS: 1500},
	}})
	for range 6 { // ... -> timeout_ms
		dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	item := dialog.selectedItem()
	require.Equal(t, "timeout_ms", item.spec.key)
	item.input.SetValue("0")
	action := dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Empty(t, ws.setCalls, "a non-positive timeout must not be persisted")
	require.IsType(t, ActionCmd{}, action)
}

// TestRouter_FailedWriteDoesNotUpdateDisplayedValue pins finding 3 from the
// final whole-branch review: a failed config write must never leave the
// row displaying a value that was never actually saved to disk.
func TestRouter_FailedWriteDoesNotUpdateDisplayedValue(t *testing.T) {
	t.Parallel()

	dialog, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{Enabled: false},
	}})
	ws.setErr = errors.New("disk full")

	item := dialog.selectedItem()
	require.NotNil(t, item)
	require.Equal(t, "false", item.Value())

	action := dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}) // toggle "enabled"

	require.Len(t, ws.setCalls, 1, "the write must still be attempted")
	require.IsType(t, ActionCmd{}, action, "a failed write must report the error")
	require.Equal(t, "false", item.Value(),
		"a failed write must leave the row showing its prior, actually-saved value")
}

// TestRouter_EnterOnModelPoolOpensPicker proves the Model Pool row no
// longer opens inline text editing: Enter now opens the RouterModelPool
// picker dialog (checkable list of OpenRouter models) instead, since
// hand-typing comma-separated model ids doesn't discover what's actually
// available. The row's value is still only ever changed by that picker
// persisting directly, hence no local edit-mode state to assert on here.
func TestRouter_EnterOnModelPoolOpensPicker(t *testing.T) {
	t.Parallel()

	dialog, _ := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{},
	}})
	for {
		item := dialog.selectedItem()
		if item.spec.key == "model_pool" {
			break
		}
		dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	}

	action := dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Equal(t, ActionOpenDialog{RouterModelPoolID}, action)
	require.False(t, dialog.selectedItem().Editing(),
		"the row must not enter inline-edit mode now that Enter opens the picker instead")
}

func TestRouter_ModelPoolSeedsFromCommaJoinedConfig(t *testing.T) {
	t.Parallel()

	dialog, _ := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{ModelPool: []string{"anthropic/claude-opus-4", "anthropic/claude-haiku-4"}},
	}})
	require.Equal(t, "anthropic/claude-opus-4, anthropic/claude-haiku-4", dialog.selectedFieldValue(t, "model_pool"))
}

func TestRouter_CloseReturnsActionClose(t *testing.T) {
	t.Parallel()

	dialog, _ := newTestRouterDialog(t, &config.Config{Options: &config.Options{}})
	action := dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.IsType(t, ActionClose{}, action)
}

// selectedFieldValue is a small test helper: it re-selects row 0 and walks
// forward until it finds the field with the given key, returning its
// current value. Keeps the "which row index is which field" mapping out
// of every individual test.
func (r *Router) selectedFieldValue(t *testing.T, key string) string {
	t.Helper()
	for _, spec := range routerFieldSpecs {
		if spec.key != key {
			continue
		}
	}
	for i := range routerFieldSpecs {
		r.list.SetSelected(i)
		item := r.selectedItem()
		require.NotNil(t, item)
		if item.spec.key == key {
			return item.Value()
		}
	}
	t.Fatalf("field %q not found", key)
	return ""
}

// routerRowIndex returns the index of key in routerFieldSpecs, failing
// the test if it's not found — kept next to selectedFieldValue as the
// other half of "which row index is which field".
func routerRowIndex(t *testing.T, key string) int {
	t.Helper()
	for i, spec := range routerFieldSpecs {
		if spec.key == key {
			return i
		}
	}
	t.Fatalf("field %q not found", key)
	return -1
}

// startEditOn selects the row for key and puts it into inline-edit mode,
// bypassing HandleMsg's key routing (bool/enum fields toggle in place on
// Enter rather than ever entering edit mode, so this only makes sense
// for text/float/int/list-kind fields).
func (r *Router) startEditOn(t *testing.T, key string) *RouterItem {
	t.Helper()
	r.list.SetSelected(routerRowIndex(t, key))
	item := r.selectedItem()
	require.NotNil(t, item)
	item.StartEdit()
	return item
}

// TestRouter_CursorAdvancesOneRowPerFieldBeingEdited proves the dialog's
// Cursor() tracks which row is actually being edited instead of always
// reporting the same Y regardless of selection — the bug this pins: two
// adjacent rows put into edit mode must report cursor Y one line apart,
// and a row further down the list must report a strictly greater Y than
// one above it, however many rows apart they are.
func TestRouter_CursorAdvancesOneRowPerFieldBeingEdited(t *testing.T) {
	t.Parallel()

	dialog, _ := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{},
	}})
	scr := uv.NewScreenBuffer(80, 30)
	area := image.Rect(0, 0, 80, 30)

	dialog.startEditOn(t, "base_url")
	baseURLCursor := dialog.Draw(scr, area)
	require.NotNil(t, baseURLCursor)

	dialog.startEditOn(t, "api_key")
	apiKeyCursor := dialog.Draw(scr, area)
	require.NotNil(t, apiKeyCursor)

	require.Equal(t, baseURLCursor.Y+1, apiKeyCursor.Y,
		"api_key sits exactly one row below base_url in routerFieldSpecs")

	dialog.startEditOn(t, "timeout_ms")
	timeoutCursor := dialog.Draw(scr, area)
	require.NotNil(t, timeoutCursor)
	require.Greater(t, timeoutCursor.Y, apiKeyCursor.Y,
		"a field further down the list must report a strictly greater cursor Y")
}

// TestRouter_CursorXSkipsPastTheLabelPrefix proves the cursor's X
// position accounts for the "label: " text rendered before the input
// (Render draws them on one line: style.Render(label + ": " +
// input.View())) — without this, the cursor would land near column 0,
// inside or before the label text itself, rather than after it where
// the actual edit input starts.
func TestRouter_CursorXSkipsPastTheLabelPrefix(t *testing.T) {
	t.Parallel()

	dialog, _ := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{},
	}})

	item := dialog.startEditOn(t, "api_key")
	scr := uv.NewScreenBuffer(80, 30)
	cur := dialog.Draw(scr, image.Rect(0, 0, 80, 30))
	require.NotNil(t, cur)

	minX := lipgloss.Width(item.spec.label + ": ")
	require.GreaterOrEqual(t, cur.X, minX,
		"cursor X must land at or after the label prefix, not inside/before it")
}
