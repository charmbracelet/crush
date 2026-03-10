package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// KeyboardShortcutsID is the identifier for the keyboard shortcuts dialog.
const KeyboardShortcutsID = "keyboard_shortcuts"

// KeyboardShortcuts represents a dialog that shows all keyboard shortcuts.
type KeyboardShortcuts struct {
	com    *common.Common
	keyMap struct {
		Close,
		ScrollUp,
		ScrollDown,
		PageUp,
		PageDown key.Binding
	}
	help       help.Model
	windowWidth int
	windowHeight int
	viewport   viewport.Model
	content    string
}

var _ Dialog = (*KeyboardShortcuts)(nil)

// ShortcutCategory represents a category of shortcuts.
type ShortcutCategory struct {
	Name      string
	Shortcuts []Shortcut
}

// Shortcut represents a single keyboard shortcut.
type Shortcut struct {
	Keys        string
	Description string
}

// NewKeyboardShortcuts creates a new keyboard shortcuts dialog.
func NewKeyboardShortcuts(com *common.Common) (*KeyboardShortcuts, error) {
	k := &KeyboardShortcuts{
		com: com,
	}

	k.help = help.New()
	k.help.Styles = com.Styles.DialogHelpStyles()

	k.keyMap.Close = key.NewBinding(
		key.WithKeys("esc", "alt+esc", "q"),
		key.WithHelp("esc/q", "close"),
	)
	k.keyMap.ScrollUp = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "scroll up"),
	)
	k.keyMap.ScrollDown = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "scroll down"),
	)
	k.keyMap.PageUp = key.NewBinding(
		key.WithKeys("pgup", "b"),
		key.WithHelp("pgup", "page up"),
	)
	k.keyMap.PageDown = key.NewBinding(
		key.WithKeys("pgdown", "space", "f"),
		key.WithHelp("pgdn", "page down"),
	)

	// Initialize viewport
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{
		Up:    k.keyMap.ScrollUp,
		Down:  k.keyMap.ScrollDown,
		Left:  key.NewBinding(key.WithDisabled()),
		Right: key.NewBinding(key.WithDisabled()),
		PageUp:       k.keyMap.PageUp,
		PageDown:     k.keyMap.PageDown,
		HalfPageUp:   key.NewBinding(key.WithDisabled()),
		HalfPageDown: key.NewBinding(key.WithDisabled()),
	}
	k.viewport = vp

	return k, nil
}

// ID implements Dialog.
func (k *KeyboardShortcuts) ID() string {
	return KeyboardShortcutsID
}

// HandleMsg implements Dialog.
func (k *KeyboardShortcuts) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, k.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, k.keyMap.ScrollUp, k.keyMap.ScrollDown, k.keyMap.PageUp, k.keyMap.PageDown):
			var cmd tea.Cmd
			k.viewport, cmd = k.viewport.Update(msg)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (k *KeyboardShortcuts) Cursor() *tea.Cursor {
	return nil
}

