package anthropic

import (
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"io"
	"time"
)

func String(s string) param.Opt[string]     { return param.NewOpt(s) }
func Int(i int64) param.Opt[int64]          { return param.NewOpt(i) }
func Bool(b bool) param.Opt[bool]           { return param.NewOpt(b) }
func Float(f float64) param.Opt[float64]    { return param.NewOpt(f) }
func Time(t time.Time) param.Opt[time.Time] { return param.NewOpt(t) }

func Opt[T comparable](v T) param.Opt[T] { return param.NewOpt(v) }
func Ptr[T any](v T) *T                  { return &v }

func IntPtr(v int64) *int64          { return &v }
func BoolPtr(v bool) *bool           { return &v }
func FloatPtr(v float64) *float64    { return &v }
func StringPtr(v string) *string     { return &v }
func TimePtr(v time.Time) *time.Time { return &v }

func File(rdr io.Reader, filename string, contentType string) file {
	return file{rdr, filename, contentType}
}

type file struct {
	io.Reader
	name        string
	contentType string
}

func (f file) Filename() string {
	if f.name != "" {
		return f.name
	} else if named, ok := f.Reader.(interface{ Name() string }); ok {
		return named.Name()
	}
	return ""
}

func (f file) ContentType() string {
	return f.contentType
}
