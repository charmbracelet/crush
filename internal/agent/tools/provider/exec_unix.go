//go:build !windows

package provider

import (
	"os/exec"
	"syscall"
)

// setProcessGroup đưa CLI con vào nhóm tiến trình riêng và khi huỷ thì gửi
// SIGTERM cho cả nhóm, để tiến trình cháu (dev server, watcher) không sống
// tiếp giữ pipe sau khi lượt giao việc đã kết thúc.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
