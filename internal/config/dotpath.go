package config

import "strings"

func GetByPath(m map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var cur interface{} = m
	for _, p := range parts {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, exists := obj[p]
		if !exists {
			return nil, false
		}
		cur = val
	}
	return cur, true
}

func SetByPath(m map[string]interface{}, path string, val interface{}) {
	parts := strings.Split(path, ".")
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = val
			return
		}
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			cur[p] = next
		}
		cur = next
	}
}
