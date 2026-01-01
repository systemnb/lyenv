package plugin

import (
	"errors"
	"fmt"
	"lyenv/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func PluginAddLocal(envDir, srcPath, overrideName string) error {
	if srcPath == "" {
		return fmt.Errorf("path must not be empty")
	}
	fi, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("path is not a directory: %s", srcPath)
	}

	pluginsDir := filepath.Join(envDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return err
	}

	name := filepath.Base(srcPath)
	if strings.TrimSpace(overrideName) != "" {
		name = overrideName
	}
	installName := name
	targetDir := filepath.Join(pluginsDir, installName)
	_ = os.RemoveAll(targetDir)

	if err := copyDir(srcPath, targetDir); err != nil {
		return fmt.Errorf("failed to copy plugin dir: %w", err)
	}

	man, err := LoadManifest(targetDir)
	if err != nil {
		return err
	}
	if err := ValidateManifestStruct(man); err != nil {
		return err
	}

	_ = NormalizePluginPermissions(targetDir)
	_ = EnsureLogsDir(targetDir)

	if err := CreateShims(envDir, installName, man.Expose); err != nil {
		return err
	}

	ip := InstalledPlugin{
		Name:        man.Name,
		InstallName: installName,
		Version:     man.Version,
		Source:      "local",
		Ref:         "",
		Shims:       man.Expose,
		InstalledAt: time.Now().UTC(),
	}
	if err := RegisterInstall(envDir, ip); err != nil {
		return err
	}

	fmt.Println("Plugin installed successfully.")
	for _, e := range man.Expose {
		fmt.Printf("Executable generated: bin/%s\n", e)
	}
	return nil
}

func PluginAdd(envDir, src, optSource, optRepo, optRef, optProxy, overrideName string) error {
	pluginsDir := filepath.Join(envDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return err
	}

	// Decide source type and default install name
	var srcType string // local|git|archive|url
	var name string

	// Local path: "./..." or "/..."; also allow relative non-empty path that exists
	if src != "" && (strings.HasPrefix(src, ".") || filepath.IsAbs(src)) {
		if st, err := os.Stat(src); err == nil && st.IsDir() {
			srcType = "local"
			name = filepath.Base(src)
		}
	}
	// Explicit source URL (zip/tgz)
	if srcType == "" && strings.TrimSpace(optSource) != "" {
		srcType = detectSourceType(optSource)
		name = inferNameFromSource(optSource)
	}
	// Git repo (org/repo)
	if srcType == "" && strings.TrimSpace(optRepo) != "" {
		srcType = "git"
		name = strings.TrimSuffix(filepath.Base(optRepo), ".git")
	}
	// If still unknown, error out clearly
	if srcType == "" {
		return errors.New("missing source: provide a local path, or --repo=<org/repo>, or --source=<url>")
	}

	// Apply custom install name (sanitized)
	if strings.TrimSpace(overrideName) != "" {
		name = sanitizeInstallName(overrideName)
	}
	installName := name
	targetDir := filepath.Join(pluginsDir, installName)

	// Proxy fallback from lyenv.yaml (config.network.proxy_url)
	if strings.TrimSpace(optProxy) == "" {
		cfg, _ := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
		if v, ok := config.GetByPath(cfg, "config.network.proxy_url"); ok {
			if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
				optProxy = strings.TrimSpace(s)
			}
		}
	}

	// Overwrite existing installation directory
	_ = os.RemoveAll(targetDir)

	switch srcType {
	case "local":
		// Copy entire directory into plugins/<installName>/
		if err := copyDir(src, targetDir); err != nil {
			return fmt.Errorf("failed to install local plugin: %w", err)
		}

	case "git":
		// Require git available
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("'git' is not available. Please install git or use --source=<zip|tgz url>")
		}
		// Build clone command with optional ref
		args := []string{"clone"}
		if strings.TrimSpace(optRef) != "" {
			args = append(args, "--branch", strings.TrimSpace(optRef))
		}
		args = append(args, "--depth", "1", repoURL(optRepo, optProxy), targetDir)

		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}

	case "archive":
		// Expect .tar.gz / .tgz
		tmp := filepath.Join(os.TempDir(), installName+"-plugin.tgz")
		if err := fetchURL(optSource, tmp, optProxy); err != nil {
			return err
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		// Extract archive into targetDir (strip top-level directory)
		if _, err := exec.LookPath("tar"); err != nil {
			return fmt.Errorf("'tar' is not available")
		}
		cmd := exec.Command("tar", "-xzf", tmp, "-C", targetDir, "--strip-components=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tar extract failed: %w", err)
		}

	case "url":
		// Support .zip URLs; download and unzip
		if !strings.HasSuffix(strings.ToLower(optSource), ".zip") {
			return fmt.Errorf("unsupported URL type: %s (only .zip supported here)", optSource)
		}
		tmp := filepath.Join(os.TempDir(), installName+"-plugin.zip")
		if err := fetchURL(optSource, tmp, optProxy); err != nil {
			return err
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		if _, err := exec.LookPath("unzip"); err != nil {
			return fmt.Errorf("'unzip' is not available")
		}
		cmd := exec.Command("unzip", "-o", tmp, "-d", targetDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unzip failed: %w", err)
		}

	default:
		return fmt.Errorf("unsupported source type: %s", srcType)
	}

	// Load manifest and validate minimal fields
	man, err := LoadManifest(targetDir)
	if err != nil {
		return err
	}
	if err := ValidateManifestStruct(man); err != nil {
		return err
	}

	_ = NormalizePluginPermissions(targetDir)
	_ = EnsureLogsDir(targetDir)

	// Create shims in bin/ for each exposed alias
	if err := CreateShims(envDir, installName, man.Expose); err != nil {
		return err
	}

	// Record installation into registry with richer metadata
	ip := InstalledPlugin{
		Name:        man.Name,
		InstallName: installName,
		Version:     man.Version,
		Source:      map[string]string{"local": "local", "git": "git", "archive": "archive", "url": "url"}[srcType],
		Ref:         strings.TrimSpace(optRef),
		Shims:       man.Expose,
		InstalledAt: time.Now().UTC(),
	}
	if srcType == "git" {
		ip.Source = repoURL(optRepo, optProxy) // store resolved URL (with proxy prefix when any)
	}
	if err := RegisterInstall(envDir, ip); err != nil {
		return err
	}

	// Success output (English)
	fmt.Println("Plugin installed successfully.")
	for _, e := range man.Expose {
		fmt.Printf("Executable generated: bin/%s\n", e)
	}
	return nil
}

// If you do not have this helper yet, you can include it:
func sanitizeInstallName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "plugin"
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "plugin"
	}
	return string(out)
}

func detectSourceType(u string) string {
	if strings.HasSuffix(u, ".tar.gz") || strings.HasSuffix(u, ".tgz") {
		return "archive"
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return "url"
	}
	return "url"
}

func inferNameFromSource(u string) string {
	base := filepath.Base(u)
	base = strings.TrimSuffix(base, ".tar.gz")
	base = strings.TrimSuffix(base, ".tgz")
	base = strings.TrimSuffix(base, ".zip")
	return base
}

func repoURL(repo, proxy string) string {
	url := "https://github.com/" + strings.TrimSpace(repo) + ".git"
	if proxy != "" {
		return proxy + "/" + url
	}
	return url
}
