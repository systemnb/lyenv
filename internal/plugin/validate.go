package plugin

import (
	"fmt"
)

// ValidateManifestStruct performs lightweight validation based on a JSON-Schema subset.
func ValidateManifestStruct(m *PluginManifest) error {
	// Required top-level fields
	if m.Name == "" {
		return fmt.Errorf("manifest validation failed: 'name' is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest validation failed: 'version' is required")
	}
	if len(m.Expose) == 0 {
		return fmt.Errorf("manifest validation failed: 'expose' must have at least one alias")
	}
	for i, e := range m.Expose {
		if e == "" {
			return fmt.Errorf("manifest validation failed: expose[%d] must be non-empty string", i)
		}
	}

	// Commands
	if len(m.Commands) == 0 {
		// allow entry-only stdio plugins
		if m.Entry.Path == "" {
			return fmt.Errorf("manifest validation failed: either 'commands' or 'entry.path' must be provided")
		}
		if m.Entry.Type != "stdio" {
			return fmt.Errorf("manifest validation failed: entry.type must be 'stdio' when commands are empty")
		}
	} else {
		for i, c := range m.Commands {
			if c.Name == "" {
				return fmt.Errorf("manifest validation failed: commands[%d].name is required", i)
			}
			if c.Executor != "shell" && c.Executor != "stdio" {
				return fmt.Errorf("manifest validation failed: commands[%d].executor must be 'shell' or 'stdio'", i)
			}
			if c.Program == "" {
				return fmt.Errorf("manifest validation failed: commands[%d].program is required", i)
			}
		}
	}

	return nil
}
