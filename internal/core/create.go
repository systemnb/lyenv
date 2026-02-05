// internal/core/create.go
// lyenv "create" command implementation (platform-neutral).
// - Initialize ENV_ROOT (neutral to any plugin)
// - Create directories: config/, .cache/, plugins/, workspace/, logs/, tmp/
// - Write default YAML if absent (SSOT): config/lyenv.yaml
// - Update runtime cache JSON: .cache/lyenv.json
// - Auto-detect platform & package manager; if not found and interactive => ask

package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/systemnb/lyenv/internal/platform"
)

type CreateOptions struct {
	Root           string // ENV_ROOT (relative or absolute)
	NonInteractive bool   // do not ask; fail best-effort if cannot determine
	Force          bool   // reserved
}

type CreateSummary struct {
	EnvRoot      string
	Platform     string
	PackageMgr   string
	ConfigYAML   string
	RuntimeCache string
	CreatedDirs  []string
}

// Neutral default YAML (no plugin-specific keys). Comments in English.
const defaultYAML = `# lyenv global configuration (SSOT)
# This file is created by "lyenv create" if absent.
ly:
  project:
    root: "./"            # Relative to ENV_ROOT; lyenv will normalize internally
    target_arch: "aarch64"
  platform:
    prefer: ""            # "", or one of: windows | linux | android
  pm:
    prefer: ""            # "", or: winget/choco/scoop/pkg/apt/apt-get/dnf/pacman/zypper
  paths:
    bin_dir: "bin"
    logs_dir: "logs"
    tmp_dir: "tmp"

plugins: {}
`

func CreateProject(opts CreateOptions) (CreateSummary, error) {
	sum := CreateSummary{}
	if opts.Root == "" {
		opts.Root = "."
	}
	envRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return sum, fmt.Errorf("normalize root: %w", err)
	}
	sum.EnvRoot = envRoot

	// Prepare base folders (neutral; no plugin-specific dirs)
	baseDirs := []string{
		filepath.Join(envRoot, "config"),
		filepath.Join(envRoot, ".cache"),
		filepath.Join(envRoot, "plugins"),
		filepath.Join(envRoot, "workspace"),
		filepath.Join(envRoot, "logs"),
		filepath.Join(envRoot, "tmp"),
	}
	for _, d := range baseDirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return sum, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	sum.CreatedDirs = append(sum.CreatedDirs, baseDirs...)

	// Write default YAML if absent
	cfgYAML := filepath.Join(envRoot, "config", "lyenv.yaml")
	if _, err := os.Stat(cfgYAML); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(cfgYAML, []byte(defaultYAML), 0o644); err != nil {
			return sum, fmt.Errorf("write %s: %w", cfgYAML, err)
		}
	}
	sum.ConfigYAML = cfgYAML

	// Detect platform
	plat := detectPlatform()
	sum.Platform = plat

	// Detect package manager (platform-specific)
	pm, ok := platform.DetectPM()
	if !ok {
		if !opts.NonInteractive {
			pm = askPM(plat)
		} else {
			pm = "" // leave empty in non-interactive mode (best-effort)
		}
	}
	sum.PackageMgr = pm

	// Update runtime cache .cache/lyenv.json
	cachePath := filepath.Join(envRoot, ".cache", "lyenv.json")
	cache := readJSON(cachePath)
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	ensure := func(m map[string]any, k string) map[string]any {
		if v, ok := m[k]; ok {
			if mm, ok2 := v.(map[string]any); ok2 {
				return mm
			}
		}
		n := map[string]any{}
		m[k] = n
		return n
	}

	ly := ensure(cache, "ly")
	status := ensure(ly, "status")
	status["last_create_at"] = now
	platMap := ensure(ly, "platform")
	platMap["detected"] = plat
	pmMap := ensure(ly, "pm")
	if pm != "" {
		pmMap["detected"] = pm
	}
	proj := ensure(ly, "project")
	proj["root_abs"] = envRoot

	if err := writeJSON(cachePath, cache); err != nil {
		return sum, fmt.Errorf("write cache: %w", err)
	}
	sum.RuntimeCache = cachePath

	return sum, nil
}

func detectPlatform() string {
	switch goos := runtime.GOOS; goos {
	case "windows":
		return "windows"
	case "android":
		return "android"
	default:
		return "linux"
	}
}

func askPM(platform string) string {
	reader := bufio.NewReader(os.Stdin)
	opts := []string{}
	switch platform {
	case "android":
		opts = []string{"pkg", "apt"}
	case "windows":
		opts = []string{"winget", "choco", "scoop"}
	default: // linux
		opts = []string{"apt", "apt-get", "dnf", "pacman", "zypper"}
	}
	fmt.Fprintf(os.Stderr, "No package manager detected automatically.\n")
	fmt.Fprintf(os.Stderr, "Select one (%s) or press Enter to skip: ", strings.Join(opts, "/"))
	line, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(line)
	for _, o := range opts {
		if choice == o {
			return choice
		}
	}
	return ""
}

func readJSON(path string) map[string]any {
	f, err := os.Open(path)
	if err != nil {
		return map[string]any{}
	}
	defer f.Close()
	var m map[string]any
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return map[string]any{}
	}
	return m
}

func writeJSON(path string, data map[string]any) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
