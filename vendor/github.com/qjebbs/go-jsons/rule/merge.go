// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package rule

import "github.com/qjebbs/go-jsons/merge"

func mergeByFields(s []interface{}, fields []Field) ([]interface{}, error) {
	if len(s) == 0 || len(fields) == 0 {
		return s, nil
	}
	// from: [a,"",b,"",a,"",b,""]
	// to: [a,"",b,"",merged,"",merged,""]
	merged := &struct{}{}
	for i, item1 := range s {
		map1, ok := item1.(map[string]interface{})
		if !ok {
			continue
		}
		tags1 := getTags(map1, fields)
		if len(tags1) == 0 {
			continue
		}
		for j := i + 1; j < len(s); j++ {
			map2, ok := s[j].(map[string]interface{})
			if !ok {
				continue
			}
			tags2 := getTags(map2, fields)
			if !matchTags(tags1, tags2) {
				continue
			}
			s[j] = merged
			err := merge.Maps(map1, map2)
			if err != nil {
				return nil, err
			}
		}
	}
	// remove merged
	ns := make([]interface{}, 0)
	for _, item := range s {
		if item == merged {
			continue
		}
		ns = append(ns, item)
	}
	return ns, nil
}

func matchTags(a, b []string) bool {
	for _, tag1 := range a {
		for _, tag2 := range b {
			if tag1 == tag2 {
				return true
			}
		}
	}
	return false
}

func getTags(v map[string]interface{}, fields []Field) []string {
	tags := make([]string, 0, len(fields))
	for _, field := range fields {
		value, ok := v[field.Key]
		if !ok {
			continue
		}
		if tag, ok := value.(string); ok && tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
