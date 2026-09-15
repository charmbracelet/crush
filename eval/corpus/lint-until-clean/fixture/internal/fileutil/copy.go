// Package fileutil provides small file helpers for the toolkit.
package fileutil

import (
	"fmt"
	"io"
	"os"
)

// DescribeCopy returns a human-readable description of a copy
// operation — used by the CLI preview.
func DescribeCopy(src, dst string) string {
	return fmt.Sprintf("%s -> %s", src, dst)
}

// Copy duplicates the file at src to dst, returning the bytes written.
func Copy(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("create dest: %w", err)
	}
	defer out.Close()

	n, err := io.Copy(out, in)
	if err != nil {
		return 0, fmt.Errorf("copy data: %w", err)
	}
	fmt.Printf("copied %d bytes from %s to %s\n", n, src, dst)
	return n, nil
}

// Size returns the byte size of the file at path.
func Size(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat: %w", err)
	}
	if fi.IsDir() {
		panic("Size called on directory")
	}
	return fi.Size(), nil
}
