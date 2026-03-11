//go:build !windows && !unix

package shell

import "syscall"

func detachedSysProcAttr(detached bool) *syscall.SysProcAttr {
	_ = detached
	return nil
}
