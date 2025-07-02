//go:build !unix && !windows

package alloc

import "github.com/tetratelabs/wazero/experimental"

func NewMemory(cap, max uint64) experimental.LinearMemory {
	return &sliceMemory{make([]byte, 0, cap)}
}

type sliceMemory struct {
	buf []byte
}

func (b *sliceMemory) Free() {}

func (b *sliceMemory) Reallocate(size uint64) []byte {
	if cap := uint64(cap(b.buf)); size > cap {
		b.buf = append(b.buf[:cap], make([]byte, size-cap)...)
	} else {
		b.buf = b.buf[:size]
	}
	return b.buf
}
