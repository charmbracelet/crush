package apijson

import (
	"errors"
	"github.com/openai/openai-go/packages/param"
	"reflect"

	"github.com/tidwall/gjson"
)

func isEmbeddedUnion(t reflect.Type) bool {
	var apiunion param.APIUnion
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type == reflect.TypeOf(apiunion) && t.Field(i).Anonymous {
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

func (d *decoderBuilder) newEmbeddedUnionDecoder(t reflect.Type) decoderFunc {
	decoders := []decoderFunc{}

	for i := 0; i < t.NumField(); i++ {
		variant := t.Field(i)
		decoder := d.typeDecoder(variant.Type)
		decoders = append(decoders, decoder)
	}

	unionEntry := unionEntry{
		variants: []UnionVariant{},
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
