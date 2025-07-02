package resp

// A Field contains metadata about a JSON field that was
// unmarshalled from a response.
//
// To check if the field was unmarshalled successfully, use the [Field.IsPresent] method.
//
// Use the [Field.IsExplicitNull] method to check if the JSON value is "null".
//
// If the [Field.Raw] is the empty string, then the field was omitted.
//
// Otherwise, if the field was invalid and couldn't be marshalled successfully, [Field.IsPresent] will be false,
// and [Field.Raw] will not be empty.
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

// IsPresent returns true if the field was unmarshalled successfully.
// If IsPresent is false, the field was either omitted, the JSON value "null", or an unexpected type.
func (j Field) IsPresent() bool { return j.status > invalid }

// Returns true if the field is the JSON value "null".
func (j Field) IsExplicitNull() bool { return j.status == null }

// Returns the raw JSON value of the field.
func (j Field) Raw() string {
	if j.status == omitted {
		return ""
	}
	return j.raw
}

func NewValidField(raw string) Field {
	if raw == "null" {
		return NewNullField()
	}
	return Field{raw: raw, status: valid}
}

func NewNullField() Field {
	return Field{status: null}
}

func NewInvalidField(raw string) Field {
	return Field{status: invalid, raw: raw}
}
