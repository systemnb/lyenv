package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"lyenv/internal/config"
)

// SearchCenterPlugins fetches registry index (cache if present) and performs keyword search
// over plugin names and descriptions. Returns a slice of "name: desc" strings.
func SearchCenterPlugins(envDir string, keywords []string) ([]string, error) {
	if len(keywords) == 0 {
		return nil, fmt.Errorf("missing keywords")
	}
	// Load registry_url; use cache if available
	cfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read lyenv.yaml: %w", err)
	}
	regURL := config.GetString(cfg, "plugins.registry_url")
	if strings.TrimSpace(regURL) == "" {
		return nil, fmt.Errorf("plugins.registry_url not configured")
	}
	proxy := config.GetString(cfg, "config.network.proxy_url")

	// Use cached index if present
	cachePath := filepath.Join(envDir, ".lyenv", "registry", "index.yaml")
	idx := map[string]interface{}{}
	if _, err := os.Stat(cachePath); err == nil {
		idx, err = config.LoadAny(cachePath)
		if err != nil {
			return nil, fmt.Errorf("invalid cached index: %w")
		}
	} else {
		// fetch remote
		path, err := fetchToTempOrUseLocal(regURL, proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch registry index: %w", err)
		}
		idx, err = config.LoadAny(path)
		if err != nil {
			return nil, fmt.Errorf("invalid registry index: %w")
		}
	}

	pluginsRaw, ok := config.GetByPath(idx, "plugins")
	if !ok {
		return nil, fmt.Errorf("registry index missing 'plugins'")
	}
	plugins, ok := pluginsRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("registry plugins is not a map")
	}

	// Build keyword regexps (case-insensitive)
	var regs []*regexp.Regexp
	for _, k := range keywords {
		r, err := regexp.Compile("(?i)" + regexp.QuoteMeta(strings.TrimSpace(k)))
		if err == nil {
			regs = append(regs, r)
		}
	}
	matchAny := func(s string) bool {
		for _, r := range regs {
			if r.MatchString(s) {
				return true
			}
		}
		return false
	}

	results := []string{}
	for name, entryRaw := range plugins {
		entry, _ := entryRaw.(map[string]interface{})
		desc := asString(entry["desc"])
		if matchAny(name) || matchAny(desc) {
			results = append(results, fmt.Sprintf("%s: %s", name, desc))
		}
	}
	return results, nil
}
