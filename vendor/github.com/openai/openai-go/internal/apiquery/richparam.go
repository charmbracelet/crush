package apiquery

import (
	"reflect"

	"github.com/openai/openai-go/packages/param"
)

func (e *encoder) newRichFieldTypeEncoder(t reflect.Type) encoderFunc {
	f, _ := t.FieldByName("Value")
	enc := e.typeEncoder(f.Type)
	return func(key string, value reflect.Value) ([]Pair, error) {
		if opt, ok := value.Interface().(param.Optional); ok && opt.Valid() {
			return enc(key, value.FieldByIndex(f.Index))
		} else if ok && param.IsNull(opt) {
			return []Pair{{key, "null"}}, nil
		}
		return nil, nil
	}
}
