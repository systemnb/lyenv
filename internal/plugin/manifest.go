package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type CommandSpec struct {
	Name       string            `yaml:"name"`
	Summary    string            `yaml:"summary"`
	Executor   string            `yaml:"executor"` // shell|stdio
	Program    string            `yaml:"program"`  // shell: command line; stdio: program path
	Args       []string          `yaml:"args"`
	Workdir    string            `yaml:"workdir"`
	Env        map[string]string `yaml:"env"`
	UseStdio   bool              `yaml:"use_stdio"`
	LogCapture bool              `yaml:"log_capture"`
}

type EntrySpec struct {
	Type string   `yaml:"type"` // optional: stdio
	Path string   `yaml:"path"`
	Args []string `yaml:"args"`
}

type ConfigSpec struct {
	Namespace string `yaml:"namespace"`
	LocalFile string `yaml:"local_file"`
	StateFile string `yaml:"state_file"`
}

type PluginManifest struct {
	Name     string        `yaml:"name"`
	Version  string        `yaml:"version"`
	Entry    EntrySpec     `yaml:"entry"`
	Config   ConfigSpec    `yaml:"config"`
	Commands []CommandSpec `yaml:"commands"`
	Expose   []string      `yaml:"expose"`
}

func LoadManifest(pluginDir string) (*PluginManifest, error) {
	candidates := []string{
		filepath.Join(pluginDir, "manifest.yaml"),
		filepath.Join(pluginDir, "manifest.yml"),
		filepath.Join(pluginDir, "manifest.json"),
	}
	var path string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("failed to read manifest: not found (manifest.yaml|manifest.yml|manifest.json)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	var m PluginManifest
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("invalid manifest (json): %w", err)
		}
	default:
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("invalid manifest (yaml): %w", err)
		}
	}
	if m.Name == "" {
		m.Name = filepath.Base(pluginDir)
	}
	if len(m.Expose) == 0 {
		return nil, fmt.Errorf("manifest: expose is required")
	}
	return &m, nil
}
