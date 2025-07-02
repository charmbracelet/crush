//go:build !darwin && !(aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris)

package fastwalk

import "os"

// readDir calls fn for each directory entry in dirName.
// It does not descend into directories or follow symlinks.
// If fn returns a non-nil error, readDir returns with that error
// immediately.
func (w *walker) readDir(dirName string, depth int) error {
	f, err := os.Open(dirName)
	if err != nil {
		return err
	}
	des, readErr := f.ReadDir(-1)
	f.Close()
	if readErr != nil && len(des) == 0 {
		return readErr
	}

	var p *[]DirEntry
	if w.sortMode != SortNone {
		p = direntSlicePool.Get().(*[]DirEntry)
	}
	defer putDirentSlice(p)

	var skipFiles bool
	for _, d := range des {
		if skipFiles && d.Type().IsRegular() {
			continue
		}
		// Need to use FileMode.Type().Type() for fs.DirEntry
		e := newDirEntry(dirName, d, depth)
		if w.sortMode == SortNone {
			if err := w.onDirEnt(dirName, d.Name(), e); err != nil {
				if err != ErrSkipFiles {
					return err
				}
				skipFiles = true
			}
		} else {
			*p = append(*p, e)
		}
	}
	if w.sortMode == SortNone {
		return readErr
	}

	dents := *p
	sortDirents(w.sortMode, dents)
	for _, d := range dents {
		d := d
		if skipFiles && d.Type().IsRegular() {
			continue
		}
		if err := w.onDirEnt(dirName, d.Name(), d); err != nil {
			if err != ErrSkipFiles {
				return err
			}
			skipFiles = true
		}
	}
	return readErr
}
