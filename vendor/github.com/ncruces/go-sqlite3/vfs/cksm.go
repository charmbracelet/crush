package vfs

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"strconv"

	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
	"github.com/ncruces/go-sqlite3/util/sql3util"
)

func cksmWrapFile(name *Filename, flags OpenFlag, file File) File {
	// Checksum only main databases and WALs.
	if flags&(OPEN_MAIN_DB|OPEN_WAL) == 0 {
		return file
	}

	cksm := cksmFile{File: file}

	if flags&OPEN_WAL != 0 {
		main, _ := name.DatabaseFile().(cksmFile)
		cksm.cksmFlags = main.cksmFlags
	} else {
		cksm.cksmFlags = new(cksmFlags)
		cksm.isDB = true
	}

	return cksm
}

type cksmFile struct {
	File
	*cksmFlags
	isDB bool
}

type cksmFlags struct {
	computeCksm bool
	verifyCksm  bool
	inCkpt      bool
	pageSize    int
}

func (c cksmFile) ReadAt(p []byte, off int64) (n int, err error) {
	n, err = c.File.ReadAt(p, off)
	p = p[:n]

	if isHeader(c.isDB, p, off) {
		c.init((*[100]byte)(p))
	}

	// Verify checksums.
	if c.verifyCksm && !c.inCkpt && len(p) == c.pageSize {
		cksm1 := cksmCompute(p[:len(p)-8])
		cksm2 := *(*[8]byte)(p[len(p)-8:])
		if cksm1 != cksm2 {
			return 0, _IOERR_DATA
		}
	}
	return n, err
}

func (c cksmFile) WriteAt(p []byte, off int64) (n int, err error) {
	if isHeader(c.isDB, p, off) {
		c.init((*[100]byte)(p))
	}

	// Compute checksums.
	if c.computeCksm && !c.inCkpt && len(p) == c.pageSize {
		*(*[8]byte)(p[len(p)-8:]) = cksmCompute(p[:len(p)-8])
	}

	return c.File.WriteAt(p, off)
}

func (c cksmFile) Pragma(name string, value string) (string, error) {
	switch name {
	case "checksum_verification":
		b, ok := sql3util.ParseBool(value)
		if ok {
			c.verifyCksm = b && c.computeCksm
		}
		if !c.verifyCksm {
			return "0", nil
		}
		return "1", nil

	case "page_size":
		if c.computeCksm {
			// Do not allow page size changes on a checksum database.
			return strconv.Itoa(c.pageSize), nil
		}
	}
	return "", _NOTFOUND
}

func (c cksmFile) DeviceCharacteristics() DeviceCharacteristic {
	ret := c.File.DeviceCharacteristics()
	if c.verifyCksm {
		ret &^= IOCAP_SUBPAGE_READ
	}
	return ret
}

func (c cksmFile) fileControl(ctx context.Context, mod api.Module, op _FcntlOpcode, pArg ptr_t) _ErrorCode {
	switch op {
	case _FCNTL_CKPT_START:
		c.inCkpt = true
	case _FCNTL_CKPT_DONE:
		c.inCkpt = false
	case _FCNTL_PRAGMA:
		rc := vfsFileControlImpl(ctx, mod, c, op, pArg)
		if rc != _NOTFOUND {
			return rc
		}
	}
	return vfsFileControlImpl(ctx, mod, c.File, op, pArg)
}

func (f *cksmFlags) init(header *[100]byte) {
	f.pageSize = 256 * int(binary.LittleEndian.Uint16(header[16:18]))
	if r := header[20] == 8; r != f.computeCksm {
		f.computeCksm = r
		f.verifyCksm = r
	}
	if !sql3util.ValidPageSize(f.pageSize) {
		f.computeCksm = false
		f.verifyCksm = false
	}
}

func isHeader(isDB bool, p []byte, off int64) bool {
	check := sql3util.ValidPageSize(len(p))
	if isDB {
		check = off == 0 && len(p) >= 100
	}
	return check && bytes.HasPrefix(p, []byte("SQLite format 3\000"))
}

func cksmCompute(a []byte) (cksm [8]byte) {
	var s1, s2 uint32
	for len(a) >= 8 {
		s1 += binary.LittleEndian.Uint32(a[0:4]) + s2
		s2 += binary.LittleEndian.Uint32(a[4:8]) + s1
		a = a[8:]
	}
	if len(a) != 0 {
		panic(util.AssertErr())
	}
	binary.LittleEndian.PutUint32(cksm[0:4], s1)
	binary.LittleEndian.PutUint32(cksm[4:8], s2)
	return
}

func (c cksmFile) SharedMemory() SharedMemory {
	if f, ok := c.File.(FileSharedMemory); ok {
		return f.SharedMemory()
	}
	return nil
}

func (c cksmFile) Unwrap() File {
	return c.File
}
