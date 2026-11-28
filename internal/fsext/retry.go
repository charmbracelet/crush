package fsext

import (
	"os"
	"time"
)

// transientRetryBudget bounds how long a transient failure is retried.
const transientRetryBudget = 2 * time.Second

// Rename is os.Rename, retried while Windows briefly locks the destination
// (antivirus, the search indexer, a concurrent reader).
func Rename(oldpath, newpath string) error {
	return retryTransient(func() error { return os.Rename(oldpath, newpath) })
}

// ReadFile is os.ReadFile, retried while Windows briefly locks the file, such
// as during another writer's Rename.
func ReadFile(path string) ([]byte, error) {
	var data []byte
	err := retryTransient(func() (err error) {
		data, err = os.ReadFile(path)
		return err
	})
	return data, err
}

// retryTransient runs op with backoff until it succeeds, fails with a
// non-transient error, or transientRetryBudget is spent.
func retryTransient(op func() error) error {
	var slept time.Duration
	delay := time.Millisecond
	for {
		err := op()
		if err == nil || !isTransientError(err) || slept >= transientRetryBudget {
			return err
		}
		time.Sleep(delay)
		slept += delay
		delay = min(delay*2, 50*time.Millisecond)
	}
}
