//go:build !unix

package db

import (
	"os"
)

// tryExclusiveLock is a no-op on non-Unix platforms. Windows uses
// mandatory locking which SQLite handles natively.
func tryExclusiveLock(_ string) (*os.File, error) {
	return nil, nil
}
