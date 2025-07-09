package util

import (
	"context"
	"os"
	"reflect"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	"golang.org/x/sys/windows"
)

type MappedRegion struct {
	windows.Handle
	Data []byte
	addr uintptr
}

func MapRegion(ctx context.Context, mod api.Module, f *os.File, offset int64, size int32) (*MappedRegion, error) {
	h, err := windows.CreateFileMapping(windows.Handle(f.Fd()), nil, windows.PAGE_READWRITE, 0, 0, nil)
	if h == 0 {
		return nil, err
	}

	a, err := windows.MapViewOfFile(h, windows.FILE_MAP_WRITE,
		uint32(offset>>32), uint32(offset), uintptr(size))
	if a == 0 {
		windows.CloseHandle(h)
		return nil, err
	}

	ret := &MappedRegion{Handle: h, addr: a}
	// SliceHeader, although deprecated, avoids a go vet warning.
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&ret.Data))
	sh.Len = int(size)
	sh.Cap = int(size)
	sh.Data = a
	return ret, nil
}

func (r *MappedRegion) Unmap() error {
	if r.Data == nil {
		return nil
	}
	err := windows.UnmapViewOfFile(r.addr)
	if err != nil {
		return err
	}
	r.Data = nil
	return windows.CloseHandle(r.Handle)
}
