//go:build windows

package dirid

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

// identity is a volume serial number and file ID. os.SameFile is not used
// on Windows because it compares only the 64-bit file index, which ReFS
// (Dev Drive) sets to the same invalid value for many files.
type identity struct {
	// ext reports whether the values came from FILE_ID_INFO (64-bit
	// serial, 128-bit ID) rather than BY_HANDLE_FILE_INFORMATION (32-bit
	// serial, 64-bit index), so the two forms are never compared.
	ext    bool
	volume uint64
	file   [16]byte
}

// fileIDInfo mirrors FILE_ID_INFO.
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

func of(path string) (identity, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return identity{}, false
	}
	// FILE_FLAG_BACKUP_SEMANTICS is required to open a directory. Without
	// FILE_FLAG_OPEN_REPARSE_POINT, junctions and symlinks are followed.
	h, err := windows.CreateFile(
		p,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return identity{}, false
	}
	defer windows.CloseHandle(h) //nolint:errcheck

	var ex fileIDInfo
	err = windows.GetFileInformationByHandleEx(
		h,
		windows.FileIdInfo,
		(*byte)(unsafe.Pointer(&ex)),
		uint32(unsafe.Sizeof(ex)),
	)
	if err == nil {
		if !validFileID(ex.FileID) {
			return identity{}, false
		}
		return identity{
			ext:    true,
			volume: ex.VolumeSerialNumber,
			file:   ex.FileID,
		}, true
	}

	// FILE_ID_INFO is not available on every filesystem; fall back to the
	// 64-bit index, rejecting the values that do not identify a file.
	var bh windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &bh); err != nil {
		return identity{}, false
	}
	idx := uint64(bh.FileIndexHigh)<<32 | uint64(bh.FileIndexLow)
	if !validFileIndex(idx) {
		return identity{}, false
	}
	id := identity{volume: uint64(bh.VolumeSerialNumber)}
	binary.LittleEndian.PutUint64(id.file[:8], idx)
	return id, true
}

func same(a, b identity) bool {
	return a == b
}
