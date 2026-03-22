package dialog

import "testing"

func TestKeyboardShortcutsIncludeSubagentNavigation(t *testing.T) {
	t.Parallel()

	shortcuts := (&KeyboardShortcuts{}).getShortcutCategories()

	foundCategory := false
	foundOpenChild := false
	foundNextChild := false
	foundPrevChild := false
	foundParent := false

	for _, category := range shortcuts {
		if category.Name == "聊天与 Subagent 导航" {
			foundCategory = true
		}
		for _, shortcut := range category.Shortcuts {
			switch {
			case shortcut.Keys == "ctrl+→" && shortcut.Description == "进入当前选中的 subagent":
				foundOpenChild = true
			case shortcut.Keys == "ctrl+↓" && shortcut.Description == "进入下一个 subagent":
				foundNextChild = true
			case shortcut.Keys == "ctrl+↑" && shortcut.Description == "进入上一个 subagent":
				foundPrevChild = true
			case shortcut.Keys == "ctrl+←" && shortcut.Description == "退出到父会话 / 主会话":
				foundParent = true
			}
		}
	}

	if !foundCategory || !foundOpenChild || !foundNextChild || !foundPrevChild || !foundParent {
		t.Fatalf(
			"expected subagent navigation shortcuts in help dialog: category=%v open=%v next=%v prev=%v parent=%v",
			foundCategory,
			foundOpenChild,
			foundNextChild,
			foundPrevChild,
			foundParent,
		)
	}
}
