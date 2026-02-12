package guictl

import (
	"errors"
	"fmt"
	"lyenv/internal/env"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// English comments (per your preference).

type GuiConfig struct {
	Version int `yaml:"version"`
	Envs    struct {
		Pinned []PinnedEnv `yaml:"pinned"`
		// We keep scan field for backward compatibility, but your "design intent"
		// can set it empty and rely on pinned managed by lyenv.
		Scan []any `yaml:"scan,omitempty"`
	} `yaml:"envs"`
	Server struct {
		Addr string `yaml:"addr,omitempty"`
	} `yaml:"server,omitempty"`
}

type PinnedEnv struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

func configPath() (string, error) {
	home, err := globalHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "gui", "config.yaml"), nil
}

func ensureConfigDir() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	return p, nil
}

func loadGuiConfig() (*GuiConfig, string, error) {
	p, err := ensureConfigDir()
	if err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := defaultGuiConfig()
			_ = saveGuiConfig(&cfg)
			return &cfg, p, nil
		}
		return nil, p, err
	}
	var cfg GuiConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, p, err
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	// Ensure arrays not nil
	if cfg.Envs.Pinned == nil {
		cfg.Envs.Pinned = []PinnedEnv{}
	}
	return &cfg, p, nil
}

func saveGuiConfig(cfg *GuiConfig) error {
	p, err := ensureConfigDir()
	if err != nil {
		return err
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0o644)
}

func defaultGuiConfig() GuiConfig {
	var cfg GuiConfig
	cfg.Version = 1
	cfg.Envs.Pinned = []PinnedEnv{}
	// Default: no scan. You can keep scan for legacy, but rely on lyenv gui add.
	cfg.Envs.Scan = []any{}
	cfg.Server.Addr = "127.0.0.1:18888"
	return cfg
}

func isLyenvEnvDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "lyenv.yaml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, ".lyenv", "version")); err == nil {
		return true
	}
	return false
}

func normalizeAbsPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func pickNameFromPath(abs string) string {
	base := filepath.Base(abs)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "env"
	}
	return base
}

// GuiEnvAdd registers an env directory into GUI pinned list.
func GuiEnvAdd(path string, name string, createOn bool) error {
	abs, err := normalizeAbsPath(path)
	if err != nil {
		return err
	}
	if !isLyenvEnvDir(abs) {
		if !createOn {
			return fmt.Errorf("not a lyenv environment dir: %s (use --create=1 to auto create/init)", abs)
		}
		// Auto create + init (idempotent)
		if err := env.CmdCreate(abs); err != nil {
			return err
		}
		if err := env.CmdInit(abs); err != nil {
			return err
		}

	}

	cfg, _, err := loadGuiConfig()
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = pickNameFromPath(abs)
	}

	// de-dup by path; update name if already exists
	for i := range cfg.Envs.Pinned {
		if filepath.Clean(cfg.Envs.Pinned[i].Path) == abs {
			cfg.Envs.Pinned[i].Name = name
			return saveGuiConfig(cfg)
		}
	}

	// de-dup by name (if name already exists, append suffix)
	finalName := name
	existsName := func(n string) bool {
		for _, e := range cfg.Envs.Pinned {
			if e.Name == n {
				return true
			}
		}
		return false
	}
	if existsName(finalName) {
		// simple suffix strategy
		k := 2
		for existsName(fmt.Sprintf("%s-%d", name, k)) {
			k++
		}
		finalName = fmt.Sprintf("%s-%d", name, k)
	}

	cfg.Envs.Pinned = append(cfg.Envs.Pinned, PinnedEnv{Name: finalName, Path: abs})
	return saveGuiConfig(cfg)
}

// GuiEnvRemove removes by name or path.
func GuiEnvRemove(nameOrPath string) error {
	key := strings.TrimSpace(nameOrPath)
	if key == "" {
		return fmt.Errorf("name/path is empty")
	}

	cfg, _, err := loadGuiConfig()
	if err != nil {
		return err
	}

	// If looks like a path, normalize it.
	abs := ""
	if strings.Contains(key, string(filepath.Separator)) || strings.HasPrefix(key, ".") {
		if a, err := normalizeAbsPath(key); err == nil {
			abs = a
		}
	}

	out := make([]PinnedEnv, 0, len(cfg.Envs.Pinned))
	removed := false
	for _, e := range cfg.Envs.Pinned {
		if e.Name == key {
			removed = true
			continue
		}
		if abs != "" && filepath.Clean(e.Path) == abs {
			removed = true
			continue
		}
		out = append(out, e)
	}
	cfg.Envs.Pinned = out

	if !removed {
		return fmt.Errorf("not found: %s", key)
	}
	return saveGuiConfig(cfg)
}

func GuiEnvList() ([]PinnedEnv, error) {
	cfg, _, err := loadGuiConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Envs.Pinned, nil
}

func GuiEnvPrune() (int, error) {
	cfg, _, err := loadGuiConfig()
	if err != nil {
		return 0, err
	}

	out := make([]PinnedEnv, 0, len(cfg.Envs.Pinned))
	removed := 0
	for _, e := range cfg.Envs.Pinned {
		// keep only existing, valid env dirs
		if isLyenvEnvDir(e.Path) {
			out = append(out, e)
		} else {
			removed++
		}
	}
	cfg.Envs.Pinned = out
	if err := saveGuiConfig(cfg); err != nil {
		return removed, err
	}
	return removed, nil
}
