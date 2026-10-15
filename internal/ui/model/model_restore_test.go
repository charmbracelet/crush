package model

import (
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// modelLogWorkspace ghi lại model lớn được ghi, theo thứ tự.
type modelLogWorkspace struct {
	*countingWorkspace
	mu      sync.Mutex
	written []string
}

func (w *modelLogWorkspace) UpdatePreferredModel(scope config.Scope, t config.SelectedModelType, sel config.SelectedModel) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t == config.SelectedModelTypeLarge {
		w.written = append(w.written, sel.Provider+"/"+sel.Model)
	}
	return w.countingWorkspace.UpdatePreferredModel(scope, t, sel)
}

func (w *modelLogWorkspace) log() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.written...)
}

func restoreUI(t *testing.T) (*UI, *modelLogWorkspace) {
	t.Helper()
	m, cw := newModelScopeUI(t)
	cw.cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{Provider: "p", Model: "a"}
	cw.cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{Provider: "p", Model: "small"}
	ws := &modelLogWorkspace{countingWorkspace: cw}
	m.com.Workspace = ws
	return m, ws
}

func lastAssistantFor(model string) []message.Message {
	return []message.Message{{Role: message.Assistant, Provider: "p", Model: model}}
}

func modelWithID(id string) catwalk.Model { return catwalk.Model{ID: id} }

// availableModels khai các model để IsModelAvailable nhận chúng.
func availableModels(t *testing.T, cw *countingWorkspace, ids ...string) {
	t.Helper()
	pc, ok := cw.cfg.Providers.Get("p")
	require.True(t, ok)
	pc.Models = nil
	for _, id := range ids {
		pc.Models = append(pc.Models, modelWithID(id))
	}
	cw.cfg.Providers.Set("p", pc)
}

// Mở session khác lúc có run vẫn khôi phục model của session đó: lượt đang
// chạy đã ghim model riêng nên không bị đổi provider giữa chừng.
func TestRestoreModelFromSessionWhileBusyStillRestores(t *testing.T) {
	m, ws := restoreUI(t)
	availableModels(t, ws.countingWorkspace, "a", "b")
	m.agentBusyCache.set(true)

	cmd := m.restoreModelFromSession(lastAssistantFor("b"))
	require.NotNil(t, cmd)
	msg, ok := cmd().(modelRestoredMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	require.Equal(t, []string{"p/b"}, ws.log())
}

// Ghi cấu hình chạy nền, lần lượt; trong hàng chờ chỉ lần mới nhất được chạy.
func TestRestoreModelLatestRequestWins(t *testing.T) {
	m, ws := restoreUI(t)
	first := m.requestModelRestore(modelRestore{sel: config.SelectedModel{Provider: "p", Model: "x"}})
	require.NotNil(t, first)
	require.Nil(t, m.requestModelRestore(modelRestore{sel: config.SelectedModel{Provider: "p", Model: "y"}}), "queued while x is written")
	require.Nil(t, m.requestModelRestore(modelRestore{sel: config.SelectedModel{Provider: "p", Model: "z"}}), "replaces y")

	next := m.handleModelRestored(first().(modelRestoredMsg))
	require.NotNil(t, next)
	m.handleModelRestored(next().(modelRestoredMsg))
	require.Equal(t, []string{"p/x", "p/z"}, ws.log(), "y was superseded before it ran")
	require.Nil(t, m.restorer.target())
}

// A→B→A nhanh: lần về A không được bị bỏ vì cấu hình (chưa ghi xong B) vẫn
// đang là A; kết thúc phải ở model của A.
func TestRestoreModelQuickBackAndForthEndsOnLastSession(t *testing.T) {
	m, ws := restoreUI(t)
	availableModels(t, ws.countingWorkspace, "a", "b")

	toB := m.restoreModelFromSession(lastAssistantFor("b"))
	require.NotNil(t, toB)
	require.Nil(t, m.restoreModelFromSession(lastAssistantFor("a")), "queued behind the write of b")

	toA := m.handleModelRestored(toB().(modelRestoredMsg))
	require.NotNil(t, toA, "the queued restore of a must run")
	m.handleModelRestored(toA().(modelRestoredMsg))
	require.Equal(t, []string{"p/b", "p/a"}, ws.log())
}

// Đổi model khi có run (ở bất kỳ session nào) không còn bị chặn; thông báo
// nói rõ thiết lập mới áp cho tin nhắn sau.
func TestModelSwitchAllowedWhileAnySessionBusy(t *testing.T) {
	m, ws := restoreUI(t)
	m.agentBusyCache.set(true)

	cmd := m.handleSelectModel(dialog.ActionSelectModel{
		Model:     config.SelectedModel{Provider: "p", Model: "b"},
		ModelType: config.SelectedModelTypeLarge,
	})
	require.NotNil(t, cmd)
	require.Equal(t, []string{"p/b"}, ws.log(), "the selection is written even while a run is in flight")
	require.Contains(t, appliesNextNote(true), "next message")
	require.Empty(t, appliesNextNote(false))
}
