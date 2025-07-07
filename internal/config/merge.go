package config

import (
	"bytes"
	"io"

	"github.com/qjebbs/go-jsons"
)

func Merge(data []io.Reader) (io.Reader, error) {
	got, err := jsons.Merge(data)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(got), nil
}
