// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package rule

// FieldType is the field type
type FieldType int

// FieldTypes
const (
	FieldTypeOrder FieldType = iota
	FieldTypeMerge
)

// Field is the field for rules
type Field struct {
	Type   FieldType
	Key    string // field key
	Remove bool   // whether to remove the field after rules applied
}

// OrderBy is the order by field for slice sort rule
func OrderBy(key string) Field {
	return Field{
		Type:   FieldTypeOrder,
		Key:    key,
		Remove: false,
	}
}

// MergeBy is the merge by field for slice sort rule
func MergeBy(key string) Field {
	return Field{
		Type:   FieldTypeMerge,
		Key:    key,
		Remove: false,
	}
}

// OrderByAndRemove is the order by field for slice merge rule
func OrderByAndRemove(key string) Field {
	return Field{
		Type:   FieldTypeOrder,
		Key:    key,
		Remove: true,
	}
}

// MergeByAndRemove is the merge by field for slice merge rule
func MergeByAndRemove(key string) Field {
	return Field{
		Type:   FieldTypeMerge,
		Key:    key,
		Remove: true,
	}
}
