package plugin

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"lyenv/internal/config"
)

// CenterRecord describes one plugin resolved from registry index.
type CenterRecord struct {
	Repo    string
	Ref     string
	Subpath string
	Source  string // optional download URL (zip/tgz) if used later
	Shims   []string
}

// ResolveFromCenterMonorepo resolves <NAME> from plugin center registry.
// registry_url can be local file path or HTTP URL (downloaded to temp by helper).
// It returns repo/ref/subpath/shims for monorepo sparse checkout.
func ResolveFromCenterMonorepo(envDir, name, wantVersion string) (*CenterRecord, error) {
	cfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read lyenv.yaml: %w")
	}
	v, ok := config.GetByPath(cfg, "plugins.registry_url")
	if !ok {
		return nil, fmt.Errorf("plugins.registry_url not configured")
	}
	regURL, _ := v.(string)
	if strings.TrimSpace(regURL) == "" {
		return nil, fmt.Errorf("plugins.registry_url empty")
	}
	proxy := config.GetString(cfg, "config.network.proxy_url")

	indexPath, err := fetchToTempOrUseLocal(regURL, proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry index: %w", err)
	}

	idx, err := config.LoadAny(indexPath)
	if err != nil {
		return nil, fmt.Errorf("invalid registry index: %w", err)
	}

	pluginsRaw, ok := config.GetByPath(idx, "plugins")
	if !ok {
		return nil, fmt.Errorf("registry index missing 'plugins'")
	}
	plugins, ok := pluginsRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("registry plugins is not a map")
	}

	entryRaw, ok := plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin not found in registry: %s", name)
	}
	entry, ok := entryRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid registry entry for: %s", name)
	}

	repo := asString(entry["repo"])
	subpath := asString(entry["subpath"])
	ref := asString(entry["ref"])

	var shims []string
	if sArr, ok := entry["shims"].([]interface{}); ok {
		for _, x := range sArr {
			shims = append(shims, fmt.Sprint(x))
		}
	}

	if vMapRaw, ok := entry["versions"].(map[string]interface{}); ok && len(vMapRaw) > 0 {
		versionKey := wantVersion
		if strings.TrimSpace(versionKey) == "" {
			versionKey = pickLatestVersionKey(vMapRaw)
		}
		vEntry, ok := vMapRaw[versionKey].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("version not found: %s@%s", name, versionKey)
		}
		repo = nonEmpty(asString(vEntry["repo"]), repo)
		ref = nonEmpty(asString(vEntry["ref"]), ref)
		subpath = nonEmpty(asString(vEntry["subpath"]), subpath)
		if sArr2, ok := vEntry["shims"].([]interface{}); ok {
			shims = shims[:0]
			for _, x := range sArr2 {
				shims = append(shims, fmt.Sprint(x))
			}
		}
	}

	if strings.TrimSpace(repo) == "" || strings.TrimSpace(subpath) == "" {
		return nil, fmt.Errorf("registry entry must provide repo and subpath for monorepo: %s", name)
	}
	if strings.TrimSpace(ref) == "" {
		ref = "main"
	}

	return &CenterRecord{Repo: repo, Ref: ref, Subpath: subpath, Shims: shims}, nil
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// pickLatestVersionKey chooses the latest semver-like key (simple string sort fallback).
func pickLatestVersionKey(versions map[string]interface{}) string {
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys[len(keys)-1]
}
