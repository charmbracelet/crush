package apijson

import (
	"reflect"

	"github.com/tidwall/gjson"
)

type UnionVariant struct {
	TypeFilter         gjson.Type
	DiscriminatorValue any
	Type               reflect.Type
}

var unionRegistry = map[reflect.Type]unionEntry{}
var unionVariants = map[reflect.Type]any{}

type unionEntry struct {
	discriminatorKey string
	variants         []UnionVariant
}

func Discriminator[T any](value any) UnionVariant {
	var zero T
	return UnionVariant{
		TypeFilter:         gjson.JSON,
		DiscriminatorValue: value,
		Type:               reflect.TypeOf(zero),
	}
}

func RegisterUnion[T any](discriminator string, variants ...UnionVariant) {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	unionRegistry[typ] = unionEntry{
		discriminatorKey: discriminator,
		variants:         variants,
	}
	for _, variant := range variants {
		unionVariants[variant.Type] = typ
	}
}

// Useful to wrap a union type to force it to use [apijson.UnmarshalJSON] since you cannot define an
// UnmarshalJSON function on the interface itself.
type UnionUnmarshaler[T any] struct {
	Value T
}

func (c *UnionUnmarshaler[T]) UnmarshalJSON(buf []byte) error {
	return UnmarshalRoot(buf, &c.Value)
}
