package sqlite3

import (
	"bytes"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// Quote escapes and quotes a value
// making it safe to embed in SQL text.
// Strings with embedded NUL characters are truncated.
//
// https://sqlite.org/lang_corefunc.html#quote
func Quote(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case bool:
		if v {
			return "1"
		} else {
			return "0"
		}

	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		switch {
		case math.IsNaN(v):
			return "NULL"
		case math.IsInf(v, 1):
			return "9.0e999"
		case math.IsInf(v, -1):
			return "-9.0e999"
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case time.Time:
		return "'" + v.Format(time.RFC3339Nano) + "'"

	case string:
		if i := strings.IndexByte(v, 0); i >= 0 {
			v = v[:i]
		}

		buf := make([]byte, 2+len(v)+strings.Count(v, "'"))
		buf[0] = '\''
		i := 1
		for _, b := range []byte(v) {
			if b == '\'' {
				buf[i] = b
				i += 1
			}
			buf[i] = b
			i += 1
		}
		buf[len(buf)-1] = '\''
		return unsafe.String(&buf[0], len(buf))

	case []byte:
		buf := make([]byte, 3+2*len(v))
		buf[1] = '\''
		buf[0] = 'x'
		i := 2
		for _, b := range v {
			const hex = "0123456789ABCDEF"
			buf[i+0] = hex[b/16]
			buf[i+1] = hex[b%16]
			i += 2
		}
		buf[len(buf)-1] = '\''
		return unsafe.String(&buf[0], len(buf))

	case ZeroBlob:
		buf := bytes.Repeat([]byte("0"), int(3+2*int64(v)))
		buf[1] = '\''
		buf[0] = 'x'
		buf[len(buf)-1] = '\''
		return unsafe.String(&buf[0], len(buf))
	}

	v := reflect.ValueOf(value)
	k := v.Kind()

	if k == reflect.Interface || k == reflect.Pointer {
		if v.IsNil() {
			return "NULL"
		}
		v = v.Elem()
		k = v.Kind()
	}

	switch {
	case v.CanInt():
		return strconv.FormatInt(v.Int(), 10)
	case v.CanUint():
		return strconv.FormatUint(v.Uint(), 10)
	case v.CanFloat():
		return Quote(v.Float())
	case k == reflect.Bool:
		return Quote(v.Bool())
	case k == reflect.String:
		return Quote(v.String())
	case (k == reflect.Slice || k == reflect.Array && v.CanAddr()) &&
		v.Type().Elem().Kind() == reflect.Uint8:
		return Quote(v.Bytes())
	}

	panic(util.ValueErr)
}

// QuoteIdentifier escapes and quotes an identifier
// making it safe to embed in SQL text.
// Strings with embedded NUL characters panic.
func QuoteIdentifier(id string) string {
	if strings.IndexByte(id, 0) >= 0 {
		panic(util.ValueErr)
	}

	buf := make([]byte, 2+len(id)+strings.Count(id, `"`))
	buf[0] = '"'
	i := 1
	for _, b := range []byte(id) {
		if b == '"' {
			buf[i] = b
			i += 1
		}
		buf[i] = b
		i += 1
	}
	buf[len(buf)-1] = '"'
	return unsafe.String(&buf[0], len(buf))
}
