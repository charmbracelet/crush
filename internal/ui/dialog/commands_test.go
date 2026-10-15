package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
)

// cfgWorkspace chỉ cung cấp cấu hình mà bảng lệnh đọc khi dựng danh sách.
type cfgWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w cfgWorkspace) Config() *config.Config { return w.cfg }

// newCommandsDialog dựng bảng lệnh với model có các mức suy luận (để có mục
// "Select Reasoning Effort").
func newCommandsDialog() *Commands {
	sty := styles.CharmtonePantera()
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("p", config.ProviderConfig{ID: "p", Models: []catwalk.Model{
		{ID: "m", CanReason: true, ReasoningLevels: []string{"low", "high"}},
	}})
	cfg := &config.Config{
		Options:   &config.Options{TUI: &config.TUIOptions{}},
		Providers: providers,
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "p", Model: "m"},
		},
		Agents: map[string]config.Agent{config.AgentCoder: {Model: config.SelectedModelTypeLarge}},
	}
	return &Commands{
		com:        &common.Common{Styles: &sty, Workspace: cfgWorkspace{cfg: cfg}},
		hasSession: true,
	}
}

func commandsByID(c *Commands) map[string]*CommandItem {
	out := map[string]*CommandItem{}
	for _, item := range c.defaultCommands() {
		out[item.ID()] = item
	}
	return out
}

// Gợi ý phím trên bảng lệnh khớp với phím chính hiện hành (các phím Alt), không
// còn các phím Ctrl cũ.
func TestCommandsShowCurrentKeyHints(t *testing.T) {
	t.Parallel()

	items := commandsByID(newCommandsDialog())
	for id, want := range map[string]string{
		"new_session":             "alt+n",
		"switch_session":          "alt+s",
		"switch_model":            "alt+m",
		"switch_provider":         "alt+p",
		"select_reasoning_effort": "alt+e",
		"home":                    "alt+w",
	} {
		item, ok := items[id]
		require.Truef(t, ok, "command %q is missing", id)
		require.Equalf(t, want, item.shortcut, "key hint of %q", id)
	}
}

// Mục "Home" chạy cùng việc với Alt+W, và tìm được bằng chữ "workspace".
func TestCommandsHomeEntry(t *testing.T) {
	t.Parallel()

	items := commandsByID(newCommandsDialog())
	home := items["home"]
	require.NotNil(t, home)
	require.Equal(t, ActionGoHome{}, home.action)
	require.Contains(t, home.Filter(), "workspace")

	require.Equal(t, ActionOpenDialog{ProvidersID}, items["switch_provider"].action)
	require.Equal(t, ActionOpenDialog{ReasoningID}, items["select_reasoning_effort"].action)
}

// Hộp thoại xác nhận về Home: Yes trả ActionGoHomeConfirmed (không phải thoát
// hẳn), No/Esc đóng lại, mặc định là No.
func TestQuitToHomeConfirmation(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	com := &common.Common{Styles: &sty}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	left := tea.KeyPressMsg{Code: tea.KeyLeft}

	q := NewQuitToHome(com)
	require.IsType(t, ActionClose{}, q.HandleMsg(enter), "default is No")
	require.Nil(t, q.HandleMsg(left))
	require.IsType(t, ActionGoHomeConfirmed{}, q.HandleMsg(enter))

	q = NewQuitToHome(com)
	require.IsType(t, ActionGoHomeConfirmed{}, q.HandleMsg(keyMsg('y')))
	q = NewQuitToHome(com)
	require.IsType(t, ActionClose{}, q.HandleMsg(keyMsg('n')))
	require.IsType(t, ActionClose{}, q.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))

	// Bản Quit thường vẫn thoát hẳn.
	q = NewQuit(com)
	require.Nil(t, q.HandleMsg(left))
	require.IsType(t, ActionQuit{}, q.HandleMsg(enter))

	question, _, _ := NewQuitToHome(com).texts()
	require.Contains(t, question, "Home")
	question, _, _ = NewQuit(com).texts()
	require.Contains(t, question, "quit")
}

// Hộp thoại về Home nói đúng điều sắp xảy ra (run dừng theo), không phải câu hỏi
// thoát chung chung.
func TestQuitToHomeDrawsItsOwnQuestion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	com := &common.Common{Styles: &sty}

	home := screenText(drawQuit(t, NewQuitToHome(com)))
	require.Contains(t, home, "go to Home?")
	require.Contains(t, home, "task stops")
	require.NotContains(t, home, "Are you sure you want to quit?")

	plain := screenText(drawQuit(t, NewQuit(com)))
	require.Contains(t, plain, "Are you sure you want to quit?")
	require.NotContains(t, plain, "Home")
}
