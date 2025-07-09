package log

import (
	"fmt"
	"time"
)

// DefaultTimeFormat is the default time format.
const DefaultTimeFormat = "2006/01/02 15:04:05"

// TimeFunction is a function that returns a time.Time.
type TimeFunction = func(time.Time) time.Time

// NowUTC is a convenient function that returns the
// current time in UTC timezone.
//
// This is to be used as a time function.
// For example:
//
//	log.SetTimeFunction(log.NowUTC)
func NowUTC(t time.Time) time.Time {
	return t.UTC()
}

// CallerFormatter is the caller formatter.
type CallerFormatter func(string, int, string) string

// ShortCallerFormatter is a caller formatter that returns the last 2 levels of the path
// and line number.
func ShortCallerFormatter(file string, line int, _ string) string {
	return fmt.Sprintf("%s:%d", trimCallerPath(file, 2), line)
}

// LongCallerFormatter is a caller formatter that returns the full path and line number.
func LongCallerFormatter(file string, line int, _ string) string {
	return fmt.Sprintf("%s:%d", file, line)
}

// Options is the options for the logger.
type Options struct {
	// TimeFunction is the time function for the logger. The default is time.Now.
	TimeFunction TimeFunction
	// TimeFormat is the time format for the logger. The default is "2006/01/02 15:04:05".
	TimeFormat string
	// Level is the level for the logger. The default is InfoLevel.
	Level Level
	// Prefix is the prefix for the logger. The default is no prefix.
	Prefix string
	// ReportTimestamp is whether the logger should report the timestamp. The default is false.
	ReportTimestamp bool
	// ReportCaller is whether the logger should report the caller location. The default is false.
	ReportCaller bool
	// CallerFormatter is the caller format for the logger. The default is ShortCallerFormatter.
	CallerFormatter CallerFormatter
	// CallerOffset is the caller format for the logger. The default is 0.
	CallerOffset int
	// Fields is the fields for the logger. The default is no fields.
	Fields []interface{}
	// Formatter is the formatter for the logger. The default is TextFormatter.
	Formatter Formatter
}
