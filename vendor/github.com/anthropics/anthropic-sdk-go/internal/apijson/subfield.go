package apijson

import (
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"reflect"
)

func getSubField(root reflect.Value, index []int, name string) reflect.Value {
	strct := root.FieldByIndex(index[:len(index)-1])
	if !strct.IsValid() {
		panic("couldn't find encapsulating struct for field " + name)
	}
	meta := strct.FieldByName("JSON")
	if !meta.IsValid() {
		return reflect.Value{}
	}
	field := meta.FieldByName(name)
	if !field.IsValid() {
		return reflect.Value{}
	}
	return field
}

func setMetadataSubField(root reflect.Value, index []int, name string, meta Field) {
	target := getSubField(root, index, name)
	if !target.IsValid() {
		return
	}

	if target.Type() == reflect.TypeOf(meta) {
		target.Set(reflect.ValueOf(meta))
	} else if respMeta := meta.toRespField(); target.Type() == reflect.TypeOf(respMeta) {
		target.Set(reflect.ValueOf(respMeta))
	}
}

func setMetadataExtraFields(root reflect.Value, index []int, name string, metaExtras map[string]Field) {
	target := getSubField(root, index, name)
	if !target.IsValid() {
		return
	}

	if target.Type() == reflect.TypeOf(metaExtras) {
		target.Set(reflect.ValueOf(metaExtras))
		return
	}

	newMap := make(map[string]respjson.Field, len(metaExtras))
	if target.Type() == reflect.TypeOf(newMap) {
		for k, v := range metaExtras {
			newMap[k] = v.toRespField()
		}
		target.Set(reflect.ValueOf(newMap))
	}
}

func (f Field) toRespField() respjson.Field {
	if f.IsMissing() {
		return respjson.Field{}
	} else if f.IsNull() {
		return respjson.NewField("null")
	} else if f.IsInvalid() {
		return respjson.NewInvalidField(f.raw)
	} else {
		return respjson.NewField(f.raw)
	}
}
