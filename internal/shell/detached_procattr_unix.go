//go:build unix

package shell

import "syscall"

func detachedSysProcAttr(detached bool) *syscall.SysProcAttr {
	if !detached {
		return nil
	}

	return &syscall.SysProcAttr{Setsid: true}
}
