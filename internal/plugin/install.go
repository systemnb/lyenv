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

	var name string
	var srcType string // local|git|archive|url|git-subpath

	// Case 1: explicit local path
	if src != "" && (strings.HasPrefix(src, ".") || filepath.IsAbs(src)) {
		if st, err := os.Stat(src); err == nil && st.IsDir() {
			srcType = "local"
			name = filepath.Base(src)
		}
	}

	// Case 2: explicit source URL (zip/tgz)
	if srcType == "" && strings.TrimSpace(optSource) != "" {
		srcType = detectSourceType(optSource)
		name = inferNameFromSource(optSource)
	}

	// Case 3: explicit repo (org/repo)
	if srcType == "" && strings.TrimSpace(optRepo) != "" {
		srcType = "git"
		name = strings.TrimSuffix(filepath.Base(optRepo), ".git")
	}

	// Case 4: center resolution when only NAME provided
	if srcType == "" && src != "" {
		rec, err := ResolveFromCenterMonorepo(envDir, strings.TrimSpace(src), strings.TrimSpace(optRef))
		if err != nil {
			return fmt.Errorf("failed to resolve from plugin center: %w", err)
		}
		srcType = "git-subpath"
		optRepo = rec.Repo
		optRef = rec.Ref
		src = rec.Subpath // reuse src to carry subpath
		name = filepath.Base(rec.Subpath)
	}

	if srcType == "" {
		return errors.New("missing source: provide <PATH>, or --repo=<org/repo>, or --source=<url>, or configure plugin center")
	}

	// Apply custom install name (sanitized)
	if strings.TrimSpace(overrideName) != "" {
		name = sanitizeInstallName(overrideName)
	}
	installName := name
	targetDir := filepath.Join(pluginsDir, installName)
	_ = os.RemoveAll(targetDir)

	// Proxy fallback from lyenv.yaml if not provided
	if strings.TrimSpace(optProxy) == "" {
		cfg, _ := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
		if v, ok := config.GetByPath(cfg, "config.network.proxy_url"); ok {
			if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
				optProxy = strings.TrimSpace(s)
			}
		}
	}

	switch srcType {
	case "local":
		if err := copyDir(src, targetDir); err != nil {
			return fmt.Errorf("failed to install local plugin: %w", err)
		}

	case "git":
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("'git' is not available. Please install git or use --source=<zip url>")
		}
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

	case "git-subpath":
		work, err := cloneSparseSubpath("https://github.com/"+strings.TrimSpace(optRepo), strings.TrimSpace(optRef), optProxy)
		if err != nil {
			return err
		}
		subAbs := filepath.Join(work, src)
		if _, err := os.Stat(subAbs); err != nil {
			return fmt.Errorf("subpath not found in monorepo: %s", src)
		}
		if err := copyDir(subAbs, targetDir); err != nil {
			return fmt.Errorf("failed to copy subpath to target: %w", err)
		}

	case "archive":
		tmp := filepath.Join(os.TempDir(), installName+"-plugin.tgz")
		if err := fetchURL(optSource, tmp, optProxy); err != nil {
			return err
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		cmd := exec.Command("tar", "-xzf", tmp, "-C", targetDir, "--strip-components=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tar extract failed: %w", err)
		}

	case "url":
		if !strings.HasSuffix(strings.ToLower(optSource), ".zip") {
			return fmt.Errorf("unsupported URL type: %s (only .zip supported)", optSource)
		}
		tmp := filepath.Join(os.TempDir(), installName+"-plugin.zip")
		if err := fetchURL(optSource, tmp, optProxy); err != nil {
			return err
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
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

	// Normalize permissions and ensure logs dir
	_ = NormalizePluginPermissions(targetDir)
	_ = EnsureLogsDir(targetDir)

	// Load manifest
	man, err := LoadManifest(targetDir)
	if err != nil {
		return err
	}
	if err := ValidateManifestStruct(man); err != nil {
		return err
	}

	// Create shims bound to installName
	if err := CreateShims(envDir, installName, man.Expose); err != nil {
		return err
	}

	// Register installation
	ip := InstalledPlugin{
		Name:        man.Name,
		InstallName: installName,
		Version:     man.Version,
		Source:      srcType,
		Ref:         strings.TrimSpace(optRef),
		Shims:       man.Expose,
		InstalledAt: time.Now().UTC(),
	}
	// For git-subpath, store repo URL for info
	if srcType == "git-subpath" {
		ip.Source = "https://github.com/" + strings.TrimSpace(optRepo)
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
