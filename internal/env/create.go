package env

import (
	"errors"
	"fmt"
	"lyenv/internal/version"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func DefaultLyenvYAML() string {
	return `version: "1.0"
config:
  network:
    proxy_url: ""
  workspace:
    root: "./workspace"
  logs:
    dispatch_dir: ".lyenv/logs"
plugins:
  registry_url: "https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml"
  registry_format: "yaml"
  default_version_strategy: "latest"
`
}

// cmdCreate creates the directory structure for a lyenv environment.
func CmdCreate(dir string) error {
	absDir, errAbs := filepath.Abs(dir)
	if errAbs != nil {
		return fmt.Errorf("failed to resolve target path: %w", errAbs)
	}

	// Check if target exists
	fi, errStat := os.Stat(absDir)
	if errStat == nil {
		// Target exists
		if !fi.IsDir() {
			return fmt.Errorf("target exists but is not a directory: %s", absDir)
		}
		// Is a directory — check if already a lyenv dir
		if IsLyenvDir(absDir) {
			fmt.Println("Note: target directory already looks like a lyenv environment. Skipped.")
			return nil
		}
	} else if !errors.Is(errStat, os.ErrNotExist) {
		// Stat failed for reasons other than "not exists"
		return fmt.Errorf("failed to stat target directory: %w", errStat)
	}

	// Create base structure (mkdir -p style)
	subdirs := []string{
		filepath.Join(absDir, ".lyenv"),
		filepath.Join(absDir, ".lyenv", "logs"),
		filepath.Join(absDir, ".lyenv", "registry"),
		filepath.Join(absDir, "bin"),
		filepath.Join(absDir, "cache"),
		filepath.Join(absDir, "plugins"),
		filepath.Join(absDir, "workspace"),
	}
	for _, d := range subdirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Write metadata
	if err := WriteFileIfNotExists(filepath.Join(absDir, ".lyenv", "version"), version.Version+"\n", 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/version: %w", err)
	}
	stateJSON := `{
  "created_at": "` + time.Now().Local().String() + `",
  "components": [],
  "notes": ""
}
`
	if err := WriteFileIfNotExists(filepath.Join(absDir, ".lyenv", "state.json"), stateJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/state.json: %w", err)
	}

	// Initialize registry file (empty list)
	if err := WriteFileIfNotExists(filepath.Join(absDir, ".lyenv", "registry", "installed.yaml"), "plugins: []\n", 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/registry/installed.yaml: %w", err)
	}

	// Default environment configuration (augmented with plugin center settings)
	defaultCfg := `env:
  name: "default"
  platform: "auto"        # auto-detect system platform
path:
  bin: "./bin"
  cache: "./cache"
  workspace: "./workspace"
plugins:
  installed: []
  registry_url: "https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml"
  registry_format: "yaml"
  default_version_strategy: "latest"
config:
  use_container: false
  pkg_manager: "auto"     # auto-detect package manager
`
	cfgPath := filepath.Join(absDir, "lyenv.yaml")
	if err := WriteFileIfNotExists(cfgPath, defaultCfg, 0o644); err != nil {
		return fmt.Errorf("failed to write lyenv.yaml: %w", err)
	}

	return nil
}

// CmdInit verifies and repairs the lyenv environment (idempotent).
func CmdInit(dir string) error {
	absDir, errAbs := filepath.Abs(dir)
	if errAbs != nil {
		return fmt.Errorf("failed to resolve target path: %w", errAbs)
	}

	fmt.Println("Checking environment...")

	// Must exist and be a directory
	fi, errStat := os.Stat(absDir)
	if errStat != nil {
		return fmt.Errorf("target directory not found: %s", absDir)
	}
	if !fi.IsDir() {
		return fmt.Errorf("target exists but is not a directory: %s", absDir)
	}

	// Create missing structure (idempotent)
	subdirs := []string{
		filepath.Join(absDir, ".lyenv"),
		filepath.Join(absDir, ".lyenv", "logs"),
		filepath.Join(absDir, ".lyenv", "registry"),
		filepath.Join(absDir, "bin"),
		filepath.Join(absDir, "cache"),
		filepath.Join(absDir, "plugins"),
		filepath.Join(absDir, "workspace"),
	}
	for _, d := range subdirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Ensure metadata files exist
	if err := WriteFileIfNotExists(filepath.Join(absDir, ".lyenv", "version"), version.Version+"\n", 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/version: %w", err)
	}
	// Merge or create state.json with initialized_at (simple append if missing)
	statePath := filepath.Join(absDir, ".lyenv", "state.json")
	_ = EnsureInitializedAt(statePath)

	// Initialize registry file if missing
	_ = WriteFileIfNotExists(filepath.Join(absDir, ".lyenv", "registry", "installed.yaml"), "plugins: []\n", 0o644)

	// Ensure main config exists (augmented with plugin center settings)
	defaultCfg := `env:
  name: "default"
  platform: "auto"        # auto-detect system platform
path:
  bin: "./bin"
  cache: "./cache"
  workspace: "./workspace"
plugins:
  installed: []
  registry_url: "https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml"
  registry_format: "yaml"
  default_version_strategy: "latest"
config:
  use_container: false
  pkg_manager: "auto"     # auto-detect package manager
`
	if err := WriteFileIfNotExists(filepath.Join(absDir, "lyenv.yaml"), defaultCfg, 0o644); err != nil {
		return fmt.Errorf("failed to write lyenv.yaml: %w", err)
	}

	fmt.Println("OK: structure verified")
	return nil
}

// ensureInitializedAt writes/updates initialized_at in state.json (best-effort).
func EnsureInitializedAt(statePath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	// If not exists, create a minimal state.json with initialized_at
	_, err := os.Stat(statePath)
	if errors.Is(err, os.ErrNotExist) {
		content := `{
  "created_at": "` + now + `",
  "initialized_at": "` + now + `",
  "components": [],
  "notes": ""
}
`
		return os.WriteFile(statePath, []byte(content), 0o644)
	}
	// If exists, append a simple marker file next to it to avoid full JSON merge complexity in MVP
	marker := statePath + ".initialized"
	return os.WriteFile(marker, []byte(now+"\n"), 0o644)
}

// cmdActivate prints a snippet to activate the lyenv environment.
//
// Usage examples:
//   - bash/zsh (Linux/macOS): eval "$(lyenv activate)"
//   - PowerShell (Windows):  lyenv activate | Invoke-Expression
//   - CMD (Windows):         for /f "delims=" %i in ('lyenv activate --shell=cmd') do %i
func CmdActivate(shellOpt string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	bin := filepath.Join(cwd, "bin")

	// Normalize shell option
	sh := strings.ToLower(strings.TrimSpace(shellOpt))

	// Auto-detect if not provided
	if sh == "" {
		if runtime.GOOS == "windows" {
			sh = "powershell"
		} else {
			sh = "bash"
		}
	}

	switch sh {
	case "bash", "zsh", "sh":
		// Bash/Zsh snippet. User runs: eval "$(lyenv activate)"
		// NOTE: PS1 may be undefined in non-interactive shells; guard it to avoid 'unbound variable' with 'set -u'.
		fmt.Printf("export LYENV_HOME=%q\n", cwd)
		fmt.Printf("export PATH=%q:$PATH\n", bin)
		fmt.Println(`export LYENV_ACTIVE=1`)
		fmt.Println(`if [ -z "${LYENV_PROMPT_APPLIED+x}" ]; then`)
		fmt.Println(`  export LYENV_PROMPT_APPLIED=1`)
		fmt.Println(`  # Only modify PS1 if it is defined (handles 'set -u' non-interactive shells).`)
		fmt.Println(`  if [ -n "${PS1+x}" ]; then`)
		fmt.Println(`    export PS1="(lyenv) ${PS1}"`)
		fmt.Println(`  fi`)
		fmt.Println(`fi`)
		return nil

	case "powershell", "pwsh":
		// PowerShell snippet. User runs: lyenv activate | Invoke-Expression
		fmt.Printf("$env:LYENV_HOME = %q\n", cwd)
		fmt.Printf("$env:PATH = %q + \";\" + $env:PATH\n", bin)
		fmt.Println(`$env:LYENV_ACTIVE = "1"`)
		// IMPORTANT: must be single-line to work with pipeline + Invoke-Expression
		fmt.Println(`if (-not $env:LYENV_PROMPT_APPLIED) { $env:LYENV_PROMPT_APPLIED = "1" }`)
		return nil

	case "cmd":
		// CMD snippet. User runs:
		//   for /f "delims=" %i in ('lyenv activate --shell=cmd') do %i
		// Use %%PATH%% inside printed script because CMD expands % at execution time.
		fmt.Printf("set LYENV_HOME=%s\n", cwd)
		fmt.Printf("set PATH=%s;%%PATH%%\n", bin)
		fmt.Println("set LYENV_ACTIVE=1")
		return nil

	default:
		return fmt.Errorf("unsupported shell: %s (use --shell=bash|zsh|powershell|pwsh|cmd)", sh)
	}
}

// isLyenvDir checks if the directory already looks like a lyenv environment.
func IsLyenvDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".lyenv", "version")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "lyenv.yaml")); err == nil {
		return true
	}
	return false
}

// WriteFileIfNotExists writes a file only if it does not already exist.
func WriteFileIfNotExists(path, content string, perm os.FileMode) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil // skip existing
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check file %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(content), perm)
}
