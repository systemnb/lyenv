package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"lyenv/internal/config"
)

// CenterSync fetches registry_url and caches it into .lyenv/registry/index.{yaml|json}
func CenterSync(envDir string) (string, error) {
	cfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return "", fmt.Errorf("failed to read lyenv.yaml: %w", err)
	}
	regURL := config.GetString(cfg, "plugins.registry_url")
	if regURL == "" {
		return "", fmt.Errorf("plugins.registry_url not configured")
	}
	proxy := config.GetString(cfg, "config.network.proxy_url")

	// Download or use local file
	path, err := fetchToTempOrUseLocal(regURL, proxy)
	if err != nil {
		return "", fmt.Errorf("failed to fetch registry index: %w", err)
	}
	// Load and validate content
	idx, err := config.LoadAny(path)
	if err != nil {
		return "", fmt.Errorf("invalid registry index: %w", err)
	}
	// Write to cache
	cacheDir := filepath.Join(envDir, ".lyenv", "registry")
	_ = os.MkdirAll(cacheDir, 0o755)
	cacheName := "index.yaml"
	if config.IsJSON(path) {
		cacheName = "index.json"
	}
	cachePath := filepath.Join(cacheDir, cacheName)
	if err := config.SaveAny(cachePath, idx); err != nil {
		return "", fmt.Errorf("failed to write cache: %w", err)
	}
	return cachePath, nil
}
