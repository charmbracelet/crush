package apiform

import (
	"github.com/openai/openai-go/packages/param"
	"mime/multipart"
	"reflect"
)

func (e *encoder) newRichFieldTypeEncoder(t reflect.Type, underlyingValueIdx []int) encoderFunc {
	underlying := t.FieldByIndex(underlyingValueIdx)
	primitiveEncoder := e.newPrimitiveTypeEncoder(underlying.Type)
	return func(key string, value reflect.Value, writer *multipart.Writer) error {
		if opt, ok := value.Interface().(param.Optional); ok && opt.IsPresent() {
			return primitiveEncoder(key, value.FieldByIndex(underlyingValueIdx), writer)
		} else if ok && opt.IsNull() {
			return writer.WriteField(key, "null")
		}
		return nil
	}
}
