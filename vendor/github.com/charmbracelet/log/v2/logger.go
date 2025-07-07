package log

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/colorprofile"
)

// ErrMissingValue is returned when a key is missing a value.
var ErrMissingValue = fmt.Errorf("missing value")

// LoggerOption is an option for a logger.
type LoggerOption = func(*Logger)

// Logger is a Logger that implements Logger.
type Logger struct {
	w  colorprofile.Writer
	b  bytes.Buffer
	mu *sync.RWMutex

	isDiscard uint32

	level           int64
	prefix          string
	timeFunc        TimeFunction
	timeFormat      string
	callerOffset    int
	callerFormatter CallerFormatter
	formatter       Formatter

	reportCaller    bool
	reportTimestamp bool

	fields []interface{}

	helpers *sync.Map
	styles  *Styles
}

// Logf logs a message with formatting.
func (l *Logger) Logf(level Level, format string, args ...interface{}) {
	l.Log(level, fmt.Sprintf(format, args...))
}

// Log logs the given message with the given keyvals for the given level.
func (l *Logger) Log(level Level, msg interface{}, keyvals ...interface{}) {
	if atomic.LoadUint32(&l.isDiscard) != 0 {
		return
	}

	// check if the level is allowed
	if atomic.LoadInt64(&l.level) > int64(level) {
		return
	}

	var frame runtime.Frame
	if l.reportCaller {
		// Skip log.log, the caller, and any offset added.
		frames := l.frames(l.callerOffset + 2)
		for {
			f, more := frames.Next()
			_, helper := l.helpers.Load(f.Function)
			if !helper || !more {
				// Found a frame that wasn't a helper function.
				// Or we ran out of frames to check.
				frame = f
				break
			}
		}
	}
	l.handle(level, l.timeFunc(time.Now()), []runtime.Frame{frame}, msg, keyvals...)
}

func (l *Logger) handle(level Level, ts time.Time, frames []runtime.Frame, msg interface{}, keyvals ...interface{}) {
	var kvs []interface{}
	if l.reportTimestamp && !ts.IsZero() {
		kvs = append(kvs, TimestampKey, ts)
	}

	_, ok := l.styles.Levels[level]
	if ok {
		kvs = append(kvs, LevelKey, level)
	}

	if l.reportCaller && len(frames) > 0 && frames[0].PC != 0 {
		file, line, fn := l.location(frames)
		if file != "" {
			caller := l.callerFormatter(file, line, fn)
			kvs = append(kvs, CallerKey, caller)
		}
	}

	if l.prefix != "" {
		kvs = append(kvs, PrefixKey, l.prefix)
	}

	if msg != nil {
		if m := fmt.Sprint(msg); m != "" {
			kvs = append(kvs, MessageKey, m)
		}
	}

	// append logger fields
	kvs = append(kvs, l.fields...)
	if len(l.fields)%2 != 0 {
		kvs = append(kvs, ErrMissingValue)
	}

	// append the rest
	kvs = append(kvs, keyvals...)
	if len(keyvals)%2 != 0 {
		kvs = append(kvs, ErrMissingValue)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	switch l.formatter {
	case LogfmtFormatter:
		l.logfmtFormatter(kvs...)
	case JSONFormatter:
		l.jsonFormatter(kvs...)
	default:
		l.textFormatter(kvs...)
	}

	// WriteTo will reset the buffer
	if _, err := l.b.WriteTo(&l.w); err != nil {
		if errors.Is(err, io.ErrShortWrite) {
			// Reset the buffer even if the lengths don't match up. If we're
			// using colorprofile's Writer, it will strip the ansi sequences based on
			// the color profile which can cause this error.
			l.b.Reset()
		}
	}
}

// Helper marks the calling function as a helper
// and skips it for source location information.
// It's the equivalent of testing.TB.Helper().
func (l *Logger) Helper() {
	l.helper(1)
}

func (l *Logger) helper(skip int) {
	var pcs [1]uintptr
	// Skip runtime.Callers, and l.helper
	n := runtime.Callers(skip+2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	frame, _ := frames.Next()
	l.helpers.LoadOrStore(frame.Function, struct{}{})
}

// frames returns the runtime.Frames for the caller.
func (l *Logger) frames(skip int) *runtime.Frames {
	// Copied from testing.T
	const maxStackLen = 50
	var pc [maxStackLen]uintptr

	// Skip runtime.Callers, and l.frame
	n := runtime.Callers(skip+2, pc[:])
	frames := runtime.CallersFrames(pc[:n])
	return frames
}

func (l *Logger) location(frames []runtime.Frame) (file string, line int, fn string) {
	if len(frames) == 0 {
		return "", 0, ""
	}
	f := frames[0]
	return f.File, f.Line, f.Function
}

// Cleanup a path by returning the last n segments of the path only.
func trimCallerPath(path string, n int) string {
	// lovely borrowed from zap
	// nb. To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.

	// Return the full path if n is 0.
	if n <= 0 {
		return path
	}

	// Find the last separator.
	idx := strings.LastIndexByte(path, '/')
	if idx == -1 {
		return path
	}

	for i := 0; i < n-1; i++ {
		// Find the penultimate separator.
		idx = strings.LastIndexByte(path[:idx], '/')
		if idx == -1 {
			return path
		}
	}

	return path[idx+1:]
}

// SetReportTimestamp sets whether the timestamp should be reported.
func (l *Logger) SetReportTimestamp(report bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reportTimestamp = report
}

// SetReportCaller sets whether the caller location should be reported.
func (l *Logger) SetReportCaller(report bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reportCaller = report
}

// GetLevel returns the current level.
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return Level(l.level)
}

// SetLevel sets the current level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	atomic.StoreInt64(&l.level, int64(level))
}

