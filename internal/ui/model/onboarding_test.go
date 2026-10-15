package model

import (
	"reflect"
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

// newInitUI dựng UI ở màn "khởi tạo dự án" (dự án chưa khởi tạo).
func newInitUI(t *testing.T) (*UI, *initWorkspace) {
	t.Helper()
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
	return m, ws
}

// runCmdTree chạy cả cây Cmd (Batch lẫn Sequence) và trả các message lá. Kiểu
// của tea.Sequence không xuất ra ngoài nên đọc nó như slice Cmd bằng reflect.
func runCmdTree(cmds ...tea.Cmd) []tea.Msg {
	var out []tea.Msg
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		if rv := reflect.ValueOf(msg); rv.Kind() == reflect.Slice && rv.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
			for i := range rv.Len() {
				if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
					out = append(out, runCmdTree(c)...)
				}
			}
			continue
		}
		out = append(out, msg)
	}
	return out
}

// Gõ lung tung lúc màn "khởi tạo dự án" vừa hiện (lần mở dự án đầu tiên đã
// từng chạy init nhầm) không được bắt đầu khởi tạo: chữ y/Y không còn là phím
// đồng ý. Chỉ chọn Yep (←/→/Tab) rồi Enter mới chạy.
func TestInitializePromptTypingNeverInitializes(t *testing.T) {
	t.Parallel()

	for _, typed := range []string{"yes please", "YES", "y", "Y", "yyy", "hello world, yep!"} {
		m, ws := newInitUI(t)
		var cmds []tea.Cmd
		for _, r := range typed {
			// Chữ n/N là phím "Nope" và kết thúc màn này, nên chỉ gõ chuỗi không có n.
			if r == 'n' || r == 'N' {
				continue
			}
			cmds = append(cmds, m.updateInitializeView(tea.KeyPressMsg{Code: r, Text: string(r)})...)
		}
		require.Emptyf(t, cmds, "typing %q must not start or skip initialization", typed)
		require.Equalf(t, uiInitialize, m.state, "typing %q", typed)
		require.Falsef(t, m.onboarding.yesInitializeSelected, "typing %q must not move the selection", typed)

		// Enter sau cả đoạn gõ vẫn là Nope mặc định: không quét dự án.
		runCmdTree(m.updateInitializeView(tea.KeyPressMsg{Code: tea.KeyEnter})...)
		require.Falsef(t, ws.promptAsked, "typing %q then enter must not initialize", typed)
		require.Truef(t, ws.marked, "Nope still records the answer")
		require.Equal(t, uiLanding, m.state)
	}
}

// Đồng ý chỉ bằng cách chuyển lựa chọn sang Yep (←/→/Tab) rồi Enter; n/N/Esc là
// Nope; mặc định là Nope.
func TestInitializePromptAcceptsOnlyViaSelectionAndEnter(t *testing.T) {
	t.Parallel()

	for name, move := range map[string]tea.KeyPressMsg{
		"right": {Code: tea.KeyRight},
		"left":  {Code: tea.KeyLeft},
		"tab":   {Code: tea.KeyTab},
	} {
		m, ws := newInitUI(t)
		require.Empty(t, m.updateInitializeView(move), name)
		require.True(t, m.onboarding.yesInitializeSelected, "%s selects Yep", name)
		require.False(t, ws.promptAsked)

		runCmdTree(m.updateInitializeView(tea.KeyPressMsg{Code: tea.KeyEnter})...)
		require.True(t, ws.promptAsked, "%s then enter initializes", name)
		require.True(t, ws.marked)
	}

	for name, k := range map[string]tea.KeyPressMsg{
		"n":   {Code: 'n', Text: "n"},
		"N":   {Code: 'N', Text: "N"},
		"esc": {Code: tea.KeyEscape},
	} {
		m, ws := newInitUI(t)
		m.onboarding.yesInitializeSelected = true // Cho thấy n/N/Esc thắng cả khi đang chọn Yep.
		runCmdTree(m.updateInitializeView(k)...)
		require.False(t, ws.promptAsked, name)
		require.True(t, ws.marked, name)
		require.Equal(t, uiLanding, m.state, name)
	}
}
