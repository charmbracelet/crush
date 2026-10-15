//go:build windows

package provider

import "os/exec"

// setProcessGroup không làm gì trên Windows: Go huỷ bằng Process.Kill, và
// WaitDelay vẫn đóng pipe để Run trả về.
func setProcessGroup(*exec.Cmd) {}
