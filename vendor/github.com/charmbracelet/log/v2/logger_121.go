//go:build go1.21
// +build go1.21

package log

import (
	"context"
	"log/slog"
	"runtime"
	"sync/atomic"
)

// type aliases for slog.
type (
	slogAttr      = slog.Attr
	slogValue     = slog.Value
	slogLogValuer = slog.LogValuer
)

const slogKindGroup = slog.KindGroup

// Enabled reports whether the logger is enabled for the given level.
//
// Implements slog.Handler.
func (l *Logger) Enabled(_ context.Context, level slog.Level) bool {
	return atomic.LoadInt64(&l.level) <= int64(level)
}

// Handle handles the Record. It will only be called if Enabled returns true.
//
// Implements slog.Handler.
func (l *Logger) Handle(ctx context.Context, record slog.Record) error {
	if !l.Enabled(ctx, record.Level) {
		return nil
	}

	fields := make([]interface{}, 0, record.NumAttrs()*2)
	record.Attrs(func(a slog.Attr) bool {
		fields = append(fields, a.Key, a.Value)
		return true
	})
	// Get the caller frame using the record's PC.
	frames := runtime.CallersFrames([]uintptr{record.PC})
	frame, _ := frames.Next()
	l.handle(Level(record.Level), l.timeFunc(record.Time), []runtime.Frame{frame}, record.Message, fields...)
	return nil
}

// WithAttrs returns a new Handler with the given attributes added.
//
// Implements slog.Handler.
func (l *Logger) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make([]interface{}, 0, len(attrs)*2)
	for _, attr := range attrs {
		fields = append(fields, attr.Key, attr.Value)
	}
	return l.With(fields...)
}

// WithGroup returns a new Handler with the given group name prepended to the
// current group name or prefix.
//
// Implements slog.Handler.
func (l *Logger) WithGroup(name string) slog.Handler {
	if l.prefix != "" {
		name = l.prefix + "." + name
	}
	return l.WithPrefix(name)
}

var _ slog.Handler = (*Logger)(nil)
