//go:build !windows
// +build !windows

package uv

import (
	"io"

	"github.com/muesli/cancelreader"
)

func newCancelreader(r io.Reader) (cancelreader.CancelReader, error) {
	return cancelreader.NewReader(r) //nolint:wrapcheck
}
