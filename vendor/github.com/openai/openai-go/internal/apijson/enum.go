package apijson

import (
	"fmt"
	"reflect"
	"sync"
)

/********************/
/* Validating Enums */
/********************/

type validationEntry struct {
	field       reflect.StructField
	nullable    bool
	legalValues []reflect.Value
}

type validatorFunc func(reflect.Value) exactness

var validators sync.Map
var validationRegistry = map[reflect.Type][]validationEntry{}

func RegisterFieldValidator[T any, V string | bool | int](fieldName string, nullable bool, values ...V) {
	var t T
	parentType := reflect.TypeOf(t)

	if _, ok := validationRegistry[parentType]; !ok {
		validationRegistry[parentType] = []validationEntry{}
	}

	// The following checks run at initialization time,
	// it is impossible for them to panic if any tests pass.
	if parentType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("apijson: cannot initialize validator for non-struct %s", parentType.String()))
	}
	field, found := parentType.FieldByName(fieldName)
	if !found {
		panic(fmt.Sprintf("apijson: cannot initialize validator for unknown field %q in %s", fieldName, parentType.String()))
	}

	newEntry := validationEntry{field, nullable, make([]reflect.Value, len(values))}
	for i, value := range values {
		newEntry.legalValues[i] = reflect.ValueOf(value)
	}

	// Store the information necessary to create a validator, so that we can use it
	// lazily create the validator function when did.
	validationRegistry[parentType] = append(validationRegistry[parentType], newEntry)
}

// Enums are the only types which are validated
func typeValidator(t reflect.Type) validatorFunc {
	entry, ok := validationRegistry[t]
	if !ok {
		return nil
	}

	if fi, ok := validators.Load(t); ok {
		return fi.(validatorFunc)
	}

	fi, _ := validators.LoadOrStore(t, validatorFunc(func(v reflect.Value) exactness {
		return validateEnum(v, entry)
	}))
	return fi.(validatorFunc)
}

func validateEnum(v reflect.Value, entry []validationEntry) exactness {
	if v.Kind() != reflect.Struct {
		return loose
	}

	for _, check := range entry {
		field := v.FieldByIndex(check.field.Index)
		if !field.IsValid() {
			return loose
		}
		for _, opt := range check.legalValues {
			if field.Equal(opt) {
				return exact
			}
		}
	}

	return loose
}
