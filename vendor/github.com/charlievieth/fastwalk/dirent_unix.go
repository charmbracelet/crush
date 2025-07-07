//go:build darwin || aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package fastwalk

import (
	"io/fs"
	"os"
	"sort"
	"sync"

	"github.com/charlievieth/fastwalk/internal/fmtdirent"
)

type unixDirent struct {
	parent string
	name   string
	typ    fs.FileMode
	depth  uint32 // uint32 so that we can pack it next to typ
	info   *fileInfo
	stat   *fileInfo
}

func (d *unixDirent) Name() string      { return d.name }
func (d *unixDirent) IsDir() bool       { return d.typ.IsDir() }
func (d *unixDirent) Type() fs.FileMode { return d.typ }
func (d *unixDirent) Depth() int        { return int(d.depth) }
func (d *unixDirent) String() string    { return fmtdirent.FormatDirEntry(d) }

func (d *unixDirent) Info() (fs.FileInfo, error) {
	info := loadFileInfo(&d.info)
	info.once.Do(func() {
		info.FileInfo, info.err = os.Lstat(d.parent + "/" + d.name)
	})
	return info.FileInfo, info.err
}

func (d *unixDirent) Stat() (fs.FileInfo, error) {
	if d.typ&os.ModeSymlink == 0 {
		return d.Info()
	}
	stat := loadFileInfo(&d.stat)
	stat.once.Do(func() {
		stat.FileInfo, stat.err = os.Stat(d.parent + "/" + d.name)
	})
	return stat.FileInfo, stat.err
}

func newUnixDirent(parent, name string, typ fs.FileMode, depth int) *unixDirent {
	return &unixDirent{
		parent: parent,
		name:   name,
		typ:    typ,
		depth:  uint32(depth),
	}
}

func fileInfoToDirEntry(dirname string, fi fs.FileInfo) DirEntry {
	info := &fileInfo{
		FileInfo: fi,
	}
	info.once.Do(func() {})
	return &unixDirent{
		parent: dirname,
		name:   fi.Name(),
		typ:    fi.Mode().Type(),
		info:   info,
	}
}

var direntSlicePool = sync.Pool{
	New: func() any {
		a := make([]*unixDirent, 0, 32)
		return &a
	},
}

func putDirentSlice(p *[]*unixDirent) {
	if p != nil && cap(*p) <= 32*1024 /* 256Kb */ {
		a := *p
		for i := range a {
			a[i] = nil
		}
		*p = a[:0]
		direntSlicePool.Put(p)
	}
}

func sortDirents(mode SortMode, dents []*unixDirent) {
	if len(dents) <= 1 {
		return
	}
	switch mode {
	case SortLexical:
		sort.Slice(dents, func(i, j int) bool {
			return dents[i].name < dents[j].name
		})
	case SortFilesFirst:
		sort.Slice(dents, func(i, j int) bool {
			d1 := dents[i]
			d2 := dents[j]
			r1 := d1.typ.IsRegular()
			r2 := d2.typ.IsRegular()
			switch {
			case r1 && !r2:
				return true
			case !r1 && r2:
				return false
			case !r1 && !r2:
				// Both are not regular files: sort directories last
				dd1 := d1.typ.IsDir()
				dd2 := d2.typ.IsDir()
				switch {
				case !dd1 && dd2:
					return true
				case dd1 && !dd2:
					return false
				}
			}
			return d1.name < d2.name
		})
	case SortDirsFirst:
		sort.Slice(dents, func(i, j int) bool {
			d1 := dents[i]
			d2 := dents[j]
			dd1 := d1.typ.IsDir()
			dd2 := d2.typ.IsDir()
			switch {
			case dd1 && !dd2:
				return true
			case !dd1 && dd2:
				return false
			case !dd1 && !dd2:
				// Both are not directories: sort regular files first
				r1 := d1.typ.IsRegular()
				r2 := d2.typ.IsRegular()
				switch {
				case r1 && !r2:
					return true
				case !r1 && r2:
					return false
				}
			}
			return d1.name < d2.name
		})
	}
}
