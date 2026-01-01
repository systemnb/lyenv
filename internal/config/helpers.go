package config

func GetString(m map[string]interface{}, key string) string {
	v, ok := GetByPath(m, key)
	if !ok || v == nil {
		return ""
	}
	if s, ok2 := v.(string); ok2 {
		return s
	}
	return ""
}
