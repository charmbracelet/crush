package fastwalk

import (
	"io/fs"
	"os"
	"path/filepath"
)

func isDir(path string, d fs.DirEntry) bool {
	if d.IsDir() {
		return true
	}
	if d.Type()&os.ModeSymlink != 0 {
		if fi, err := StatDirEntry(path, d); err == nil {
			return fi.IsDir()
		}
	}
	return false
}

// IgnoreDuplicateDirs wraps [fs.WalkDirFunc] walkFn to make it follow symbolic
// links and ignore duplicate directories (if a symlink points to a directory
// that has already been traversed it is skipped). The walkFn is called for
// for skipped directories, but the directory is not traversed (this is
// required for error handling).
//
// The Follow [Config] setting has no effect on the behavior of Walk when
// this wrapper is used.
//
// In most use cases, the returned [fs.WalkDirFunc] should not be reused.
// If it is reused, any previously visited file will be skipped.
//
// NOTE: The order of traversal is undefined. Given an "example" directory
// like the one below where "dir" is a directory and "smydir1" and "smydir2"
// are links to it, only one of "dir", "smydir1", or "smydir2" will be
// traversed, but which one is undefined.
//
//	example
//	├── dir
//	├── smydir1 -> dir
//	└── smydir2 -> dir
func IgnoreDuplicateDirs(walkFn fs.WalkDirFunc) fs.WalkDirFunc {
	filter := NewEntryFilter()
	return func(path string, d fs.DirEntry, err error) error {
		// Call walkFn before checking the entry filter so that we
		// don't record directories that are skipped with SkipDir.
		err = walkFn(path, d, err)
		if err != nil {
			if err != filepath.SkipDir && isDir(path, d) {
				filter.Entry(path, d)
			}
			return err
		}
		if isDir(path, d) {
			if filter.Entry(path, d) {
				return filepath.SkipDir
			}
			if d.Type() == os.ModeSymlink {
				return ErrTraverseLink
			}
		}
		return nil
	}
}

// IgnoreDuplicateFiles wraps walkFn so that symlinks are followed and duplicate
// files are ignored. If a symlink resolves to a file that has already been
// visited it will be skipped.
//
// In most use cases, the returned [fs.WalkDirFunc] should not be reused.
// If it is reused, any previously visited file will be skipped.
//
// This can significantly slow Walk as os.Stat() is called for each path
// (on Windows, os.Stat() is only needed for symlinks).
func IgnoreDuplicateFiles(walkFn fs.WalkDirFunc) fs.WalkDirFunc {
	filter := NewEntryFilter()
	return func(path string, d fs.DirEntry, err error) error {
		// Skip all duplicate files, directories, and links
		if filter.Entry(path, d) {
			if isDir(path, d) {
				return filepath.SkipDir
			}
			return nil
		}
		err = walkFn(path, d, err)
		if err == nil && d.Type() == os.ModeSymlink && isDir(path, d) {
			err = ErrTraverseLink
		}
		return err
	}
}

// IgnorePermissionErrors wraps walkFn so that [fs.ErrPermission] permission
// errors are ignored. The returned [fs.WalkDirFunc] may be reused.
func IgnorePermissionErrors(walkFn fs.WalkDirFunc) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil && os.IsPermission(err) {
			return nil
		}
		return walkFn(path, d, err)
	}
}
