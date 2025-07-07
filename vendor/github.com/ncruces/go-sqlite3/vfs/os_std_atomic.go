//go:build !linux || !(amd64 || arm64 || riscv64)

package vfs

import "os"

func osBatchAtomic(*os.File) bool {
	return false
}
