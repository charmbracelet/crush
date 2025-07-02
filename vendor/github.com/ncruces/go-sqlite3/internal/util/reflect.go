package util

import "reflect"

func ReflectType(v reflect.Value) reflect.Type {
	if v.Kind() != reflect.Invalid {
		return v.Type()
	}
	return nil
}