// getShortcutCategories returns all shortcut categories with their shortcuts.
func (k *KeyboardShortcuts) getShortcutCategories() []ShortcutCategory {
	return []ShortcutCategory{
		{
			Name: "全局快捷键",
			Shortcuts: []Shortcut{
				{"ctrl+p", "打开命令面板"},
				{"ctrl+l / ctrl+m", "切换模型"},
				{"ctrl+s", "会话管理"},
				{"ctrl+g", "查看快捷键帮助"},
				{"ctrl+c", "退出程序"},
				{"ctrl+z", "挂起程序"},
				{"tab", "切换焦点"},
			},
		},
		{
			Name: "编辑器快捷键",
			Shortcuts: []Shortcut{
				{"enter", "发送消息"},
				{"ctrl+j / shift+enter", "换行"},
				{"ctrl+o", "打开外部编辑器"},
				{"@", "提及文件"},
				{"/", "搜索命令"},
				{"ctrl+f", "添加图片"},
				{"ctrl+v / alt+v", "从剪贴板粘贴图片"},
				{"ctrl+r 然后 1-9", "按编号删除附件"},
				{"r (在 ctrl+r 后)", "删除所有附件"},
				{"del", "删除最后一个附件"},
				{"ctrl+del", "清空所有附件"},
				{"esc", "取消删除模式"},
				{"↑/↓", "历史记录导航"},
			},
		},
		{
			Name: "聊天导航",
			Shortcuts: []Shortcut{
				{"ctrl+n", "新建会话"},
				{"ctrl+t / ctrl+space", "切换任务面板"},
				{"ctrl+d", "切换详情"},
				{"↑/↓", "向上/向下滚动"},
				{"shift+↑/↓", "逐项滚动"},
				{"←/→", "切换面板区域"},
				{"g", "滚动到顶部"},
				{"G", "滚动到底部"},
				{"f / pgdn", "向下翻页"},
				{"b / pgup", "向上翻页"},
				{"d", "向下半页"},
				{"u", "向上半页"},
				{"space", "展开/收起"},
				{"c / y", "复制内容"},
				{"esc", "清除选择"},
			},
		},
		{
			Name: "队列管理",
			Shortcuts: []Shortcut{
				{"x", "删除队列项"},
				{"ctrl+x", "清空队列"},
			},
		},
		{
			Name: "初始化",
			Shortcuts: []Shortcut{
				{"y", "确认"},
				{"n / esc", "取消"},
				{"tab / ←/→", "切换选项"},
				{"enter", "选择"},
			},
		},
	}
}

// Draw implements Dialog.
func (k *KeyboardShortcuts) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := k.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight*2, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))

	if area.Dx() != k.windowWidth || area.Dy() != k.windowHeight {
		k.windowWidth = area.Dx()
		k.windowHeight = area.Dy()
		
		// Update viewport size
		innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
		contentHeight := height - t.Dialog.Title.GetVerticalFrameSize() -
			titleContentHeight - t.Dialog.HelpView.GetVerticalFrameSize() -
			t.Dialog.View.GetVerticalFrameSize()
		k.viewport.SetWidth(max(0, innerWidth))
		k.viewport.SetHeight(max(0, contentHeight))
	}

	k.help.SetWidth(width - t.Dialog.View.GetHorizontalFrameSize())

	rc := NewRenderContext(t, width)
	rc.Title = "快捷键帮助"

	// Build content if not already built
	if k.content == "" {
		k.content = k.buildContent(width - t.Dialog.View.GetHorizontalFrameSize())
	}

	// Set viewport content and render
	k.viewport.SetContent(k.content)
	contentView := t.Dialog.List.Height(k.viewport.Height()).Render(k.viewport.View())
	rc.AddPart(contentView)
	rc.Help = k.help.View(k)

	view := rc.Render()

	cur := k.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// buildContent builds the shortcut categories content string.
func (k *KeyboardShortcuts) buildContent(innerWidth int) string {
	var content strings.Builder
	categories := k.getShortcutCategories()

	for i, category := range categories {
		if i > 0 {
			content.WriteString("\n")
		}

		// Category header
		headerStyle := k.com.Styles.Dialog.PrimaryText
		content.WriteString(headerStyle.Render(category.Name))
		content.WriteString("\n")

		// Shortcuts
		for _, shortcut := range category.Shortcuts {
			keyStyle := k.com.Styles.Dialog.SecondaryText
			descStyle := k.com.Styles.Base

			// Format: keys - description
			line := fmt.Sprintf("  %s  %s",
				keyStyle.Render(shortcut.Keys+"."),
				descStyle.Render(shortcut.Description),
			)
			line = lipgloss.NewStyle().MaxWidth(innerWidth - 4).Render(line)
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	return content.String()
}

// ShortHelp implements help.KeyMap.
func (k *KeyboardShortcuts) ShortHelp() []key.Binding {
	return []key.Binding{
		k.keyMap.ScrollUp,
		k.keyMap.ScrollDown,
		k.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (k *KeyboardShortcuts) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.keyMap.ScrollUp, k.keyMap.ScrollDown},
		{k.keyMap.PageUp, k.keyMap.PageDown},
		{k.keyMap.Close},
	}
}
