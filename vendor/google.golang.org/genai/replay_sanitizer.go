// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genai

import (
	"encoding/base64"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// sanitizeMapWithSourceType sanitizes byte fields within a map based on the provided source type.
// It converts byte fields encoded with URL Base64 to standard Base64 encoding to prevent SDK unmarshal error.
//
// Args:
//
//	t: The testing.T instance for reporting errors.
//	sourceType: The reflect.Type of the source struct. This is used to determine the paths to byte fields.
//	m: The map containing the data to sanitize.  The map will be modified in place.
func sanitizeMapWithSourceType(t *testing.T, sourceType reflect.Type, m any) {
	t.Helper()
	paths := make([]string, 0)

	st := sourceType
	if sourceType.Kind() == reflect.Slice {
		st = sourceType.Elem()
	}
	visitedTypes := make(map[string]bool)
	if err := getFieldPath(st, reflect.TypeOf([]byte{}), &paths, "", visitedTypes, false); err != nil {
		t.Fatal(err)
	}

	stdBase64Handler := func(data any, path string) any {
		s := data.(string)
		b, err := base64.URLEncoding.DecodeString(s)
		if err != nil {
			b, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				t.Errorf("invalid base64 string %s at path %s", s, path)
			}
		}
		return base64.StdEncoding.EncodeToString(b)
	}

	for _, path := range paths {
		if sourceType.Kind() == reflect.Slice {
			data := m.([]any)
			for i := 0; i < len(data); i++ {
				sanitizeMapByPath(data[i], path, stdBase64Handler, false)
			}
		} else {
			sanitizeMapByPath(m.(map[string]any), path, stdBase64Handler, false)
		}
	}
}

// sanitizeMapByPath sanitizes a value within a nested map structure based on the given path.
// It applies the provided sanitizer function to the value found at the specified path.
//
// Args:
//
//	data: The map containing the data to sanitize. This can be a nested map structure. The map may be modified in place.
//	path: The path to the value to sanitize. This is a dot-separated string, where each component represents a key in the map.
//	      Array elements can be accessed using the "[]" prefix, e.g., "[]sliceField.fieldName".
//	sanitizer: The function to apply to the value found at the specified path. The function should take the value and the path as input and return the sanitized value.
//	debug: A boolean indicating whether debug logging should be enabled.
func sanitizeMapByPath(data any, path string, sanitizer func(data any, path string) any, debug bool) {
	if _, ok := data.(map[string]any); !ok {
		if debug {
			log.Println("data is not map type", data, path)
		}
		return
	}
	m := data.(map[string]any)

	keys := strings.Split(path, ".")
	key := keys[0]

	// Handle path not exists.
	if strings.HasPrefix(key, "[]") {
		if _, ok := m[key[2:]]; !ok {
			if debug {
				log.Println("path doesn't exist", data, path)
			}
			return
		}
	} else if _, ok := m[key]; !ok {
		if debug {
			log.Println("path doesn't exist", data, path)
		}
		return
	}

	// We are at the last component of the path.
	if strings.HasPrefix(key, "[]") && len(keys) == 1 {
		items := []any{}
		v := m[key[2:]]
		if reflect.ValueOf(v).Type().Kind() != reflect.Slice {
			if debug {
				log.Println("data is not slice type as the path denoted", data, path)
			}
			return
		}
		for i := 0; i < reflect.ValueOf(v).Len(); i++ {
			items = append(items, sanitizer(reflect.ValueOf(v).Index(i).Interface(), key))
		}
		m[key[2:]] = items
		return
	} else if len(keys) == 1 {
		m[key] = sanitizer(m[key], path)
		return
	}

	if strings.HasPrefix(key, "[]") {
		v := m[key[2:]]
		if reflect.ValueOf(v).Type().Kind() != reflect.Slice {
			if debug {
				log.Println("data is not slice type as the path denoted", data, path)
			}
			return
		}
		s := reflect.ValueOf(v)
		for i := 0; i < s.Len(); i++ {
			element := s.Index(i).Interface()
			sanitizeMapByPath(element, strings.Join(keys[1:], "."), sanitizer, debug)
		}
	} else {
		sanitizeMapByPath(m[key], strings.Join(keys[1:], "."), sanitizer, debug)
	}
}

