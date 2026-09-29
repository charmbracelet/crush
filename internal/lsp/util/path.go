package util

import (
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// PathFromURI converts a LSP document URI to a filesystem path.
//
// It works around a path-normalization issue where converting a file URI back
// to a path can leave a leading path separator in front of a Windows drive
// letter (for example "\G:\foo" instead of "G:\foo"). Such paths are invalid
// on Windows, because a leading separator turns them into root-relative or
// UNC-style paths, which causes file reads and LSP refreshes to fail.
// See issue #3089.
func PathFromURI(uri protocol.DocumentURI) (string, error) {
	path, err := uri.Path()
	if err != nil {
		return "", err
	}
	return normalizeWindowsDrivePath(path), nil
}

// normalizeWindowsDrivePath strips a stray leading path separator that may
// appear in front of a Windows drive letter after a URI-to-path conversion
// (for example "\G:\foo" or "/G:/foo" becomes "G:\foo").
func normalizeWindowsDrivePath(path string) string {
	if len(path) >= 3 &&
		isPathSeparator(path[0]) &&
		isASCIILetter(path[1]) &&
		path[2] == ':' {
		return path[1:]
	}
	return path
}

func isPathSeparator(b byte) bool {
	return b == '/' || b == '\\'
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
