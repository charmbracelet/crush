//go:build darwin && go1.13
// +build darwin,go1.13

package fastwalk

import (
	"strings"
	"syscall"
	"unsafe"
)

// TODO: consider using "go linkname" for everything but "opendir" which is not
// implemented in the stdlib

// Implemented in the runtime package (runtime/sys_darwin.go)
func syscall_syscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)
func syscall_syscallPtr(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:linkname syscall_syscall syscall.syscall
//go:linkname syscall_syscallPtr syscall.syscallPtr

func closedir(dir uintptr) (err error) {
	_, _, e1 := syscall_syscall(libc_closedir_trampoline_addr, dir, 0, 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}

var libc_closedir_trampoline_addr uintptr

//go:cgo_import_dynamic libc_closedir closedir "/usr/lib/libSystem.B.dylib"

func readdir_r(dir uintptr, entry *syscall.Dirent, result **syscall.Dirent) syscall.Errno {
	res, _, _ := syscall_syscall(libc_readdir_r_trampoline_addr, dir, uintptr(unsafe.Pointer(entry)), uintptr(unsafe.Pointer(result)))
	return syscall.Errno(res)
}

var libc_readdir_r_trampoline_addr uintptr

//go:cgo_import_dynamic libc_readdir_r readdir_r "/usr/lib/libSystem.B.dylib"

func opendir(path string) (dir uintptr, err error) {
	// We implent opendir so that we don't have to open a file, duplicate
	// it's FD, then call fdopendir with it.

	const maxPath = len(syscall.Dirent{}.Name) // Tested by TestFastWalk_LongPath

	var buf [maxPath]byte
	if len(path) >= len(buf) {
		return 0, errEINVAL
	}
	if strings.IndexByte(path, 0) != -1 {
		return 0, errEINVAL
	}
	copy(buf[:], path)
	buf[len(path)] = 0
	r0, _, e1 := syscall_syscallPtr(libc_opendir_trampoline_addr, uintptr(unsafe.Pointer(&buf[0])), 0, 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return r0, err
}

var libc_opendir_trampoline_addr uintptr

//go:cgo_import_dynamic libc_opendir opendir "/usr/lib/libSystem.B.dylib"

// Copied from syscall/syscall_unix.go

// Do the interface allocations only once for common
// Errno values.
var (
	errEAGAIN error = syscall.EAGAIN
	errEINVAL error = syscall.EINVAL
	errENOENT error = syscall.ENOENT
)

// errnoErr returns common boxed Errno values, to prevent
// allocations at runtime.
func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case syscall.EAGAIN:
		return errEAGAIN
	case syscall.EINVAL:
		return errEINVAL
	case syscall.ENOENT:
		return errENOENT
	}
	return e
}
