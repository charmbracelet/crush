package apiform

import (
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"mime/multipart"
	"reflect"
)

func (e *encoder) newRichFieldTypeEncoder(t reflect.Type) encoderFunc {
	f, _ := t.FieldByName("Value")
	enc := e.newPrimitiveTypeEncoder(f.Type)
	return func(key string, value reflect.Value, writer *multipart.Writer) error {
		if opt, ok := value.Interface().(param.Optional); ok && opt.Valid() {
			return enc(key, value.FieldByIndex(f.Index), writer)
		} else if ok && param.IsNull(opt) {
			return writer.WriteField(key, "null")
		}
		return nil
	}
}
