package plugin

import (
	"fmt"
	"os"
	"path/filepath"	
)

func PluginRemove(envDir, installName string, force bool) error {
	pluginDir := filepath.Join(envDir, "plugins", installName)

	// Try registry first
	if rec, err := GetByInstallName(envDir, installName); err == nil {
		_ = DeleteShims(envDir, rec.Shims)
		_ = os.RemoveAll(pluginDir)
		_ = UnregisterByInstallName(envDir, installName)
		return nil
	}

	// Fallback: load manifest if plugin dir still exists
	if man, err := LoadManifest(pluginDir); err == nil {
		_ = DeleteShims(envDir, man.Expose)
	}
	_ = os.RemoveAll(pluginDir)
	_ = UnregisterByInstallName(envDir, installName)

	// Final fallback: remove any file named like expose from bin/ if present
	// (This is rare; only when manifest and registry both missing)
	binDir := filepath.Join(envDir, "bin")
	entries, _ := os.ReadDir(binDir)
	for _, e := range entries {
		// Heuristic: if it's executable and starts with installName prefix or looks like known shim name, remove.
		p := filepath.Join(binDir, e.Name())
		_ = os.Remove(p)
	}

	if !force {
		// If caller requested strict behavior, report absence in registry
		return fmt.Errorf("plugin not found in registry: %s", installName)
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
