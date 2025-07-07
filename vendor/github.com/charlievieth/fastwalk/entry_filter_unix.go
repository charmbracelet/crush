//go:build darwin || aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package fastwalk

import (
	"io/fs"
	"sync"
	"syscall"
)

type fileKey struct {
	Dev uint64
	Ino uint64
}

type entryMap struct {
	mu   sync.Mutex
	keys map[fileKey]struct{}
}

// An EntryFilter keeps track of visited directory entries and can be used to
// detect and avoid symlink loops or processing the same file twice.
type EntryFilter struct {
	// Use an array of 8 to reduce lock contention. The entryMap is
	// picked via the inode number. We don't take the device number
	// into account because: we don't expect to see many of them and
	// uniformly spreading the load isn't terribly beneficial here.
	ents [8]entryMap
}

// NewEntryFilter returns a new EntryFilter
func NewEntryFilter() *EntryFilter {
	return new(EntryFilter)
}

func (e *EntryFilter) seen(dev, ino uint64) (seen bool) {
	m := &e.ents[ino%uint64(len(e.ents))]
	m.mu.Lock()
	if _, seen = m.keys[fileKey{dev, ino}]; !seen {
		if m.keys == nil {
			m.keys = make(map[fileKey]struct{})
		}
		m.keys[fileKey{dev, ino}] = struct{}{}
	}
	m.mu.Unlock()
	return seen
}

// TODO: this name is confusing and should be fixed

// Entry returns if path and [fs.DirEntry] have been seen before.
func (e *EntryFilter) Entry(path string, de fs.DirEntry) (seen bool) {
	fi, err := StatDirEntry(path, de)
	if err != nil {
		return true // treat errors as duplicate files
	}
	stat := fi.Sys().(*syscall.Stat_t)
	return e.seen(uint64(stat.Dev), uint64(stat.Ino))
}
