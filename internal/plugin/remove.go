package plugin

import (
	"fmt"
	"os"
	"path/filepath"	
)

func PluginRemove(envDir, installName string, force bool) error {
    // First try registry record for shims and directory
    rec, err := GetByInstallName(envDir, installName)
    pluginDir := filepath.Join(envDir, "plugins", installName)

    if err == nil {
        // Remove shims listed in registry
        if err := DeleteShims(envDir, rec.Shims); err != nil && !force {
            return err
        }
        if err := os.RemoveAll(pluginDir); err != nil && !force {
            return fmt.Errorf("failed to remove plugin dir: %w", err)
        }
        if err := UnregisterByInstallName(envDir, installName); err != nil && !force {
            return err
        }
        return nil
    }

    // Fallback: try manifest direct read and remove shims
    man, err2 := LoadManifest(pluginDir)
    if err2 == nil {
        _ = DeleteShims(envDir, man.Expose)
    }
    _ = os.RemoveAll(pluginDir)
    _ = UnregisterByInstallName(envDir, installName)

    if !force && err != nil {
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
