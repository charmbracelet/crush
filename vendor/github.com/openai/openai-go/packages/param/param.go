package param

import (
	"encoding/json"
	"reflect"
)

// NullObj is used to mark a struct as null.
// To send null to an [Opt] field use [NullOpt].
func NullObj[T NullableObject, PT Settable[T]]() T {
	var t T
	pt := PT(&t)
	pt.setMetadata(nil)
	return *pt
}

// To override a specific field in a struct, use its [WithExtraFields] method.
func OverrideObj[T OverridableObject, PT Settable[T]](v any) T {
	var t T
	pt := PT(&t)
	pt.setMetadata(nil)
	return *pt
}

// IsOmitted returns true if v is the zero value of its type.
//
// It indicates if a field with the `json:"...,omitzero"` tag will be omitted
// from serialization.
//
// If v is set explicitly to the JSON value "null", this function will return false.
// Therefore, when available, prefer using the [IsPresent] method to check whether
// a field is present.
//
// Generally, this function should only be used on structs, arrays, maps.
func IsOmitted(v any) bool {
	if v == nil {
		return false
	}
	if o, ok := v.(interface{ IsOmitted() bool }); ok {
		return o.IsOmitted()
	}
	return reflect.ValueOf(v).IsZero()
}

type NullableObject = overridableStruct
type OverridableObject = overridableStruct

type Settable[T overridableStruct] interface {
	setMetadata(any)
	*T
}

type overridableStruct interface {
	IsNull() bool
	IsOverridden() (any, bool)
	GetExtraFields() map[string]any
}

// APIObject should be embedded in api object fields, preferably using an alias to make private
type APIObject struct{ metadata }

// APIUnion should be embedded in all api unions fields, preferably using an alias to make private
type APIUnion struct{ metadata }

type metadata struct{ any }
type metadataNull struct{}
type metadataExtraFields map[string]any

// IsNull returns true if the field is the explicit value `null`,
// prefer using [IsPresent] to check for presence, since it checks against null and omitted.
func (m metadata) IsNull() bool {
	if _, ok := m.any.(metadataNull); ok {
		return true
	}

	if msg, ok := m.any.(json.RawMessage); ok {
		return string(msg) == "null"
	}

	return false
}

func (m metadata) IsOverridden() (any, bool) {
	if _, ok := m.any.(metadataExtraFields); ok {
		return nil, false
	}
	return m.any, m.any != nil
}

func (m metadata) GetExtraFields() map[string]any {
	if extras, ok := m.any.(metadataExtraFields); ok {
		return extras
	}
	return nil
}

func (m *metadata) WithExtraFields(fields map[string]any) {
	m.any = metadataExtraFields(fields)
}

func (m *metadata) setMetadata(override any) {
	if override == nil {
		m.any = metadataNull{}
		return
	}
	m.any = override
}
