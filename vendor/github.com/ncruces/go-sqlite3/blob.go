package sqlite3

import (
	"io"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// ZeroBlob represents a zero-filled, length n BLOB
// that can be used as an argument to
// [database/sql.DB.Exec] and similar methods.
type ZeroBlob int64

// Blob is an handle to an open BLOB.
//
// It implements [io.ReadWriteSeeker] for incremental BLOB I/O.
//
// https://sqlite.org/c3ref/blob.html
type Blob struct {
	c      *Conn
	bytes  int64
	offset int64
	handle ptr_t
	bufptr ptr_t
	buflen int64
}

var _ io.ReadWriteSeeker = &Blob{}

// OpenBlob opens a BLOB for incremental I/O.
//
// https://sqlite.org/c3ref/blob_open.html
func (c *Conn) OpenBlob(db, table, column string, row int64, write bool) (*Blob, error) {
	if c.interrupt.Err() != nil {
		return nil, INTERRUPT
	}

	defer c.arena.mark()()
	blobPtr := c.arena.new(ptrlen)
	dbPtr := c.arena.string(db)
	tablePtr := c.arena.string(table)
	columnPtr := c.arena.string(column)

	var flags int32
	if write {
		flags = 1
	}

	rc := res_t(c.call("sqlite3_blob_open", stk_t(c.handle),
		stk_t(dbPtr), stk_t(tablePtr), stk_t(columnPtr),
		stk_t(row), stk_t(flags), stk_t(blobPtr)))

	if err := c.error(rc); err != nil {
		return nil, err
	}

	blob := Blob{c: c}
	blob.handle = util.Read32[ptr_t](c.mod, blobPtr)
	blob.bytes = int64(int32(c.call("sqlite3_blob_bytes", stk_t(blob.handle))))
	return &blob, nil
}

// Close closes a BLOB handle.
//
// It is safe to close a nil, zero or closed Blob.
//
// https://sqlite.org/c3ref/blob_close.html
func (b *Blob) Close() error {
	if b == nil || b.handle == 0 {
		return nil
	}

	rc := res_t(b.c.call("sqlite3_blob_close", stk_t(b.handle)))
	b.c.free(b.bufptr)
	b.handle = 0
	return b.c.error(rc)
}

// Size returns the size of the BLOB in bytes.
//
// https://sqlite.org/c3ref/blob_bytes.html
func (b *Blob) Size() int64 {
	return b.bytes
}

// Read implements the [io.Reader] interface.
//
// https://sqlite.org/c3ref/blob_read.html
func (b *Blob) Read(p []byte) (n int, err error) {
	if b.offset >= b.bytes {
		return 0, io.EOF
	}

	want := int64(len(p))
	avail := b.bytes - b.offset
	if want > avail {
		want = avail
	}
	if want > b.buflen {
		b.bufptr = b.c.realloc(b.bufptr, want)
		b.buflen = want
	}

	rc := res_t(b.c.call("sqlite3_blob_read", stk_t(b.handle),
		stk_t(b.bufptr), stk_t(want), stk_t(b.offset)))
	err = b.c.error(rc)
	if err != nil {
		return 0, err
	}
	b.offset += want
	if b.offset >= b.bytes {
		err = io.EOF
	}

	copy(p, util.View(b.c.mod, b.bufptr, want))
	return int(want), err
}

// WriteTo implements the [io.WriterTo] interface.
//
// https://sqlite.org/c3ref/blob_read.html
func (b *Blob) WriteTo(w io.Writer) (n int64, err error) {
	if b.offset >= b.bytes {
		return 0, nil
	}

	want := int64(1024 * 1024)
	avail := b.bytes - b.offset
	if want > avail {
		want = avail
	}
	if want > b.buflen {
		b.bufptr = b.c.realloc(b.bufptr, want)
		b.buflen = want
	}

	for want > 0 {
		rc := res_t(b.c.call("sqlite3_blob_read", stk_t(b.handle),
			stk_t(b.bufptr), stk_t(want), stk_t(b.offset)))
		err = b.c.error(rc)
		if err != nil {
			return n, err
		}

		mem := util.View(b.c.mod, b.bufptr, want)
		m, err := w.Write(mem[:want])
		b.offset += int64(m)
		n += int64(m)
		if err != nil {
			return n, err
		}
		if int64(m) != want {
			// notest // Write misbehaving
			return n, io.ErrShortWrite
		}

		avail = b.bytes - b.offset
		if want > avail {
			want = avail
		}
	}
	return n, nil
}

// Write implements the [io.Writer] interface.
//
// https://sqlite.org/c3ref/blob_write.html
func (b *Blob) Write(p []byte) (n int, err error) {
	want := int64(len(p))
	if want > b.buflen {
		b.bufptr = b.c.realloc(b.bufptr, want)
		b.buflen = want
	}
	util.WriteBytes(b.c.mod, b.bufptr, p)

	rc := res_t(b.c.call("sqlite3_blob_write", stk_t(b.handle),
		stk_t(b.bufptr), stk_t(want), stk_t(b.offset)))
	err = b.c.error(rc)
	if err != nil {
		return 0, err
	}
	b.offset += int64(len(p))
	return len(p), nil
}

// ReadFrom implements the [io.ReaderFrom] interface.
//
// https://sqlite.org/c3ref/blob_write.html
func (b *Blob) ReadFrom(r io.Reader) (n int64, err error) {
	want := int64(1024 * 1024)
	avail := b.bytes - b.offset
	if l, ok := r.(*io.LimitedReader); ok && want > l.N {
		want = l.N
	}
	if want > avail {
		want = avail
	}
	if want < 1 {
		want = 1
	}
	if want > b.buflen {
		b.bufptr = b.c.realloc(b.bufptr, want)
		b.buflen = want
	}

	for {
		mem := util.View(b.c.mod, b.bufptr, want)
		m, err := r.Read(mem[:want])
		if m > 0 {
			rc := res_t(b.c.call("sqlite3_blob_write", stk_t(b.handle),
				stk_t(b.bufptr), stk_t(m), stk_t(b.offset)))
			err := b.c.error(rc)
			if err != nil {
				return n, err
			}
			b.offset += int64(m)
			n += int64(m)
		}
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}

		avail = b.bytes - b.offset
		if want > avail {
			want = avail
		}
		if want < 1 {
			want = 1
		}
	}
}

// Seek implements the [io.Seeker] interface.
func (b *Blob) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, util.WhenceErr
	case io.SeekStart:
		break
	case io.SeekCurrent:
		offset += b.offset
	case io.SeekEnd:
		offset += b.bytes
	}
	if offset < 0 {
		return 0, util.OffsetErr
	}
	b.offset = offset
	return offset, nil
}

// Reopen moves a BLOB handle to a new row of the same database table.
//
// https://sqlite.org/c3ref/blob_reopen.html
func (b *Blob) Reopen(row int64) error {
	if b.c.interrupt.Err() != nil {
		return INTERRUPT
	}
	err := b.c.error(res_t(b.c.call("sqlite3_blob_reopen", stk_t(b.handle), stk_t(row))))
	b.bytes = int64(int32(b.c.call("sqlite3_blob_bytes", stk_t(b.handle))))
	b.offset = 0
	return err
}
