package param

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	shimjson "github.com/openai/openai-go/internal/encoding/json"

	"github.com/tidwall/sjson"
)

// This type will not be stable and shouldn't be relied upon
type EncodedAsDate Opt[time.Time]

func (m EncodedAsDate) MarshalJSON() ([]byte, error) {
	underlying := Opt[time.Time](m)
	bytes := underlying.MarshalJSONWithTimeLayout("2006-01-02")
	if len(bytes) > 0 {
		return bytes, nil
	}
	return underlying.MarshalJSON()
}

// This uses a shimmed 'encoding/json' from Go 1.24, to support the 'omitzero' tag
func MarshalObject[T OverridableObject](f T, underlying any) ([]byte, error) {
	if f.IsNull() {
		return []byte("null"), nil
	} else if extras := f.GetExtraFields(); extras != nil {
		bytes, err := shimjson.Marshal(underlying)
		if err != nil {
			return nil, err
		}
		for k, v := range extras {
			bytes, err = sjson.SetBytes(bytes, k, v)
			if err != nil {
				return nil, err
			}
		}
		return bytes, nil
	} else if ovr, ok := f.IsOverridden(); ok {
		return shimjson.Marshal(ovr)
	} else {
		return shimjson.Marshal(underlying)
	}
}

// This uses a shimmed 'encoding/json' from Go 1.24, to support the 'omitzero' tag
func MarshalUnion[T any](variants ...any) ([]byte, error) {
	nPresent := 0
	presentIdx := -1
	for i, variant := range variants {
		if !IsOmitted(variant) {
			nPresent++
			presentIdx = i
		}
	}
	if nPresent == 0 || presentIdx == -1 {
		return []byte(`null`), nil
	} else if nPresent > 1 {
		return nil, &json.MarshalerError{
			Type: typeFor[T](),
			Err:  fmt.Errorf("expected union to have only one present variant, got %d", nPresent),
		}
	}
	return shimjson.Marshal(variants[presentIdx])
}

// shimmed from Go 1.23 "reflect" package
func typeFor[T any]() reflect.Type {
	var v T
	if t := reflect.TypeOf(v); t != nil {
		return t // optimize for T being a non-interface kind
	}
	return reflect.TypeOf((*T)(nil)).Elem() // only for an interface kind
}

var optStringType = typeFor[Opt[string]]()
var optIntType = typeFor[Opt[int64]]()
var optFloatType = typeFor[Opt[float64]]()
var optBoolType = typeFor[Opt[bool]]()

var OptionalPrimitiveTypes map[reflect.Type][]int

// indexOfUnderlyingValueField must only be called at initialization time
func indexOfUnderlyingValueField(t reflect.Type) []int {
	field, ok := t.FieldByName("Value")
	if !ok {
		panic("unreachable: initialization issue, underlying value field not found")
	}
	return field.Index
}

func init() {
	OptionalPrimitiveTypes = map[reflect.Type][]int{
		optStringType: indexOfUnderlyingValueField(optStringType),
		optIntType:    indexOfUnderlyingValueField(optIntType),
		optFloatType:  indexOfUnderlyingValueField(optFloatType),
		optBoolType:   indexOfUnderlyingValueField(optBoolType),
	}
}

var structFieldsCache sync.Map

func structFields(t reflect.Type) (map[string][]int, error) {
	if cached, ok := structFieldsCache.Load(t); ok {
		return cached.(map[string][]int), nil
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("resp: expected struct but got %v of kind %v", t.String(), t.Kind().String())
	}
	structFields := map[string][]int{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" || field.Anonymous {
			continue
		}
		structFields[name] = field.Index
	}
	return structFields, nil
}
