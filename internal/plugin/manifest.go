package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type CommandSpec struct {
	Name       string            `yaml:"name"`
	Summary    string            `yaml:"summary"`
	Executor   string            `yaml:"executor"`    // shell|stdio
	Program    string            `yaml:"program"`     // for shell: the command line; for stdio: program path
	Args       []string          `yaml:"args"`        // optional default args
	Workdir    string            `yaml:"workdir"`     // optional
	Env        map[string]string `yaml:"env"`         // optional extra env
	UseStdio   bool              `yaml:"use_stdio"`   // when executor=stdio, true to pass JSON req and expect JSON resp
	LogCapture bool              `yaml:"log_capture"` // default true
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
	Entry    EntrySpec     `yaml:"entry"` // optional
	Config   ConfigSpec    `yaml:"config"`
	Commands []CommandSpec `yaml:"commands"` // one or more shell/stdio commands
	Expose   []string      `yaml:"expose"`   // shim names (usually one)
}

func LoadManifest(pluginDir string) (*PluginManifest, error) {
	p := filepath.Join(pluginDir, "manifest.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	var m PluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	if m.Name == "" {
		m.Name = filepath.Base(pluginDir)
	}
	if len(m.Expose) == 0 {
		return nil, fmt.Errorf("manifest: expose is required")
	}
	return &m, nil
}
