package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

func LoadYAML(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]interface{})
	}
	return m, nil
}

func SaveYAML(path string, m map[string]interface{}) error {
	out, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
