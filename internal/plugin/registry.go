package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type InstalledPlugin struct {
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version"`
	Source      string    `yaml:"source"` // local|git|archive|url
	Ref         string    `yaml:"ref"`
	InstalledAt time.Time `yaml:"installed_at"`
}

type Registry struct {
	Plugins []InstalledPlugin `yaml:"plugins"`
}

func registryPath(envDir string) string {
	return filepath.Join(envDir, ".lyenv", "registry", "installed.yaml")
}

func LoadRegistry(envDir string) (*Registry, error) {
	p := registryPath(envDir)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Plugins: []InstalledPlugin{}}, nil
		}
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("invalid registry: %w", err)
	}
	return &r, nil
}

func SaveRegistry(envDir string, r *Registry) error {
	p := registryPath(envDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0o644)
}

func RegisterInstall(envDir string, ip InstalledPlugin) error {
	r, err := LoadRegistry(envDir)
	if err != nil {
		return err
	}
	found := false
	for i := range r.Plugins {
		if r.Plugins[i].Name == ip.Name {
			r.Plugins[i] = ip
			found = true
			break
		}
	}
	if !found {
		r.Plugins = append(r.Plugins, ip)
	}
	return SaveRegistry(envDir, r)
}
