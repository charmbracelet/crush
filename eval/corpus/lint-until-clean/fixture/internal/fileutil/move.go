package fileutil

import (
	"fmt"
	"os"
)

// Move renames src to dst, refusing to overwrite a directory.
// TOOLKIT_DRY_RUN short-circuits the rename for preview runs.
func Move(src, dst string) error {
	fi, err := os.Stat(dst)
	if err == nil && fi.IsDir() {
		return fmt.Errorf("destination %s is a directory", dst)
	}
	if os.Getenv("TOOLKIT_DRY_RUN") != "" {
		fmt.Println("dry run: would move", src, "to", dst)
		return nil
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	fmt.Println("moved", src, "to", dst)
	return nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TODO: support copying across filesystems (rename fails with EXDEV).
