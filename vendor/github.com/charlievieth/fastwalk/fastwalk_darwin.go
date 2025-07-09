//go:build darwin && go1.13

package fastwalk

import (
	"os"
	"syscall"
	"unsafe"
)

func (w *walker) readDir(dirName string, depth int) (err error) {
	var fd uintptr
	for {
		fd, err = opendir(dirName)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		return &os.PathError{Op: "opendir", Path: dirName, Err: err}
	}
	defer closedir(fd) //nolint:errcheck

	var p *[]*unixDirent
	if w.sortMode != SortNone {
		p = direntSlicePool.Get().(*[]*unixDirent)
	}
	defer putDirentSlice(p)

	skipFiles := false
	var dirent syscall.Dirent
	var entptr *syscall.Dirent
	for {
		if errno := readdir_r(fd, &dirent, &entptr); errno != 0 {
			if errno == syscall.EINTR {
				continue
			}
			return &os.PathError{Op: "readdir", Path: dirName, Err: errno}
		}
		if entptr == nil { // EOF
			break
		}
		// Darwin may return a zero inode when a directory entry has been
		// deleted but not yet removed from the directory. The man page for
		// getdirentries(2) states that programs are responsible for skipping
		// those entries:
		//
		//   Users of getdirentries() should skip entries with d_fileno = 0,
		//   as such entries represent files which have been deleted but not
		//   yet removed from the directory entry.
		//
		if dirent.Ino == 0 {
			continue
		}
		typ := dtToType(dirent.Type)
		if skipFiles && typ.IsRegular() {
			continue
		}
		name := (*[len(syscall.Dirent{}.Name)]byte)(unsafe.Pointer(&dirent.Name))[:]
		for i, c := range name {
			if c == 0 {
				name = name[:i]
				break
			}
		}
		// Check for useless names before allocating a string.
		if string(name) == "." || string(name) == ".." {
			continue
		}
		nm := string(name)
		de := newUnixDirent(dirName, nm, typ, depth)
		if w.sortMode == SortNone {
			if err := w.onDirEnt(dirName, nm, de); err != nil {
				if err != ErrSkipFiles {
					return err
				}
				skipFiles = true
			}
		} else {
			*p = append(*p, de)
		}
	}
	if w.sortMode == SortNone {
		return nil
	}

	dents := *p
	sortDirents(w.sortMode, dents)
	for _, d := range dents {
		d := d
		if skipFiles && d.typ.IsRegular() {
			continue
		}
		if err := w.onDirEnt(dirName, d.Name(), d); err != nil {
			if err != ErrSkipFiles {
				return err
			}
			skipFiles = true
		}
	}
	return nil
}

func dtToType(typ uint8) os.FileMode {
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
	return ^os.FileMode(0)
}
