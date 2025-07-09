// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package rule

import (
	"math"
	"sort"
)

type meta struct {
	index int
	order float64
	value interface{}
}

// sortByFields sort slice elements by specified fields
func sortByFields(slice []interface{}, fields []Field) {
	if len(slice) == 0 || len(fields) == 0 {
		return
	}
	metas := make([]meta, len(slice))
	for i, v := range slice {
		metas[i] = meta{
			index: i,
			order: getOrder(v, fields),
			value: v,
		}
	}
	sort.Slice(
		metas,
		func(i, j int) bool {
			if metas[i].order != metas[j].order {
				return metas[i].order < metas[j].order
			}
			return metas[i].index < metas[j].index
		},
	)
	for i, m := range metas {
		slice[i] = m.value
	}
}

func getOrder(v interface{}, fields []Field) float64 {
	m, ok := v.(map[string]interface{})
	if !ok {
		return 0
	}
	hasField := false
	min := math.Inf(1)
	for _, field := range fields {
		value, ok := m[field.Key]
		if !ok || value == nil {
			continue
		}
		hasField = true
		var num float64
		switch v := value.(type) {
		case float64:
			num = v
		case float32:
			num = float64(v)
		case int:
			num = float64(v)
		case int8:
			num = float64(v)
		case int16:
			num = float64(v)
		case int32:
			num = float64(v)
		case int64:
			num = float64(v)
		case uint:
			num = float64(v)
		case uint8:
			num = float64(v)
		case uint16:
			num = float64(v)
		case uint32:
			num = float64(v)
		case uint64:
			num = float64(v)
		default:
			num = 0
		}
		if num < min {
			min = num
		}
	}
	if !hasField {
		return 0
	}
	return min
}
