//go:build windows

package shell

import (
	"syscall"

	"golang.org/x/sys/windows"
)

func detachedSysProcAttr(detached bool) *syscall.SysProcAttr {
	if !detached {
		return nil
	}

	return &syscall.SysProcAttr{
		HideWindow: true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP |
			windows.CREATE_NO_WINDOW,
	}
}
