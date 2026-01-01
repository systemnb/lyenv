package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PluginRemove(envDir, name string) error {
	pluginDir := filepath.Join(envDir, "plugins", name)
	man, err := LoadManifest(pluginDir)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
			_ = os.RemoveAll(pluginDir)
			_ = unregister(envDir, name)
			return fmt.Errorf("manifest not found; plugin directory removed. Note: shims may remain in bin/")
		}
		return err
	}

	if err := DeleteShims(envDir, man.Expose); err != nil {
		return err
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin dir: %w", err)
	}

	if err := unregister(envDir, man.Name); err != nil {
		return err
	}
	return nil
}

func unregister(envDir, name string) error {
	r, err := LoadRegistry(envDir)
	if err != nil {
		return err
	}
	out := make([]InstalledPlugin, 0, len(r.Plugins))
	for _, p := range r.Plugins {
		if p.Name != name {
			out = append(out, p)
		}
	}
	r.Plugins = out
	return SaveRegistry(envDir, r)
}
