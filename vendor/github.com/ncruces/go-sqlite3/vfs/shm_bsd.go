//go:build ((freebsd || openbsd || netbsd || dragonfly || illumos) && (386 || arm || amd64 || arm64 || riscv64 || ppc64le) && !sqlite3_dotlk) || sqlite3_flock

package vfs

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"golang.org/x/sys/unix"

	"github.com/ncruces/go-sqlite3/internal/util"
)

type vfsShmParent struct {
	*os.File
	info os.FileInfo

	refs int // +checklocks:vfsShmListMtx

	lock [_SHM_NLOCK]int8 // +checklocks:Mutex
	sync.Mutex
}

var (
	// +checklocks:vfsShmListMtx
	vfsShmList    []*vfsShmParent
	vfsShmListMtx sync.Mutex
)

type vfsShm struct {
	*vfsShmParent
	path    string
	lock    [_SHM_NLOCK]bool
	regions []*util.MappedRegion
}

func (s *vfsShm) Close() error {
	if s.vfsShmParent == nil {
		return nil
	}

	vfsShmListMtx.Lock()
	defer vfsShmListMtx.Unlock()

	// Unlock everything.
	s.shmLock(0, _SHM_NLOCK, _SHM_UNLOCK)

	// Decrease reference count.
	if s.vfsShmParent.refs > 0 {
		s.vfsShmParent.refs--
		s.vfsShmParent = nil
		return nil
	}

	err := s.File.Close()
	for i, g := range vfsShmList {
		if g == s.vfsShmParent {
			vfsShmList[i] = nil
			s.vfsShmParent = nil
			return err
		}
	}
	panic(util.AssertErr())
}

func (s *vfsShm) shmOpen() (rc _ErrorCode) {
	if s.vfsShmParent != nil {
		return _OK
	}

	vfsShmListMtx.Lock()
	defer vfsShmListMtx.Unlock()

	// Stat file without opening it.
	// Closing it would release all POSIX locks on it.
	fi, err := os.Stat(s.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return _IOERR_FSTAT
	}

	// Find a shared file, increase the reference count.
	for _, g := range vfsShmList {
		if g != nil && os.SameFile(fi, g.info) {
			s.vfsShmParent = g
			g.refs++
			return _OK
		}
	}

	// Always open file read-write, as it will be shared.
	f, err := os.OpenFile(s.path,
		os.O_RDWR|os.O_CREATE|_O_NOFOLLOW, 0666)
	if err != nil {
		return _CANTOPEN
	}
	defer func() {
		if rc != _OK {
			f.Close()
		}
	}()

	// Dead man's switch.
	if lock, rc := osTestLock(f, _SHM_DMS, 1); rc != _OK {
		return _IOERR_LOCK
	} else if lock == unix.F_WRLCK {
		return _BUSY
	} else if lock == unix.F_UNLCK {
		if rc := osWriteLock(f, _SHM_DMS, 1); rc != _OK {
			return rc
		}
		if err := f.Truncate(0); err != nil {
			return _IOERR_SHMOPEN
		}
	}
	if rc := osReadLock(f, _SHM_DMS, 1); rc != _OK {
		return rc
	}

	fi, err = f.Stat()
	if err != nil {
		return _IOERR_FSTAT
	}

	// Add the new shared file.
	s.vfsShmParent = &vfsShmParent{
		File: f,
		info: fi,
	}
	for i, g := range vfsShmList {
		if g == nil {
			vfsShmList[i] = s.vfsShmParent
			return _OK
		}
	}
	vfsShmList = append(vfsShmList, s.vfsShmParent)
	return _OK
}

func (s *vfsShm) shmMap(ctx context.Context, mod api.Module, id, size int32, extend bool) (ptr_t, _ErrorCode) {
	// Ensure size is a multiple of the OS page size.
	if int(size)&(unix.Getpagesize()-1) != 0 {
		return 0, _IOERR_SHMMAP
	}

	if rc := s.shmOpen(); rc != _OK {
		return 0, rc
	}

	// Check if file is big enough.
	o, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, _IOERR_SHMSIZE
	}
	if n := (int64(id) + 1) * int64(size); n > o {
		if !extend {
			return 0, _OK
		}
		if osAllocate(s.File, n) != nil {
			return 0, _IOERR_SHMSIZE
		}
	}

	r, err := util.MapRegion(ctx, mod, s.File, int64(id)*int64(size), size, false)
	if err != nil {
		return 0, _IOERR_SHMMAP
	}
	s.regions = append(s.regions, r)
	return r.Ptr, _OK
}

func (s *vfsShm) shmLock(offset, n int32, flags _ShmFlag) _ErrorCode {
	s.Lock()
	defer s.Unlock()

	// Check if we can obtain/release locks locally.
	rc := s.shmMemLock(offset, n, flags)
	if rc != _OK {
		return rc
	}

	// Obtain/release the appropriate file locks.
	switch {
	case flags&_SHM_UNLOCK != 0:
		// Relasing a shared lock decrements the counter,
		// but may leave parts of the range still locked.
		begin, end := offset, offset+n
		for i := begin; i < end; i++ {
			if s.vfsShmParent.lock[i] != 0 {
				if i > begin {
					rc |= osUnlock(s.File, _SHM_BASE+int64(begin), int64(i-begin))
				}
				begin = i + 1
			}
		}
		if end > begin {
			rc |= osUnlock(s.File, _SHM_BASE+int64(begin), int64(end-begin))
		}
		return rc
	case flags&_SHM_SHARED != 0:
		// Acquiring a new shared lock on the file is only necessary
		// if there was a new shared lock in the range.
		for i := offset; i < offset+n; i++ {
			if s.vfsShmParent.lock[i] == 1 {
				rc = osReadLock(s.File, _SHM_BASE+int64(offset), int64(n))
				break
			}
		}
	case flags&_SHM_EXCLUSIVE != 0:
		// Acquiring an exclusive lock on the file is always necessary.
		rc = osWriteLock(s.File, _SHM_BASE+int64(offset), int64(n))
	default:
		panic(util.AssertErr())
	}

	// Release the local locks we had acquired.
	if rc != _OK {
		s.shmMemLock(offset, n, flags^(_SHM_UNLOCK|_SHM_LOCK))
	}
	return rc
}

func (s *vfsShm) shmUnmap(delete bool) {
	if s.vfsShmParent == nil {
		return
	}

	// Unmap regions.
	for _, r := range s.regions {
		r.Unmap()
	}
	s.regions = nil

	// Close the file.
	if delete {
		os.Remove(s.path)
	}
	s.Close()
}

func (s *vfsShm) shmBarrier() {
	s.Lock()
	//lint:ignore SA2001 memory barrier.
	s.Unlock()
}
