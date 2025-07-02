// EDIT(begin): custom time marshaler
package json

import (
	"github.com/openai/openai-go/internal/encoding/json/shims"
	"reflect"
	"time"
)

type TimeMarshaler interface {
	MarshalJSONWithTimeLayout(string) []byte
}

var timeType = shims.TypeFor[time.Time]()

const DateFmt = "2006-01-02"

func newTimeEncoder() encoderFunc {
	return func(e *encodeState, v reflect.Value, opts encOpts) {
		t := v.Interface().(time.Time)
		fmtted := t.Format(opts.timefmt)
		if opts.timefmt == "date" {
			fmtted = t.Format(DateFmt)
		}
		// Default to RFC3339 if format is invalid
		if fmtted == "" {
			fmtted = t.Format(time.RFC3339)
		}
		stringEncoder(e, reflect.ValueOf(fmtted), opts)
	}
}

// Uses continuation passing style, to add the timefmt option to k
func continueWithTimeFmt(timefmt string, k encoderFunc) encoderFunc {
	return func(e *encodeState, v reflect.Value, opts encOpts) {
		opts.timefmt = timefmt
		k(e, v, opts)
	}
}

func timeMarshalEncoder(e *encodeState, v reflect.Value, opts encOpts) bool {
	tm, ok := v.Interface().(TimeMarshaler)
	if !ok {
		return false
	}

	b := tm.MarshalJSONWithTimeLayout(opts.timefmt)
	if b != nil {
		e.Grow(len(b))
		out := e.AvailableBuffer()
		out, _ = appendCompact(out, b, opts.escapeHTML)
		e.Buffer.Write(out)
		return true
	}

	return false
}

// EDIT(end)
