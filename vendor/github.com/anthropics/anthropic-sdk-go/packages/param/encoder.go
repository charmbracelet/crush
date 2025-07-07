package param

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	shimjson "github.com/anthropics/anthropic-sdk-go/internal/encoding/json"

	"github.com/tidwall/sjson"
)

// EncodedAsDate is not be stable and shouldn't be relied upon
type EncodedAsDate Opt[time.Time]

type forceOmit int

func (m EncodedAsDate) MarshalJSON() ([]byte, error) {
	underlying := Opt[time.Time](m)
	bytes := underlying.MarshalJSONWithTimeLayout("2006-01-02")
	if len(bytes) > 0 {
		return bytes, nil
	}
	return underlying.MarshalJSON()
}

// MarshalObject uses a shimmed 'encoding/json' from Go 1.24, to support the 'omitzero' tag
//
// Stability for the API of MarshalObject is not guaranteed.
func MarshalObject[T ParamStruct](f T, underlying any) ([]byte, error) {
	return MarshalWithExtras(f, underlying, f.extraFields())
}

// MarshalWithExtras is used to marshal a struct with additional properties.
//
// Stability for the API of MarshalWithExtras is not guaranteed.
func MarshalWithExtras[T ParamStruct, R any](f T, underlying any, extras map[string]R) ([]byte, error) {
	if f.null() {
		return []byte("null"), nil
	} else if len(extras) > 0 {
		bytes, err := shimjson.Marshal(underlying)
		if err != nil {
			return nil, err
		}
		for k, v := range extras {
			var a any = v
			if a == Omit {
				// Errors when handling ForceOmitted are ignored.
				if b, e := sjson.DeleteBytes(bytes, k); e == nil {
					bytes = b
				}
				continue
			}
			bytes, err = sjson.SetBytes(bytes, k, v)
			if err != nil {
				return nil, err
			}
		}
		return bytes, nil
	} else if ovr, ok := f.Overrides(); ok {
		return shimjson.Marshal(ovr)
	} else {
		return shimjson.Marshal(underlying)
	}
}

// MarshalUnion uses a shimmed 'encoding/json' from Go 1.24, to support the 'omitzero' tag
//
// Stability for the API of MarshalUnion is not guaranteed.
func MarshalUnion[T ParamStruct](metadata T, variants ...any) ([]byte, error) {
	nPresent := 0
	presentIdx := -1
	for i, variant := range variants {
		if !IsOmitted(variant) {
			nPresent++
			presentIdx = i
		}
	}
	if nPresent == 0 || presentIdx == -1 {
		if ovr, ok := metadata.Overrides(); ok {
			return shimjson.Marshal(ovr)
		}
		return []byte(`null`), nil
	} else if nPresent > 1 {
		return nil, &json.MarshalerError{
			Type: typeFor[T](),
			Err:  fmt.Errorf("expected union to have only one present variant, got %d", nPresent),
		}
	}
	return shimjson.Marshal(variants[presentIdx])
}

// typeFor is shimmed from Go 1.23 "reflect" package
func typeFor[T any]() reflect.Type {
	var v T
	if t := reflect.TypeOf(v); t != nil {
		return t // optimize for T being a non-interface kind
	}
	return reflect.TypeOf((*T)(nil)).Elem() // only for an interface kind
}
