package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"lyenv/internal/env"

	"gopkg.in/yaml.v3"
)

func ConfigSetWithType(envDir, cfgFile, key, rawValue, typeOpt string) error {
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := LoadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	val, err := ParseWithType(rawValue, typeOpt)
	if err != nil {
		return err
	}
	SetByPath(m, key, val)
	if err := SaveYAML(cfgPath, m); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

func ConfigGet(envDir, cfgFile, key string) (string, error) {
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := LoadYAML(cfgPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}
	val, ok := GetByPath(m, key)
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	switch v := val.(type) {
	case string:
		return v + "\n", nil
	case bool:
		if v {
			return "true\n", nil
		}
		return "false\n", nil
	case int, int64, float64, float32:
		return fmt.Sprintf("%v\n", v), nil
	default:
		out, err := yaml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to serialize value: %w", err)
		}
		return string(out), nil
	}
}

func ConfigDump(envDir, cfgFile, key, outFile string) error {
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := LoadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var toWrite interface{} = m
	if key != "" {
		val, ok := GetByPath(m, key)
		if !ok {
			return fmt.Errorf("key not found: %s", key)
		}
		toWrite = val
	}
	if err := SaveAny(outFile, toWrite); err != nil {
		return fmt.Errorf("failed to write dump file: %w", err)
	}
	return nil
}

func ConfigLoadWithStrategy(envDir, cfgFile, srcFile string, strategy MergeStrategy) error {
	cfgPath := filepath.Join(envDir, cfgFile)
	base, err := LoadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read base config: %w", err)
	}
	overlay, err := LoadAny(srcFile)
	if err != nil {
		return fmt.Errorf("failed to read source config: %w", err)
	}

	merged := MergeMapWithStrategy(base, overlay, strategy)
	if err := SaveYAML(cfgPath, merged); err != nil {
		return fmt.Errorf("failed to write merged config: %w", err)
	}
	return nil
}

// Import a value from JSON file and write into lyenv.yaml, supporting type and merge strategy.
func ConfigImportJSON(envDir, cfgFile, jsonFile, jsonKey, destKey, typeOpt string, strategy MergeStrategy, inputOn bool) error {
	// Load JSON
	jf, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}
	var jm interface{}
	if err := json.Unmarshal(jf, &jm); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	// Extract value by dot path
	jmap, ok := jm.(map[string]interface{})
	var jval interface{}
	var found bool
	if ok {
		jval, found = GetByPath(jmap, jsonKey)
	}
	if !found || jval == nil {
		if inputOn {
			// Prompt user for a value (raw text)
			prompt := fmt.Sprintf("Enter value for JSON key '%s' (type=%s, or 'json'):", jsonKey, NonEmpty(typeOpt, "auto"))
			in, err := env.PromptLine(prompt)
			if err != nil {
				return fmt.Errorf("input failed: %v", err)
			}
			// Parse with type
			parsed, err := ParseWithType(in, typeOpt)
			if err != nil {
				return err
			}
			jval = parsed
		} else {
			return fmt.Errorf("JSON key not found or empty: %s", jsonKey)
		}
	}

	// Coerce type if requested
	if typeOpt != "" {
		// If original is a complex object and typeOpt != json, we still allow coercion for scalars
		raw := ToJSONStringIfNeeded(jval)
		parsed, err := ParseWithType(raw, typeOpt)
		if err != nil {
			return err
		}
		jval = parsed
	}

	// Load YAML config
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := LoadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// If destination key exists and both sides are composite, apply merge strategy.
	existing, exists := GetByPath(m, destKey)
	if exists {
		// Merge behavior:
		em, eIsMap := existing.(map[string]interface{})
		jm2, jIsMap := jval.(map[string]interface{})
		ea, eIsArr := ToIfaceSlice(existing)
		ja, jIsArr := ToIfaceSlice(jval)

		if eIsMap && jIsMap {
			merged := MergeMapWithStrategy(em, jm2, strategy)
			SetByPath(m, destKey, merged)
		} else if eIsArr && jIsArr {
			switch strategy {
			case MergeAppend:
				SetByPath(m, destKey, append(ea, ja...))
			case MergeKeep:
				// keep base
				// no-op
			case MergeOverride:
				SetByPath(m, destKey, jval)
			}
		} else {
			// scalar or type mismatch -> follow strategy
			switch strategy {
			case MergeKeep:
				// keep existing
			case MergeOverride, MergeAppend:
				// treat append as override for scalars/mismatch
				SetByPath(m, destKey, jval)
			}
		}
	} else {
		// Not exists -> set
		SetByPath(m, destKey, jval)
	}

	if err := SaveYAML(cfgPath, m); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// Import a value from YAML file and write into lyenv.yaml, supporting type and merge strategy.
// Note: The YAML root must be a mapping for dot path traversal; list root is not supported in MVP.
func ConfigImportYAML(envDir, cfgFile, yamlFile, yamlKey, destKey, typeOpt string, strategy MergeStrategy, inputOn bool) error {
	// Load YAML (as map)
	ymap, err := LoadYAML(yamlFile)
	if err != nil {
		return fmt.Errorf("failed to read YAML file: %w", err)
	}

	// Extract value by dot path
	yval, found := GetByPath(ymap, yamlKey)
	if !found || yval == nil {
		if inputOn {
			prompt := fmt.Sprintf("Enter value for YAML key '%s' (type=%s, or 'json'):", yamlKey, NonEmpty(typeOpt, "auto"))
			in, err := env.PromptLine(prompt)
			if err != nil {
				return fmt.Errorf("input failed: %v", err)
			}
			parsed, err := ParseWithType(in, typeOpt)
			if err != nil {
				return err
			}
			yval = parsed
		} else {
			return fmt.Errorf("YAML key not found or empty: %s", yamlKey)
		}
	}

	// Coerce type if requested
	if typeOpt != "" {
		raw := ToJSONStringIfNeeded(yval)
		parsed, err := ParseWithType(raw, typeOpt)
		if err != nil {
			return err
		}
		yval = parsed
	}

	// Load lyenv YAML config
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := LoadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Merge into destination key according to strategy
	existing, exists := GetByPath(m, destKey)
	if exists {
		em, eIsMap := existing.(map[string]interface{})
		ym, yIsMap := yval.(map[string]interface{})
		ea, eIsArr := ToIfaceSlice(existing)
		ya, yIsArr := ToIfaceSlice(yval)

		if eIsMap && yIsMap {
			merged := MergeMapWithStrategy(em, ym, strategy)
			SetByPath(m, destKey, merged)
		} else if eIsArr && yIsArr {
			switch strategy {
			case MergeAppend:
				SetByPath(m, destKey, append(ea, ya...))
			case MergeKeep:
				// keep base
			case MergeOverride:
				SetByPath(m, destKey, yval)
			}
		} else {
			// scalar or mismatched types
			switch strategy {
			case MergeKeep:
				// keep existing
			case MergeOverride, MergeAppend:
				SetByPath(m, destKey, yval)
			}
		}
	} else {
		// Not exists -> set
		SetByPath(m, destKey, yval)
	}

	if err := SaveYAML(cfgPath, m); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}
