package apijson

import (
	"errors"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"reflect"

	"github.com/tidwall/gjson"
)

var apiUnionType = reflect.TypeOf(param.APIUnion{})

func isStructUnion(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type == apiUnionType && t.Field(i).Anonymous {
			return true
		}
	}
	return false
}

func RegisterDiscriminatedUnion[T any](key string, mappings map[string]reflect.Type) {
	var t T
	entry := unionEntry{
		discriminatorKey: key,
		variants:         []UnionVariant{},
	}
	for k, typ := range mappings {
		entry.variants = append(entry.variants, UnionVariant{
			DiscriminatorValue: k,
			Type:               typ,
		})
	}
	unionRegistry[reflect.TypeOf(t)] = entry
}

func (d *decoderBuilder) newStructUnionDecoder(t reflect.Type) decoderFunc {
	type variantDecoder struct {
		decoder            decoderFunc
		field              reflect.StructField
		discriminatorValue any
	}

	variants := []variantDecoder{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Anonymous && field.Type == apiUnionType {
			continue
		}

		decoder := d.typeDecoder(field.Type)
		variants = append(variants, variantDecoder{
			decoder: decoder,
			field:   field,
		})
	}

	unionEntry, discriminated := unionRegistry[t]
	for _, unionVariant := range unionEntry.variants {
		for i := 0; i < len(variants); i++ {
			variant := &variants[i]
			if variant.field.Type.Elem() == unionVariant.Type {
				variant.discriminatorValue = unionVariant.DiscriminatorValue
				break
			}
		}
	}

	return func(n gjson.Result, v reflect.Value, state *decoderState) error {
		if discriminated && n.Type == gjson.JSON && len(unionEntry.discriminatorKey) != 0 {
			discriminator := n.Get(unionEntry.discriminatorKey).Value()
			for _, variant := range variants {
				if discriminator == variant.discriminatorValue {
					inner := v.FieldByIndex(variant.field.Index)
					return variant.decoder(n, inner, state)
				}
			}
			return errors.New("apijson: was not able to find discriminated union variant")
		}

		// Set bestExactness to worse than loose
		bestExactness := loose - 1
		bestVariant := -1
		for i, variant := range variants {
			// Pointers are used to discern JSON object variants from value variants
			if n.Type != gjson.JSON && variant.field.Type.Kind() == reflect.Ptr {
				continue
			}

			sub := decoderState{strict: state.strict, exactness: exact}
			inner := v.FieldByIndex(variant.field.Index)
			err := variant.decoder(n, inner, &sub)
			if err != nil {
				continue
			}
			if sub.exactness == exact {
				bestExactness = exact
				bestVariant = i
				break
			}
			if sub.exactness > bestExactness {
				bestExactness = sub.exactness
				bestVariant = i
			}
		}

		if bestExactness < loose {
			return errors.New("apijson: was not able to coerce type as union")
		}

		if guardStrict(state, bestExactness != exact) {
			return errors.New("apijson: was not able to coerce type as union strictly")
		}

		for i := 0; i < len(variants); i++ {
			if i == bestVariant {
				continue
			}
			v.FieldByIndex(variants[i].field.Index).SetZero()
		}

		return nil
	}
}

// newUnionDecoder returns a decoderFunc that deserializes into a union using an
// algorithm roughly similar to Pydantic's [smart algorithm].
//
// Conceptually this is equivalent to choosing the best schema based on how 'exact'
// the deserialization is for each of the schemas.
//
// If there is a tie in the level of exactness, then the tie is broken
// left-to-right.
//
// [smart algorithm]: https://docs.pydantic.dev/latest/concepts/unions/#smart-mode
func (d *decoderBuilder) newUnionDecoder(t reflect.Type) decoderFunc {
	unionEntry, ok := unionRegistry[t]
	if !ok {
		panic("apijson: couldn't find union of type " + t.String() + " in union registry")
	}
	decoders := []decoderFunc{}
	for _, variant := range unionEntry.variants {
		decoder := d.typeDecoder(variant.Type)
		decoders = append(decoders, decoder)
	}
	return func(n gjson.Result, v reflect.Value, state *decoderState) error {
		// If there is a discriminator match, circumvent the exactness logic entirely
		for idx, variant := range unionEntry.variants {
			decoder := decoders[idx]
			if variant.TypeFilter != n.Type {
				continue
			}

			if len(unionEntry.discriminatorKey) != 0 {
				discriminatorValue := n.Get(unionEntry.discriminatorKey).Value()
				if discriminatorValue == variant.DiscriminatorValue {
					inner := reflect.New(variant.Type).Elem()
					err := decoder(n, inner, state)
					v.Set(inner)
					return err
				}
			}
		}

		// Set bestExactness to worse than loose
		bestExactness := loose - 1
		for idx, variant := range unionEntry.variants {
			decoder := decoders[idx]
			if variant.TypeFilter != n.Type {
				continue
			}
			sub := decoderState{strict: state.strict, exactness: exact}
			inner := reflect.New(variant.Type).Elem()
			err := decoder(n, inner, &sub)
			if err != nil {
				continue
			}
			if sub.exactness == exact {
				v.Set(inner)
				return nil
			}
			if sub.exactness > bestExactness {
				v.Set(inner)
				bestExactness = sub.exactness
			}
		}

		if bestExactness < loose {
			return errors.New("apijson: was not able to coerce type as union")
		}

		if guardStrict(state, bestExactness != exact) {
			return errors.New("apijson: was not able to coerce type as union strictly")
		}

		return nil
	}
}
