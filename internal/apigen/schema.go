package apigen

import (
	"reflect"
	"strings"
	"time"
)

// Schema is a JSON-Schema-shaped description of a Go value, derived by
// reflection. It supports the subset of JSON Schema used by the docs
// page and the OpenAPI document.
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
}

var (
	timeType  = reflect.TypeOf(time.Time{})
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

// SchemaOf derives a [Schema] from the type of v, honoring encoding/json
// tags: field names come from json tags, and fields marked omitempty are
// not required.
//
// Structs from the same package as v's type are expanded recursively;
// structs from other packages (and true type cycles) are rendered as
// free-form objects described by their type name, which keeps schemas
// for wide external types honest without inventing shape.
func SchemaOf(v any) *Schema {
	t := reflect.TypeOf(v)
	if t == nil {
		return &Schema{}
	}
	return schemaOf(t, rootPkg(t), map[reflect.Type]bool{})
}

// rootPkg returns the package path of the (possibly pointer) type v's
// named root, used to bound struct expansion to one package.
func rootPkg(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath()
}

func schemaOf(t reflect.Type, pkg string, expanding map[reflect.Type]bool) *Schema {
	switch t {
	case timeType:
		return &Schema{Type: "string", Format: "date-time"}
	case errorType:
		return &Schema{Type: "string"}
	}

	switch t.Kind() {
	case reflect.Pointer:
		return schemaOf(t.Elem(), pkg, expanding)
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Slice, reflect.Array:
		return &Schema{Type: "array", Items: schemaOf(t.Elem(), pkg, expanding)}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return &Schema{Type: "object"}
		}
		return &Schema{Type: "object", AdditionalProperties: schemaOf(t.Elem(), pkg, expanding)}
	case reflect.Struct:
		return structSchema(t, pkg, expanding)
	default:
		// Interfaces (including any) and anything else: free-form.
		return &Schema{}
	}
}

func structSchema(t reflect.Type, pkg string, expanding map[reflect.Type]bool) *Schema {
	if t.PkgPath() != pkg {
		return &Schema{Type: "object", Description: t.String()}
	}
	if expanding[t] {
		return &Schema{Type: "object", Description: t.String()}
	}
	expanding[t] = true
	defer delete(expanding, t)

	s := &Schema{Type: "object"}
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			// Embedded structs contribute their fields, per
			// encoding/json semantics, even when the embedded
			// type itself is unexported.
			emb := schemaOf(f.Type, pkg, expanding)
			for k, v := range emb.Properties {
				if s.Properties == nil {
					s.Properties = map[string]*Schema{}
				}
				s.Properties[k] = v
			}
			s.Required = append(s.Required, emb.Required...)
			continue
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if s.Properties == nil {
			s.Properties = map[string]*Schema{}
		}
		s.Properties[name] = schemaOf(f.Type, pkg, expanding)
		if !strings.Contains(opts, "omitempty") {
			s.Required = append(s.Required, name)
		}
	}
	return s
}
