package fastwalk

import (
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type fileInfo struct {
	once sync.Once
	fs.FileInfo
	err error
}

func loadFileInfo(pinfo **fileInfo) *fileInfo {
	ptr := (*unsafe.Pointer)(unsafe.Pointer(pinfo))
	fi := (*fileInfo)(atomic.LoadPointer(ptr))
	if fi == nil {
		fi = &fileInfo{}
		if !atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(pinfo)), // adrr
			nil,                                      // old
			unsafe.Pointer(fi),                       // new
		) {
			fi = (*fileInfo)(atomic.LoadPointer(ptr))
		}
	}
	return fi
}

// StatDirEntry returns a [fs.FileInfo] describing the named file ([os.Stat]).
// If de is a [fastwalk.DirEntry] its Stat method is used and the returned
// FileInfo may be cached from a prior call to Stat. If a cached result is not
// desired, users should just call [os.Stat] directly.
//
// This is a helper function for calling Stat on the DirEntry passed to the
// walkFn argument to [Walk].
//
// The path argument is only used if de is not of type [fastwalk.DirEntry].
// Therefore, de should be the DirEntry describing path.
func StatDirEntry(path string, de fs.DirEntry) (fs.FileInfo, error) {
	if de == nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: syscall.EINVAL}
	}
	if de.Type()&os.ModeSymlink == 0 {
		return de.Info()
	}
	if d, ok := de.(DirEntry); ok {
		return d.Stat()
	}
	return os.Stat(path)
}

// DirEntryDepth returns the depth at which entry de was generated relative
// to the root being walked or -1 if de does not have type [fastwalk.DirEntry].
//
// This is a helper function that saves the user from having to cast the
// [fs.DirEntry] argument to their walk function to a [fastwalk.DirEntry]
// and is equivalent to the below code:
//
//	if d, _ := de.(DirEntry); d != nil {
//		return d.Depth()
//	}
//	return -1
func DirEntryDepth(de fs.DirEntry) int {
	if d, _ := de.(DirEntry); d != nil {
		return d.Depth()
	}
	return -1
}