// convertFloat64ToString recursively converts float64 values within a map[string]any to strings.
func convertFloat64ToString(data map[string]any) map[string]any {
	for key, value := range data {
		switch v := value.(type) {
		case float64:
			// Convert float64 to string
			data[key] = strconv.FormatFloat(v, 'f', 5, 64)
		case map[string]any:
			// Recursively process nested maps
			data[key] = convertFloat64ToString(v)
		case []any:
			// Recursively process slices
			data[key] = convertSliceFloat64ToString(v)
		}
	}
	return data
}

// convertSliceFloat64ToString recursively converts float64 values within a []any to strings.
func convertSliceFloat64ToString(data []any) []any {
	for i, value := range data {
		switch v := value.(type) {
		case float64:
			// Convert float64 to string
			data[i] = strconv.FormatFloat(v, 'f', -1, 64)
		case map[string]any:
			// Recursively process nested maps
			data[i] = convertFloat64ToString(v)
		case []any:
			// Recursively process nested slices
			data[i] = convertSliceFloat64ToString(v)
		}
	}
	return data
}

// getFieldPath retrieves the paths to all fields within a nested struct that match a given target type.
// It uses reflection to traverse the struct and its nested fields.
//
// Args:
//
//		sourceType: The reflect.Type of the source struct to traverse.
//		targetType: The reflect.Type of the target field to search for.
//		outputPaths: A pointer to a string slice where the resulting paths will be stored.
//		prefix: The current path prefix, used during recursive calls.
//	 	visitedTypes: Serves to prevent infinite recursion when dealing with recursive data structures
//		debug: A boolean indicating whether debug logging should be enabled.
//
// Returns:
//
//	An error if the targetType is a pointer or a struct.
func getFieldPath(sourceType reflect.Type, targetType reflect.Type, outputPaths *[]string, prefix string, visitedTypes map[string]bool, debug bool) error {
	if targetType.Kind() == reflect.Ptr {
		return fmt.Errorf("targetType cannot be a pointer")
	}
	if targetType.Kind() == reflect.Struct {
		return fmt.Errorf("targetType cannot be a struct")
	}
	if sourceType.Kind() == reflect.Ptr {
		_ = getFieldPath(sourceType.Elem(), targetType, outputPaths, prefix, visitedTypes, debug) // handle pointer nested field
	} else if sourceType.Kind() == reflect.Struct {
		for i := 0; i < sourceType.NumField(); i++ {
			field := sourceType.Field(i)
			if debug {
				log.Println("field name:", field.Name, "field type:", field.Type.String(), "field tag:", field.Tag.Get("json"))
			}
			if visitedTypes[sourceType.String()+"."+fieldJSONName(field)] {
				continue
			}
			visitedTypes[sourceType.String()+"."+fieldJSONName(field)] = true

			if field.Type == targetType {
				*outputPaths = append(*outputPaths, prefix+fieldJSONName(field))
			} else if field.Type.Kind() == reflect.Struct {
				_ = getFieldPath(field.Type, targetType, outputPaths, prefix+fieldJSONName(field)+".", visitedTypes, debug)
			} else if field.Type.Kind() == reflect.Ptr {
				_ = getFieldPath(field.Type.Elem(), targetType, outputPaths, prefix+fieldJSONName(field)+".", visitedTypes, debug)
			} else if field.Type.Kind() == reflect.Slice {
				elementType := field.Type.Elem() // Get the type of elements in the array
				_ = getFieldPath(elementType, targetType, outputPaths, prefix+"[]"+fieldJSONName(field)+".", visitedTypes, debug)
			}
			visitedTypes[sourceType.String()+"."+fieldJSONName(field)] = false
			// TODO(b/390425822): support map type.
		}
		if debug {
			log.Printf("field of type %s not found\n", targetType.String())
		}
	}
	if debug {
		log.Printf("field of type %s not found\n", targetType.String())
	}
	return nil
}

func fieldJSONName(field reflect.StructField) string {
	return strings.Split(field.Tag.Get("json"), ",")[0]
}
