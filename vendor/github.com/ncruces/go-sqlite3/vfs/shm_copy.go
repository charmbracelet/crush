//go:build (windows && (386 || arm || amd64 || arm64 || riscv64 || ppc64le)) || sqlite3_dotlk

package vfs

import (
	"unsafe"

	"github.com/ncruces/go-sqlite3/internal/util"
)

const (
	_WALINDEX_HDR_SIZE = 136
	_WALINDEX_PGSZ     = 32768
)

// This seems a safe way of keeping the WAL-index in sync.
//
// The WAL-index file starts with a header,
// and the index doesn't meaningfully change if the header doesn't change.
//
// The header starts with two 48 byte, checksummed, copies of the same information,
// which are accessed independently between memory barriers.
// The checkpoint information that follows uses 4 byte aligned words.
//
// Finally, we have the WAL-index hash tables,
// which are only modified holding the exclusive WAL_WRITE_LOCK.
//
// Since all the data is either redundant+checksummed,
// 4 byte aligned, or modified under an exclusive lock,
// the copies below should correctly keep memory in sync.
//
// https://sqlite.org/walformat.html#the_wal_index_file_format

func (s *vfsShm) shmAcquire(ptr *_ErrorCode) {
	if ptr != nil && *ptr != _OK {
		return
	}
	if len(s.ptrs) == 0 || shmEqual(s.shadow[0][:], s.shared[0][:]) {
		return
	}
	// Copies modified words from shared to private memory.
	for id, p := range s.ptrs {
		shared := shmPage(s.shared[id][:])
		shadow := shmPage(s.shadow[id][:])
		privat := shmPage(util.View(s.mod, p, _WALINDEX_PGSZ))
		for i, shared := range shared {
			if shadow[i] != shared {
				shadow[i] = shared
				privat[i] = shared
			}
		}
	}
}

func (s *vfsShm) shmRelease() {
	if len(s.ptrs) == 0 || shmEqual(s.shadow[0][:], util.View(s.mod, s.ptrs[0], _WALINDEX_HDR_SIZE)) {
		return
	}
	// Copies modified words from private to shared memory.
	for id, p := range s.ptrs {
		shared := shmPage(s.shared[id][:])
		shadow := shmPage(s.shadow[id][:])
		privat := shmPage(util.View(s.mod, p, _WALINDEX_PGSZ))
		for i, privat := range privat {
			if shadow[i] != privat {
				shadow[i] = privat
				shared[i] = privat
			}
		}
	}
}

func (s *vfsShm) shmBarrier() {
	s.Lock()
	s.shmAcquire(nil)
	s.shmRelease()
	s.Unlock()
}

func shmPage(s []byte) *[_WALINDEX_PGSZ / 4]uint32 {
	p := (*uint32)(unsafe.Pointer(unsafe.SliceData(s)))
	return (*[_WALINDEX_PGSZ / 4]uint32)(unsafe.Slice(p, _WALINDEX_PGSZ/4))
}

func shmEqual(v1, v2 []byte) bool {
	return *(*[_WALINDEX_HDR_SIZE]byte)(v1[:]) == *(*[_WALINDEX_HDR_SIZE]byte)(v2[:])
}
