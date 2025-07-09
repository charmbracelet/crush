package paramutil

import (
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
)

func AddrIfPresent[T comparable](v param.Opt[T]) *T {
	if v.Valid() {
		return &v.Value
	}
	return nil
}

func ToOpt[T comparable](v T, meta respjson.Field) param.Opt[T] {
	if meta.Valid() {
		return param.NewOpt(v)
	} else if meta.Raw() == respjson.Null {
		return param.Null[T]()
	}
	return param.Opt[T]{}
}

// Checks if the value is not omitted and not null
func Valid(v param.ParamStruct) bool {
	if ovr, ok := v.Overrides(); ok {
		return ovr != nil
	}
	return !param.IsNull(v) && !param.IsOmitted(v)
}
