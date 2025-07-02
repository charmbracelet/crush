package util

import (
	"runtime"
	"strconv"
)

type ErrorString string

func (e ErrorString) Error() string { return string(e) }

const (
	NilErr       = ErrorString("sqlite3: invalid memory address or null pointer dereference")
	OOMErr       = ErrorString("sqlite3: out of memory")
	RangeErr     = ErrorString("sqlite3: index out of range")
	NoNulErr     = ErrorString("sqlite3: missing NUL terminator")
	NoBinaryErr  = ErrorString("sqlite3: no SQLite binary embed/set/loaded")
	BadBinaryErr = ErrorString("sqlite3: invalid SQLite binary embed/set/loaded")
	TimeErr      = ErrorString("sqlite3: invalid time value")
	WhenceErr    = ErrorString("sqlite3: invalid whence")
	OffsetErr    = ErrorString("sqlite3: invalid offset")
	TailErr      = ErrorString("sqlite3: multiple statements")
	IsolationErr = ErrorString("sqlite3: unsupported isolation level")
	ValueErr     = ErrorString("sqlite3: unsupported value")
	NoVFSErr     = ErrorString("sqlite3: no such vfs: ")
)

func AssertErr() ErrorString {
	msg := "sqlite3: assertion failed"
	if _, file, line, ok := runtime.Caller(1); ok {
		msg += " (" + file + ":" + strconv.Itoa(line) + ")"
	}
	return ErrorString(msg)
}

func ErrorCodeString(rc uint32) string {
	switch rc {
	case ABORT_ROLLBACK:
		return "sqlite3: abort due to ROLLBACK"
	case ROW:
		return "sqlite3: another row available"
	case DONE:
		return "sqlite3: no more rows available"
	}
	switch rc & 0xff {
	case OK:
		return "sqlite3: not an error"
	case ERROR:
		return "sqlite3: SQL logic error"
	case INTERNAL:
		break
	case PERM:
		return "sqlite3: access permission denied"
	case ABORT:
		return "sqlite3: query aborted"
	case BUSY:
		return "sqlite3: database is locked"
	case LOCKED:
		return "sqlite3: database table is locked"
	case NOMEM:
		return "sqlite3: out of memory"
	case READONLY:
		return "sqlite3: attempt to write a readonly database"
	case INTERRUPT:
		return "sqlite3: interrupted"
	case IOERR:
		return "sqlite3: disk I/O error"
	case CORRUPT:
		return "sqlite3: database disk image is malformed"
	case NOTFOUND:
		return "sqlite3: unknown operation"
	case FULL:
		return "sqlite3: database or disk is full"
	case CANTOPEN:
		return "sqlite3: unable to open database file"
	case PROTOCOL:
		return "sqlite3: locking protocol"
	case EMPTY:
		break
	case SCHEMA:
		return "sqlite3: database schema has changed"
	case TOOBIG:
		return "sqlite3: string or blob too big"
	case CONSTRAINT:
		return "sqlite3: constraint failed"
	case MISMATCH:
		return "sqlite3: datatype mismatch"
	case MISUSE:
		return "sqlite3: bad parameter or other API misuse"
	case NOLFS:
		break
	case AUTH:
		return "sqlite3: authorization denied"
	case FORMAT:
		break
	case RANGE:
		return "sqlite3: column index out of range"
	case NOTADB:
		return "sqlite3: file is not a database"
	case NOTICE:
		return "sqlite3: notification message"
	case WARNING:
		return "sqlite3: warning message"
	}
	return "sqlite3: unknown error"
}

type ErrorJoiner []error

func (j *ErrorJoiner) Join(errs ...error) {
	for _, err := range errs {
		if err != nil {
			*j = append(*j, err)
		}
	}
}