// GetPrefix returns the current prefix.
func (l *Logger) GetPrefix() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.prefix
}

// SetPrefix sets the current prefix.
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// SetTimeFormat sets the time format.
func (l *Logger) SetTimeFormat(format string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeFormat = format
}

// SetTimeFunction sets the time function.
func (l *Logger) SetTimeFunction(f TimeFunction) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeFunc = f
}

// SetOutput sets the output destination.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w == nil {
		w = os.Stderr
	}
	l.w.Forward = w
	var isDiscard uint32
	if w == io.Discard {
		isDiscard = 1
	}
	atomic.StoreUint32(&l.isDiscard, isDiscard)
}

// SetColorProfile force sets the underlying color profile for the
// TextFormatter.
func (l *Logger) SetColorProfile(profile colorprofile.Profile) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.w.Profile = profile
}

// SetFormatter sets the formatter.
func (l *Logger) SetFormatter(f Formatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = f
}

// SetCallerFormatter sets the caller formatter.
func (l *Logger) SetCallerFormatter(f CallerFormatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callerFormatter = f
}

// SetCallerOffset sets the caller offset.
func (l *Logger) SetCallerOffset(offset int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callerOffset = offset
}

// SetStyles sets the logger styles for the TextFormatter.
func (l *Logger) SetStyles(s *Styles) {
	if s == nil {
		s = DefaultStyles()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.styles = s
}

// With returns a new logger with the given keyvals added.
func (l *Logger) With(keyvals ...interface{}) *Logger {
	var st Styles
	l.mu.Lock()
	sl := *l
	st = *l.styles
	l.mu.Unlock()
	sl.b = bytes.Buffer{}
	sl.mu = &sync.RWMutex{}
	sl.helpers = &sync.Map{}
	sl.fields = append(make([]interface{}, 0, len(l.fields)+len(keyvals)), l.fields...)
	sl.fields = append(sl.fields, keyvals...)
	sl.styles = &st
	return &sl
}

// WithPrefix returns a new logger with the given prefix.
func (l *Logger) WithPrefix(prefix string) *Logger {
	sl := l.With()
	sl.SetPrefix(prefix)
	return sl
}

// Debug prints a debug message.
func (l *Logger) Debug(msg interface{}, keyvals ...interface{}) {
	l.Log(DebugLevel, msg, keyvals...)
}

// Info prints an info message.
func (l *Logger) Info(msg interface{}, keyvals ...interface{}) {
	l.Log(InfoLevel, msg, keyvals...)
}

// Warn prints a warning message.
func (l *Logger) Warn(msg interface{}, keyvals ...interface{}) {
	l.Log(WarnLevel, msg, keyvals...)
}

// Error prints an error message.
func (l *Logger) Error(msg interface{}, keyvals ...interface{}) {
	l.Log(ErrorLevel, msg, keyvals...)
}

// Fatal prints a fatal message and exits.
func (l *Logger) Fatal(msg interface{}, keyvals ...interface{}) {
	l.Log(FatalLevel, msg, keyvals...)
	os.Exit(1)
}

// Print prints a message with no level.
func (l *Logger) Print(msg interface{}, keyvals ...interface{}) {
	l.Log(noLevel, msg, keyvals...)
}

// Debugf prints a debug message with formatting.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Log(DebugLevel, fmt.Sprintf(format, args...))
}

// Infof prints an info message with formatting.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Log(InfoLevel, fmt.Sprintf(format, args...))
}

// Warnf prints a warning message with formatting.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Log(WarnLevel, fmt.Sprintf(format, args...))
}

// Errorf prints an error message with formatting.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Log(ErrorLevel, fmt.Sprintf(format, args...))
}

// Fatalf prints a fatal message with formatting and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Log(FatalLevel, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Printf prints a message with no level and formatting.
func (l *Logger) Printf(format string, args ...interface{}) {
	l.Log(noLevel, fmt.Sprintf(format, args...))
}
