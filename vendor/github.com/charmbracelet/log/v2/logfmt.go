package log

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-logfmt/logfmt"
)

func (l *Logger) logfmtFormatter(keyvals ...interface{}) {
	e := logfmt.NewEncoder(&l.b)

	for i := 0; i < len(keyvals); i += 2 {
		switch keyvals[i] {
		case TimestampKey:
			if t, ok := keyvals[i+1].(time.Time); ok {
				keyvals[i+1] = t.Format(l.timeFormat)
			}
		default:
			if key := fmt.Sprint(keyvals[i]); key != "" {
				keyvals[i] = key
			}
		}
		err := e.EncodeKeyval(keyvals[i], keyvals[i+1])
		if err != nil && errors.Is(err, logfmt.ErrUnsupportedValueType) {
			// If the value is not supported by logfmt, we try to convert it to a string.
			_ = e.EncodeKeyval(keyvals[i], fmt.Sprintf("%+v", keyvals[i+1]))
		}
	}
	_ = e.EndRecord()
}
