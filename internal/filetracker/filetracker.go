// Package filetracker tracks file read/write times to prevent editing files
// that haven't been read, and to detect external modifications.
//
// TODO: Consider moving this to persistent storage (e.g., the database) to
// preserve file access history across sessions.
// We would need to make sure to handle the case where we reload a session and the underlying files did change.
package filetracker

import (
	"sync"
	"time"
)

// record tracks when a file was read/written.
type record struct {
	path      string
	readTime  time.Time
	writeTime time.Time
}

var (
	records     = make(map[string]record)
	recordMutex sync.RWMutex
)

// RecordRead records when a file was read.
func RecordRead(path string) {
	recordMutex.Lock()
	defer recordMutex.Unlock()

	rec, exists := records[path]
	if !exists {
		rec = record{path: path}
	}
	rec.readTime = time.Now()
	records[path] = rec
}

// LastReadTime returns when a file was last read. Returns zero time if never
// read.
func LastReadTime(path string) time.Time {
	recordMutex.RLock()
	defer recordMutex.RUnlock()

	rec, exists := records[path]
	if !exists {
		return time.Time{}
	}
	return rec.readTime
}

// RecordWrite records when a file was written.
func RecordWrite(path string) {
	recordMutex.Lock()
	defer recordMutex.Unlock()

	rec, exists := records[path]
	if !exists {
		rec = record{path: path}
	}
	rec.writeTime = time.Now()
	records[path] = rec
}

// Reset clears all file tracking records. Useful for testing.
func Reset() {
	recordMutex.Lock()
	defer recordMutex.Unlock()
	records = make(map[string]record)
}
