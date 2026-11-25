package plugins

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Extraction limits. An archive is untrusted input from a repository anyone
// can push to, so a plugin file and the whole download are capped at sizes far
// above any real Bash script.
const (
	maxEntrySize = 4 << 20  // 4 MiB per plugin script
	maxTotalSize = 64 << 20 // 64 MiB per archive
)

// ExtractTarGz unpacks a gzipped tar stream into dir and returns the size
// written. Repository archives wrap every entry in a single top-level
// directory, which is stripped so the caller sees a repository root.
//
// Links, devices, and any entry that would write outside dir are skipped: the
// archive comes from someone else's machine.
func ExtractTarGz(r io.Reader, dir string) (int64, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return 0, fmt.Errorf("failed to read the archive: %w", err)
	}
	defer zr.Close()

	var total int64
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, fmt.Errorf("failed to read the archive: %w", err)
		}

		name, ok := archivePath(hdr.Name)
		if !ok {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if hdr.Size > maxEntrySize {
			return total, fmt.Errorf("%s is %d bytes, over the %d byte limit for one plugin", name, hdr.Size, maxEntrySize)
		}
		if total+hdr.Size > maxTotalSize {
			return total, fmt.Errorf("the archive is over the %d byte limit", maxTotalSize)
		}

		local := filepath.FromSlash(name)
		if !filepath.IsLocal(local) {
			continue
		}
		target := filepath.Join(dir, local)
		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
			return total, err
		}

		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return total, fmt.Errorf("failed to write %s: %w", target, err)
		}
		n, err := io.Copy(f, io.LimitReader(tr, maxEntrySize+1))
		closeErr := f.Close()
		if err != nil {
			return total, fmt.Errorf("failed to write %s: %w", target, err)
		}
		if closeErr != nil {
			return total, closeErr
		}
		if n > maxEntrySize {
			return total, fmt.Errorf("%s is over the %d byte limit for one plugin", name, maxEntrySize)
		}
		total += n
	}
	return total, nil
}

// archivePath normalizes one tar entry name: forward slashes, no absolute
// path, no traversal, and the repository's top-level directory removed.
func archivePath(name string) (string, bool) {
	clean := path.Clean("/" + strings.TrimPrefix(filepath.ToSlash(name), "/"))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", false
	}
	// Drop "{repo}-{sha}/", which every GitHub archive starts with.
	if _, rest, ok := strings.Cut(clean, "/"); ok && rest != "" {
		clean = rest
	} else {
		return "", false
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	return clean, true
}
