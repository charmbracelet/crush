package config

import (
	"os"

	"github.com/charmbracelet/crush/internal/fsext"
)

// atomicWriteFile writes data to a file atomically by writing to a unique
// temporary file in the same directory and renaming it into place. This
// prevents concurrent readers from observing a partially-written file.
//
// The implementation lives in fsext so that plugins, which install files that
// are executed at config load, share the same crash-safe writer.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return fsext.AtomicWriteFile(path, data, perm)
}
