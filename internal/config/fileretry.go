package config

import (
	"os"
	"time"
)

// transientRetryBudget bounds how long a transient failure is retried.
const transientRetryBudget = 2 * time.Second

// readFile is os.ReadFile, retried while Windows briefly locks the file
// (another writer's renameFile, antivirus, the search indexer). Read any
// file written with atomicWriteFile through it.
func readFile(path string) ([]byte, error) {
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
		if err == nil || !isTransientFileError(err) || slept >= transientRetryBudget {
			return err
		}
		time.Sleep(delay)
		slept += delay
		delay = min(delay*2, 50*time.Millisecond)
	}
}
