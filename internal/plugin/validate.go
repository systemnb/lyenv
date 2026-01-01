package plugin

import (
	"fmt"
	"strings"
)

// ValidateManifestStruct performs lightweight validation based on a JSON-Schema subset.
func ValidateManifestStruct(m *PluginManifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest validation failed: 'name' is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest validation failed: 'version' is required")
	}
	if len(m.Expose) == 0 {
		return fmt.Errorf("manifest validation failed: 'expose' must have at least one alias")
	}
	// Expose alias sanity: alnum, -, _
	for i, e := range m.Expose {
		if e == "" {
			return fmt.Errorf("manifest validation failed: expose[%d] must be non-empty string", i)
		}
		for _, r := range e {
			if !(r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return fmt.Errorf("manifest validation failed: expose[%d] contains invalid char: %q", i, r)
			}
		}
	}
	// Commands or entry required
	if len(m.Commands) == 0 && strings.TrimSpace(m.Entry.Path) == "" {
		return fmt.Errorf("manifest validation failed: either 'commands' or 'entry.path' must be provided")
	}
	// Entry type must be stdio if used
	if len(m.Commands) == 0 && m.Entry.Type != "stdio" {
		return fmt.Errorf("manifest validation failed: entry.type must be 'stdio' when commands are empty")
	}

	// Command validation
	for i, c := range m.Commands {
		if c.Name == "" {
			return fmt.Errorf("manifest validation failed: commands[%d].name is required", i)
		}
		// name must be unique
		for j := i + 1; j < len(m.Commands); j++ {
			if m.Commands[j].Name == c.Name {
				return fmt.Errorf("manifest validation failed: duplicate command name: %s", c.Name)
			}
		}
		if c.Executor != "shell" && c.Executor != "stdio" && c.Executor != "" {
			return fmt.Errorf("manifest validation failed: commands[%d].executor must be 'shell' or 'stdio'", i)
		}
		if strings.TrimSpace(c.Program) == "" && len(c.Steps) == 0 {
			return fmt.Errorf("manifest validation failed: commands[%d] requires either 'program' or non-empty 'steps'", i)
		}
		// Steps validation
		for j, s := range c.Steps {
			if s.Executor != "shell" && s.Executor != "stdio" {
				return fmt.Errorf("manifest validation failed: commands[%d].steps[%d].executor must be 'shell' or 'stdio'", i, j)
			}
			if strings.TrimSpace(s.Program) == "" {
				return fmt.Errorf("manifest validation failed: commands[%d].steps[%d].program is required", i, j)
			}
		}
	}
	return nil
}
