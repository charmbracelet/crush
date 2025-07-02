package param

import (
	"encoding/json"
	"reflect"
	"time"
)

func NewOpt[T comparable](v T) Opt[T] {
	return Opt[T]{Value: v, Status: included}
}

// Sets an optional field to null, to set an object to null use [NullObj].
func NullOpt[T comparable]() Opt[T] { return Opt[T]{Status: null} }

type Opt[T comparable] struct {
	Value T
	// indicates whether the field should be omitted, null, or valid
	Status Status
}

type Status int8

const (
	omitted Status = iota
	null
	included
)

type Optional interface {
	// IsPresent returns true if the value is not "null" or omitted
	IsPresent() bool

	// IsOmitted returns true if the value is omitted, it returns false if the value is "null".
	IsOmitted() bool

	// IsNull returns true if the value is "null", it returns false if the value is omitted.
	IsNull() bool
}

// IsPresent returns true if the value is not "null" and not omitted
func (o Opt[T]) IsPresent() bool {
	var empty Opt[T]
	return o.Status == included || o != empty && o.Status != null
}

// IsNull returns true if the value is specifically the JSON value "null".
// It returns false if the value is omitted.
//
// Prefer to use [IsPresent] to check the presence of a value.
func (o Opt[T]) IsNull() bool { return o.Status == null }

// IsOmitted returns true if the value is omitted.
// It returns false if the value is the JSON value "null".
//
// Prefer to use [IsPresent] to check the presence of a value.
func (o Opt[T]) IsOmitted() bool { return o == Opt[T]{} }

func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if !o.IsPresent() {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

func (o *Opt[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.Status = null
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

func (o Opt[T]) Or(v T) T {
	if o.IsPresent() {
		return o.Value
	}
	return v
}

// This is a sketchy way to implement time Formatting
var timeType = reflect.TypeOf(time.Time{})
var timeTimeValueLoc, _ = reflect.TypeOf(Opt[time.Time]{}).FieldByName("Value")

// Don't worry about this function, returns nil to fallback towards [MarshalJSON]
func (o Opt[T]) MarshalJSONWithTimeLayout(format string) []byte {
	t, ok := any(o.Value).(time.Time)
	if !ok || o.IsNull() {
		return nil
	}

	if format == "" {
		format = time.RFC3339
	} else if format == "date" {
		format = "2006-01-02"
	}

	b, err := json.Marshal(t.Format(format))
	if err != nil {
		return nil
	}
	return b
}
