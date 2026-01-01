package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ParseWithType(raw string, typeOpt string) (interface{}, error) {
	switch strings.ToLower(strings.TrimSpace(typeOpt)) {
	case "":
		return ParseScalar(raw), nil
	case "string":
		return raw, nil
	case "int":
		i, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int: %v", err)
		}
		return i, nil
	case "float":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float: %v", err)
		}
		return f, nil
	case "bool":
		l := strings.ToLower(strings.TrimSpace(raw))
		if l == "true" || l == "1" || l == "yes" {
			return true, nil
		}
		if l == "false" || l == "0" || l == "no" {
			return false, nil
		}
		return nil, fmt.Errorf("invalid bool: %q (accepted: true/false/1/0/yes/no)", raw)
	case "json":
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, fmt.Errorf("invalid JSON: %v", err)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type: %s", typeOpt)
	}
}

func ParseScalar(s string) interface{} {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func ToJSONStringIfNeeded(v interface{}) string {
	switch vv := v.(type) {
	case string:
		return vv
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
