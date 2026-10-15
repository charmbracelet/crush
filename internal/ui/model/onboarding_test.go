package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// initWorkspace là dự án chưa khởi tạo: ghi lại việc đánh dấu và việc lấy
// prompt khởi tạo (chỉ xảy ra khi người dùng chọn Yep).
type initWorkspace struct {
	*countingWorkspace

	marked      bool
	promptAsked bool
}

func (w *initWorkspace) ProjectNeedsInitialization() (bool, error) { return true, nil }

func (w *initWorkspace) MarkProjectInitialized() error {
	w.marked = true
	return nil
}

func (w *initWorkspace) InitializePrompt() (string, error) {
	w.promptAsked = true
	return "initialize", nil
}

// Hộp thoại "khởi tạo dự án" mặc định chọn Nope: một cú Enter lỡ tay (gõ
// trước khi màn hình hiện xong) không được chạy cả một lượt quét dự án.
func TestInitializePromptDefaultsToNo(t *testing.T) {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("p", config.ProviderConfig{ID: "p", APIKey: "k", Models: []catwalk.Model{{ID: "m"}}})
	ws := &initWorkspace{countingWorkspace: &countingWorkspace{
		ready: true,
		cfg: &config.Config{
			Providers: providers,
			Options:   &config.Options{TUI: &config.TUIOptions{}},
			Models:    map[config.SelectedModelType]config.SelectedModel{},
		},
	}}

	m := New(common.DefaultCommon(ws), "", false)
	require.Equal(t, uiInitialize, m.state)
	require.False(t, m.onboarding.yesInitializeSelected)

	for _, cmd := range m.updateInitializeView(tea.KeyPressMsg{Code: tea.KeyEnter}) {
		if cmd != nil {
			cmd()
		}
	}
	require.True(t, ws.marked, "Nope still records the answer so the prompt does not come back")
	require.False(t, ws.promptAsked, "a stray enter must not start initialization")
	require.Equal(t, uiLanding, m.state)
}
