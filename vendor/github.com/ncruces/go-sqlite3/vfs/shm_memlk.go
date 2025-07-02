//go:build ((freebsd || openbsd || netbsd || dragonfly || illumos) && (386 || arm || amd64 || arm64 || riscv64 || ppc64le)) || sqlite3_flock || sqlite3_dotlk

package vfs

import "github.com/ncruces/go-sqlite3/internal/util"

// +checklocks:s.Mutex
func (s *vfsShm) shmMemLock(offset, n int32, flags _ShmFlag) _ErrorCode {
	switch {
	case flags&_SHM_UNLOCK != 0:
		for i := offset; i < offset+n; i++ {
			if s.lock[i] {
				if s.vfsShmParent.lock[i] <= 0 {
					s.vfsShmParent.lock[i] = 0
				} else {
					s.vfsShmParent.lock[i]--
				}
				s.lock[i] = false
			}
		}
	case flags&_SHM_SHARED != 0:
		for i := offset; i < offset+n; i++ {
			if !s.lock[i] &&
				s.vfsShmParent.lock[i]+1 <= 0 {
				return _BUSY
			}
		}
		for i := offset; i < offset+n; i++ {
			if !s.lock[i] {
				s.vfsShmParent.lock[i]++
				s.lock[i] = true
			}
		}
	case flags&_SHM_EXCLUSIVE != 0:
		for i := offset; i < offset+n; i++ {
			if s.lock[i] {
				// SQLite never requests an exclusive lock that it already holds.
				panic(util.AssertErr())
			}
			if s.vfsShmParent.lock[i] != 0 {
				return _BUSY
			}
		}
		for i := offset; i < offset+n; i++ {
			s.vfsShmParent.lock[i] = -1
			s.lock[i] = true
		}
	default:
		panic(util.AssertErr())
	}

	return _OK
}
