package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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

func IsJSON(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".json"
}

func LoadAny(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if IsJSON(path) {
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, err
		}
	}
	if m == nil {
		m = make(map[string]interface{})
	}
	return m, nil
}

func SaveAny(path string, m map[string]interface{}) error {
	var (
		out []byte
		err error
	)
	if IsJSON(path) {
		out, err = json.MarshalIndent(m, "", "  ")
	} else {
		out, err = yaml.Marshal(m)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
