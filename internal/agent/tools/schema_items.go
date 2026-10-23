package tools

import (
	"reflect"
	"strings"

	"charm.land/fantasy/schema"
)

// schemaWithItemDescriptions generates a tool schema for a params struct and
// fills in item descriptions declared via the items_description tag.
//
// It exists because the schema generator in charm.land/fantasy only applies the
// plain description tag to a field's own schema, so array element types are
// emitted without a description. Strict OpenAI-compatible providers reject the
// request in that case.
//
// The returned struct is a copy; callers must not mutate its fields.
func schemaWithItemDescriptions(t reflect.Type) schema.Schema {
	s := schema.Generate(t)
	applyItemDescriptions(t, &s)
	return s
}

// applyItemDescriptions copies items_description tag values onto the item
// schema of every array property, recursing into nested structs.
func applyItemDescriptions(t reflect.Type, s *schema.Schema) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}

		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			name = toSnakeCaseField(field.Name)
		}

		prop, ok := s.Properties[name]
		if !ok || prop == nil {
			continue
		}

		ft := field.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		switch ft.Kind() {
		case reflect.Slice, reflect.Array:
			if desc := field.Tag.Get("items_description"); desc != "" && prop.Items != nil {
				prop.Items.Description = desc
			}
			if prop.Items != nil {
				applyItemDescriptions(ft.Elem(), prop.Items)
			}
		case reflect.Struct:
			applyItemDescriptions(ft, prop)
		}
	}
}

// toSnakeCaseField mirrors the generator's naming for untagged fields so
// lookups line up with the property names it produces.
func toSnakeCaseField(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
