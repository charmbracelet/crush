package herdr

import (
	"slices"
	"strings"
)

// envVars là các biến herdr gắn vào pane để agent báo trạng thái qua socket.
var envVars = []string{"HERDR_ENV", "HERDR_SOCKET_PATH", "HERDR_PANE_ID", "HERDR_BIN_PATH"}

// StripEnv trả bản sao env không có các biến HERDR_*. Dùng cho tiến trình con
// không được tự báo trạng thái về pane của tiến trình cha: server nền của dự án
// (client TUI đã báo thay qua luồng sự kiện; server cũng báo thì hai nguồn
// tranh nhau một pane và seq của nguồn khởi động sau làm báo cáo của nguồn kia
// bị coi là cũ) và CLI con của tool giao việc (claude/codex tự báo bằng hook
// của chúng, đè trạng thái của agent-cli).
func StripEnv(env []string) []string {
	return slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(envVars, name)
	})
}
