//go:build linux

package watcher

import "syscall"

func Ulimit() (uint64, error) {
	var currentLimit uint64 = 0
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return 0, err
	}
	currentLimit = rLimit.Cur
	rLimit.Cur = rLimit.Max / 10 * 8
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return currentLimit, err
	}
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return currentLimit, err
	}
	return rLimit.Cur, nil
}
