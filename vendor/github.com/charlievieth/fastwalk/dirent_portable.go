//go:build !darwin && !(aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris)

// TODO: add a "portable_dirent" build tag so that we can test this
// on non-Windows platforms

package fastwalk

import (
	"io/fs"
	"os"
	"sort"
	"sync"

	"github.com/charlievieth/fastwalk/internal/fmtdirent"
)

var _ DirEntry = (*portableDirent)(nil)

type portableDirent struct {
	fs.DirEntry
	parent string
	stat   *fileInfo
	depth  uint32
}

func (d *portableDirent) String() string {
	return fmtdirent.FormatDirEntry(d)
}

func (d *portableDirent) Depth() int {
	return int(d.depth)
}

func (d *portableDirent) Stat() (fs.FileInfo, error) {
	if d.DirEntry.Type()&os.ModeSymlink == 0 {
		return d.DirEntry.Info()
	}
	stat := loadFileInfo(&d.stat)
	stat.once.Do(func() {
		stat.FileInfo, stat.err = os.Stat(d.parent + string(os.PathSeparator) + d.Name())
	})
	return stat.FileInfo, stat.err
}

func newDirEntry(dirName string, info fs.DirEntry, depth int) DirEntry {
	return &portableDirent{
		DirEntry: info,
		parent:   dirName,
		depth:    uint32(depth),
	}
}

func fileInfoToDirEntry(dirname string, fi fs.FileInfo) DirEntry {
	return newDirEntry(dirname, fs.FileInfoToDirEntry(fi), 0)
}

var direntSlicePool = sync.Pool{
	New: func() any {
		a := make([]DirEntry, 0, 32)
		return &a
	},
}

func putDirentSlice(p *[]DirEntry) {
	// max is half as many as Unix because twice the size
	if p != nil && cap(*p) <= 16*1024 {
		a := *p
		for i := range a {
			a[i] = nil
		}
		*p = a[:0]
		direntSlicePool.Put(p)
	}
}

func sortDirents(mode SortMode, dents []DirEntry) {
	if len(dents) <= 1 {
		return
	}
	switch mode {
	case SortLexical:
		sort.Slice(dents, func(i, j int) bool {
			return dents[i].Name() < dents[j].Name()
		})
	case SortFilesFirst:
		sort.Slice(dents, func(i, j int) bool {
			d1 := dents[i]
			d2 := dents[j]
			r1 := d1.Type().IsRegular()
			r2 := d2.Type().IsRegular()
			switch {
			case r1 && !r2:
				return true
			case !r1 && r2:
				return false
			case !r1 && !r2:
				// Both are not regular files: sort directories last
				dd1 := d1.Type().IsDir()
				dd2 := d2.Type().IsDir()
				switch {
				case !dd1 && dd2:
					return true
				case dd1 && !dd2:
					return false
				}
			}
			return d1.Name() < d2.Name()
		})
	case SortDirsFirst:
		sort.Slice(dents, func(i, j int) bool {
			d1 := dents[i]
			d2 := dents[j]
			dd1 := d1.Type().IsDir()
			dd2 := d2.Type().IsDir()
			switch {
			case dd1 && !dd2:
				return true
			case !dd1 && dd2:
				return false
			case !dd1 && !dd2:
				// Both are not directories: sort regular files first
				r1 := d1.Type().IsRegular()
				r2 := d2.Type().IsRegular()
				switch {
				case r1 && !r2:
					return true
				case !r1 && r2:
					return false
				}
			}
			return d1.Name() < d2.Name()
		})
	}
}
