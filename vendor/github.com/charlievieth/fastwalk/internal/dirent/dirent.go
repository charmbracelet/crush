//go:build aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package dirent

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// readInt returns the size-bytes unsigned integer in native byte order at offset off.
func readInt(b []byte, off, size uintptr) (u uint64, ok bool) {
	if len(b) < int(off+size) {
		return 0, false
	}
	if isBigEndian {
		return readIntBE(b[off:], size), true
	}
	return readIntLE(b[off:], size), true
}

func readIntBE(b []byte, size uintptr) uint64 {
	switch size {
	case 1:
		return uint64(b[0])
	case 2:
		_ = b[1] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[1]) | uint64(b[0])<<8
	case 4:
		_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[3]) | uint64(b[2])<<8 | uint64(b[1])<<16 | uint64(b[0])<<24
	case 8:
		_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
			uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56
	default:
		panic("syscall: readInt with unsupported size")
	}
}

func readIntLE(b []byte, size uintptr) uint64 {
	switch size {
	case 1:
		return uint64(b[0])
	case 2:
		_ = b[1] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[0]) | uint64(b[1])<<8
	case 4:
		_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24
	case 8:
		_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
			uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	default:
		panic("syscall: readInt with unsupported size")
	}
}

const InvalidMode = os.FileMode(1<<32 - 1)

func Parse(buf []byte) (consumed int, name string, typ os.FileMode) {

	reclen, ok := direntReclen(buf)
	if !ok || reclen > uint64(len(buf)) {
		// WARN: this is a hard error because we consumed 0 bytes
		// and not stopping here could lead to an infinite loop.
		return 0, "", InvalidMode
	}
	consumed = int(reclen)
	rec := buf[:reclen]

	ino, ok := direntIno(rec)
	if !ok {
		return consumed, "", InvalidMode
	}
	// When building to wasip1, the host runtime might be running on Windows
	// or might expose a remote file system which does not have the concept
	// of inodes. Therefore, we cannot make the assumption that it is safe
	// to skip entries with zero inodes.
	if ino == 0 && runtime.GOOS != "wasip1" {
		return consumed, "", InvalidMode
	}

	typ = direntType(buf)

	const namoff = uint64(unsafe.Offsetof(syscall.Dirent{}.Name))
	namlen, ok := direntNamlen(rec)
	if !ok || namoff+namlen > uint64(len(rec)) {
		return consumed, "", InvalidMode
	}
	namebuf := rec[namoff : namoff+namlen]
	for i, c := range namebuf {
		if c == 0 {
			namebuf = namebuf[:i]
			break
		}
	}
	// Check for useless names before allocating a string.
	if string(namebuf) == "." {
		name = "."
	} else if string(namebuf) == ".." {
		name = ".."
	} else {
		name = string(namebuf)
	}
	return consumed, name, typ
}
