package respjson

// A Field provides metadata to indicate the presence of a value.
//
// Use [Field.Valid] to check if an optional value was null or omitted.
//
// A Field will always occur in the following structure, where it
// mirrors the original field in it's parent struct:
//
//	type ExampleObject struct {
//		Foo bool	`json:"foo"`
//		Bar int		`json:"bar"`
//		// ...
//
//		// JSON provides metadata about the object.
//		JSON struct {
//			Foo Field
//			Bar Field
//			// ...
//		} `json:"-"`
//	}
//
// To differentiate a "nullish" value from the zero value,
// use the [Field.Valid] method.
//
//	if !example.JSON.Foo.Valid() {
//		println("Foo is null or omitted")
//	}
//
//	if example.Foo {
//		println("Foo is true")
//	} else {
//		println("Foo is false")
//	}
//
// To differentiate if a field was omitted or the JSON value "null",
// use the [Field.Raw] method.
//
//	if example.JSON.Foo.Raw() == "null" {
//		println("Foo is null")
//	}
//
//	if example.JSON.Foo.Raw() == "" {
//		println("Foo was omitted")
//	}
//
// Otherwise, if the field was invalid and couldn't be marshalled successfully,
// [Field.Valid] will be false and [Field.Raw] will not be empty.
type Field struct {
	status
	raw string
}

const (
	omitted status = iota
	null
	invalid
	valid
)

type status int8

// Valid returns true if the parent field was set.
// Valid returns false if the value doesn't exist, is JSON null, or
// is an unexpected type.
func (j Field) Valid() bool { return j.status > invalid }

const Null string = "null"
const Omitted string = ""

// Returns the raw JSON value of the field.
func (j Field) Raw() string {
	if j.status == omitted {
		return ""
	}
	return j.raw
}

func NewField(raw string) Field {
	if raw == "null" {
		return Field{status: null, raw: Null}
	}
	return Field{status: valid, raw: raw}
}

func NewInvalidField(raw string) Field {
	return Field{status: invalid, raw: raw}
}
