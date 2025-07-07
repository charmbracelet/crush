// EDIT(begin): custom time marshaler
package json

import (
	"github.com/anthropics/anthropic-sdk-go/internal/encoding/json/shims"
	"reflect"
	"time"
)

type TimeMarshaler interface {
	MarshalJSONWithTimeLayout(string) []byte
}

func TimeLayout(fmt string) string {
	switch fmt {
	case "", "date-time":
		return time.RFC3339
	case "date":
		return time.DateOnly
	default:
		return fmt
	}
}

var timeType = shims.TypeFor[time.Time]()

func newTimeEncoder() encoderFunc {
	return func(e *encodeState, v reflect.Value, opts encOpts) {
		t := v.Interface().(time.Time)
		fmtted := t.Format(TimeLayout(opts.timefmt))
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
