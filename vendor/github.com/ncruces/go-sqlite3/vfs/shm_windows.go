//go:build (386 || arm || amd64 || arm64 || riscv64 || ppc64le) && !sqlite3_dotlk

package vfs

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"golang.org/x/sys/windows"

	"github.com/ncruces/go-sqlite3/internal/util"
)

type vfsShm struct {
	*os.File
	mod      api.Module
	alloc    api.Function
	free     api.Function
	path     string
	regions  []*util.MappedRegion
	shared   [][]byte
	shadow   [][_WALINDEX_PGSZ]byte
	ptrs     []ptr_t
	stack    [1]stk_t
	fileLock bool
	blocking bool
	sync.Mutex
}

func (s *vfsShm) Close() error {
	// Unmap regions.
	for _, r := range s.regions {
		r.Unmap()
	}
	s.regions = nil

	// Close the file.
	return s.File.Close()
}

func (s *vfsShm) shmOpen() _ErrorCode {
	if s.File == nil {
		f, err := os.OpenFile(s.path, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			return _CANTOPEN
		}
		s.File = f
	}
	if s.fileLock {
		return _OK
	}

	// Dead man's switch.
	if rc := osWriteLock(s.File, _SHM_DMS, 1, 0); rc == _OK {
		err := s.Truncate(0)
		osUnlock(s.File, _SHM_DMS, 1)
		if err != nil {
			return _IOERR_SHMOPEN
		}
	}
	rc := osReadLock(s.File, _SHM_DMS, 1, 0)
	s.fileLock = rc == _OK
	return rc
}

func (s *vfsShm) shmMap(ctx context.Context, mod api.Module, id, size int32, extend bool) (_ ptr_t, rc _ErrorCode) {
	// Ensure size is a multiple of the OS page size.
	if size != _WALINDEX_PGSZ || (windows.Getpagesize()-1)&_WALINDEX_PGSZ != 0 {
		return 0, _IOERR_SHMMAP
	}
	if s.mod == nil {
		s.mod = mod
		s.free = mod.ExportedFunction("sqlite3_free")
		s.alloc = mod.ExportedFunction("sqlite3_malloc64")
	}
	if rc := s.shmOpen(); rc != _OK {
		return 0, rc
	}

	defer s.shmAcquire(&rc)

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

	// Maps regions into memory.
	for int(id) >= len(s.shared) {
		r, err := util.MapRegion(ctx, mod, s.File, int64(id)*int64(size), size)
		if err != nil {
			return 0, _IOERR_SHMMAP
		}
		s.regions = append(s.regions, r)
		s.shared = append(s.shared, r.Data)
	}

	// Allocate shadow memory.
	if int(id) >= len(s.shadow) {
		s.shadow = append(s.shadow, make([][_WALINDEX_PGSZ]byte, int(id)-len(s.shadow)+1)...)
	}

	// Allocate local memory.
	for int(id) >= len(s.ptrs) {
		s.stack[0] = stk_t(size)
		if err := s.alloc.CallWithStack(ctx, s.stack[:]); err != nil {
			panic(err)
		}
		if s.stack[0] == 0 {
			panic(util.OOMErr)
		}
		clear(util.View(s.mod, ptr_t(s.stack[0]), _WALINDEX_PGSZ))
		s.ptrs = append(s.ptrs, ptr_t(s.stack[0]))
	}

	s.shadow[0][4] = 1
	return s.ptrs[id], _OK
}

func (s *vfsShm) shmLock(offset, n int32, flags _ShmFlag) (rc _ErrorCode) {
	switch {
	case flags&_SHM_LOCK != 0:
		defer s.shmAcquire(&rc)
	case flags&_SHM_EXCLUSIVE != 0:
		s.shmRelease()
	}

	switch {
	case flags&_SHM_UNLOCK != 0:
		return osUnlock(s.File, _SHM_BASE+uint32(offset), uint32(n))
	case flags&_SHM_SHARED != 0:
		return osReadLock(s.File, _SHM_BASE+uint32(offset), uint32(n), 0)
	case flags&_SHM_EXCLUSIVE != 0:
		return osWriteLock(s.File, _SHM_BASE+uint32(offset), uint32(n), 0)
	default:
		panic(util.AssertErr())
	}
}

func (s *vfsShm) shmUnmap(delete bool) {
	if s.File == nil {
		return
	}

	s.shmRelease()

	// Free local memory.
	for _, p := range s.ptrs {
		s.stack[0] = stk_t(p)
		if err := s.free.CallWithStack(context.Background(), s.stack[:]); err != nil {
			panic(err)
		}
	}
	s.ptrs = nil
	s.shadow = nil
	s.shared = nil

	// Close the file.
	s.Close()
	s.File = nil
	if delete {
		os.Remove(s.path)
	}
}
