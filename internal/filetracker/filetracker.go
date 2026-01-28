// Package filetracker tracks file read/write times to prevent editing files
// that haven't been read, and to detect external modifications.
//
// TODO: delete this file when deleting the old TUI.
package filetracker

import (
	"sync"
	"time"
)

// record tracks when a file was read/written.
type record struct {
	path     string
	readTime time.Time
}

var (
	records     = make(map[string]record)
	recordMutex sync.RWMutex
)

// RecordRead records when a file was read.
//
// Deprecated: Use the filetracker.Service implementation instead.
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
//
// Deprecated: Use the filetracker.Service implementation instead.
func LastReadTime(path string) time.Time {
	recordMutex.RLock()
	defer recordMutex.RUnlock()

	rec, exists := records[path]
	if !exists {
		return time.Time{}
	}
	return rec.readTime
}
