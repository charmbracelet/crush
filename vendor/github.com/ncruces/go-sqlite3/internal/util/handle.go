package util

import (
	"context"
	"io"
)

type handleState struct {
	handles []any
	holes   int
}

func (s *handleState) CloseNotify(ctx context.Context, exitCode uint32) {
	for _, h := range s.handles {
		if c, ok := h.(io.Closer); ok {
			c.Close()
		}
	}
	s.handles = nil
	s.holes = 0
}

func GetHandle(ctx context.Context, id Ptr_t) any {
	if id == 0 {
		return nil
	}
	s := ctx.Value(moduleKey{}).(*moduleState)
	return s.handles[^id]
}

func DelHandle(ctx context.Context, id Ptr_t) error {
	if id == 0 {
		return nil
	}
	s := ctx.Value(moduleKey{}).(*moduleState)
	a := s.handles[^id]
	s.handles[^id] = nil
	if l := Ptr_t(len(s.handles)); l == ^id {
		s.handles = s.handles[:l-1]
	} else {
		s.holes++
	}
	if c, ok := a.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func AddHandle(ctx context.Context, a any) Ptr_t {
	if a == nil {
		panic(NilErr)
	}

	s := ctx.Value(moduleKey{}).(*moduleState)

	// Find an empty slot.
	if s.holes > cap(s.handles)-len(s.handles) {
		for id, h := range s.handles {
			if h == nil {
				s.holes--
				s.handles[id] = a
				return ^Ptr_t(id)
			}
		}
	}

	// Add a new slot.
	s.handles = append(s.handles, a)
	return -Ptr_t(len(s.handles))
}
