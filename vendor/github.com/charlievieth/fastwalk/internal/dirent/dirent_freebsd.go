//go:build freebsd

package dirent

import (
	"os"
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Fileno), unsafe.Sizeof(syscall.Dirent{}.Fileno))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Namlen), unsafe.Sizeof(syscall.Dirent{}.Namlen))
}

func direntType(buf []byte) os.FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^os.FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return os.ModeDevice
	case syscall.DT_CHR:
		return os.ModeDevice | os.ModeCharDevice
	case syscall.DT_DIR:
		return os.ModeDir
	case syscall.DT_FIFO:
		return os.ModeNamedPipe
	case syscall.DT_LNK:
		return os.ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return os.ModeSocket
	}
	return ^os.FileMode(0) // unknown
}
