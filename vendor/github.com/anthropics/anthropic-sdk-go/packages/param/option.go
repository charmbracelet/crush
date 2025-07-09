package param

import (
	"encoding/json"
	"fmt"
	shimjson "github.com/anthropics/anthropic-sdk-go/internal/encoding/json"
	"time"
)

func NewOpt[T comparable](v T) Opt[T] {
	return Opt[T]{Value: v, status: included}
}

// Null creates optional field with the JSON value "null".
//
// To set a struct to null, use [NullStruct].
func Null[T comparable]() Opt[T] { return Opt[T]{status: null} }

type status int8

const (
	omitted status = iota
	null
	included
)

// Opt represents an optional parameter of type T. Use
// the [Opt.Valid] method to confirm.
type Opt[T comparable] struct {
	Value T
	// indicates whether the field should be omitted, null, or valid
	status status
	opt
}

// Valid returns true if the value is not "null" or omitted.
//
// To check if explicitly null, use [Opt.Null].
func (o Opt[T]) Valid() bool {
	var empty Opt[T]
	return o.status == included || o != empty && o.status != null
}

func (o Opt[T]) Or(v T) T {
	if o.Valid() {
		return o.Value
	}
	return v
}

func (o Opt[T]) String() string {
	if o.null() {
		return "null"
	}
	if s, ok := any(o.Value).(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", o.Value)
}

func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if !o.Valid() {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

func (o *Opt[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.status = null
		return nil
	}

	var value *T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	if value == nil {
		o.status = omitted
		return nil
	}

	o.status = included
	o.Value = *value
	return nil
}

// MarshalJSONWithTimeLayout is necessary to bypass the internal caching performed
// by [json.Marshal]. Prefer to use [Opt.MarshalJSON] instead.
//
// This function requires that the generic type parameter of [Opt] is not [time.Time].
func (o Opt[T]) MarshalJSONWithTimeLayout(format string) []byte {
	t, ok := any(o.Value).(time.Time)
	if !ok || o.null() {
		return nil
	}

	b, err := json.Marshal(t.Format(shimjson.TimeLayout(format)))
	if err != nil {
		return nil
	}
	return b
}

func (o Opt[T]) null() bool   { return o.status == null }
func (o Opt[T]) isZero() bool { return o == Opt[T]{} }

// opt helps limit the [Optional] interface to only types in this package
type opt struct{}

func (opt) implOpt() {}

// This interface is useful for internal purposes.
type Optional interface {
	Valid() bool
	null() bool

	isZero() bool
	implOpt()
}
