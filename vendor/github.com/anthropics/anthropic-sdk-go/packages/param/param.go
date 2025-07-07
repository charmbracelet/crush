package param

import (
	"encoding/json"
	"reflect"
)

// NullStruct is used to set a struct to the JSON value null.
// Check for null structs with [IsNull].
//
// Only the first type parameter should be provided,
// the type PtrT will be inferred.
//
//	json.Marshal(param.NullStruct[MyStruct]()) -> 'null'
//
// To send null to an [Opt] field use [Null].
func NullStruct[T ParamStruct, PtrT InferPtr[T]]() T {
	var t T
	pt := PtrT(&t)
	pt.setMetadata(nil)
	return *pt
}

// Override replaces the value of a struct with any type.
//
// Only the first type parameter should be provided,
// the type PtrT will be inferred.
//
// It's often useful for providing raw JSON
//
//	param.Override[MyStruct](json.RawMessage(`{"foo": "bar"}`))
//
// The public fields of the returned struct T will be unset.
//
// To override a specific field in a struct, use its [SetExtraFields] method.
func Override[T ParamStruct, PtrT InferPtr[T]](v any) T {
	var t T
	pt := PtrT(&t)
	pt.setMetadata(v)
	return *pt
}

// IsOmitted returns true if v is the zero value of its type.
//
// If IsOmitted is true, and the field uses a `json:"...,omitzero"` tag,
// the field will be omitted from the request.
//
// If v is set explicitly to the JSON value "null", IsOmitted returns false.
func IsOmitted(v any) bool {
	if v == nil {
		return false
	}
	if o, ok := v.(Optional); ok {
		return o.isZero()
	}
	return reflect.ValueOf(v).IsZero()
}

// IsNull returns true if v was set to the JSON value null.
//
// To set a param to null use [NullStruct] or [Null]
// depending on the type of v.
//
// IsNull returns false if the value is omitted.
func IsNull(v ParamNullable) bool {
	return v.null()
}

// ParamNullable encapsulates all structs in parameters,
// and all [Opt] types in parameters.
type ParamNullable interface {
	null() bool
}

// ParamStruct represents the set of all structs that are
// used in API parameters, by convention these usually end in
// "Params" or "Param".
type ParamStruct interface {
	Overrides() (any, bool)
	null() bool
	extraFields() map[string]any
}

// This is an implementation detail and should never be explicitly set.
type InferPtr[T ParamStruct] interface {
	setMetadata(any)
	*T
}

// APIObject should be embedded in api object fields, preferably using an alias to make private
type APIObject struct{ metadata }

// APIUnion should be embedded in all api unions fields, preferably using an alias to make private
type APIUnion struct{ metadata }

// Overrides returns the value of the struct when it is created with
// [Override], the second argument helps differentiate an explicit null.
func (m metadata) Overrides() (any, bool) {
	if _, ok := m.any.(metadataExtraFields); ok {
		return nil, false
	}
	return m.any, m.any != nil
}

// ExtraFields returns the extra fields added to the JSON object.
func (m metadata) ExtraFields() map[string]any {
	if extras, ok := m.any.(metadataExtraFields); ok {
		return extras
	}
	return nil
}

// Omit can be used with [metadata.SetExtraFields] to ensure that a
// required field is omitted. This is useful as an escape hatch for
// when a required is unwanted for some unexpected reason.
const Omit forceOmit = -1

// SetExtraFields adds extra fields to the JSON object.
//
// SetExtraFields will override any existing fields with the same key.
// For security reasons, ensure this is only used with trusted input data.
//
// To intentionally omit a required field, use [Omit].
//
//	foo.SetExtraFields(map[string]any{"bar": Omit})
//
// If the struct already contains the field ExtraFields, then this
// method will have no effect.
func (m *metadata) SetExtraFields(extraFields map[string]any) {
	m.any = metadataExtraFields(extraFields)
}

// extraFields aliases [metadata.ExtraFields] to avoid name collisions.
func (m metadata) extraFields() map[string]any { return m.ExtraFields() }

func (m metadata) null() bool {
	if _, ok := m.any.(metadataNull); ok {
		return true
	}

	if msg, ok := m.any.(json.RawMessage); ok {
		return string(msg) == "null"
	}

	return false
}

type metadata struct{ any }
type metadataNull struct{}
type metadataExtraFields map[string]any

func (m *metadata) setMetadata(override any) {
	if override == nil {
		m.any = metadataNull{}
		return
	}
	m.any = override
}
