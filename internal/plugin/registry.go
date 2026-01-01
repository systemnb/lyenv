package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type InstalledPlugin struct {
	// Manifest name inside plugin (logical name)
	Name string `yaml:"name"`

	// Directory name under plugins/ (physical install name)
	InstallName string `yaml:"install_name"`

	Version     string    `yaml:"version"`
	Source      string    `yaml:"source"`
	Ref         string    `yaml:"ref"`
	Shims       []string  `yaml:"shims"`
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

// Helper to remove one plugin by InstallName (physical)
func UnregisterByInstallName(envDir, installName string) error {
	r, err := LoadRegistry(envDir)
	if err != nil {
		return err
	}
	out := make([]InstalledPlugin, 0, len(r.Plugins))
	for _, p := range r.Plugins {
		if p.InstallName != installName {
			out = append(out, p)
		}
	}
	r.Plugins = out
	return SaveRegistry(envDir, r)
}

// Helper to get a record by InstallName
func GetByInstallName(envDir, installName string) (*InstalledPlugin, error) {
	r, err := LoadRegistry(envDir)
	if err != nil {
		return nil, err
	}
	for _, p := range r.Plugins {
		if p.InstallName == installName {
			cp := p
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("plugin not found: %s", installName)
}
