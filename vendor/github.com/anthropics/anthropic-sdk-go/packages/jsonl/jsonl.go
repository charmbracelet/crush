// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package jsonl

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
)

type Stream[T any] struct {
	rc  io.ReadCloser
	scn *bufio.Scanner
	cur T
	err error
}

func NewStream[T any](res *http.Response, err error) *Stream[T] {
	if res == nil || res.Body == nil {
		return nil
	}

	return &Stream[T]{
		rc:  res.Body,
		scn: bufio.NewScanner(res.Body),
		err: err,
	}
}

func (s *Stream[T]) Next() bool {
	if s.err != nil {
		return false
	}

	if !s.scn.Scan() {
		return false
	}

	line := s.scn.Bytes()
	var nxt T
	s.err = json.Unmarshal(line, &nxt)
	s.cur = nxt
	return s.err == nil
}

func (s *Stream[T]) Current() T {
	return s.cur
}

func (s *Stream[T]) Err() error {
	return s.err
}

func (s *Stream[T]) Close() error {
	return s.rc.Close()
}
