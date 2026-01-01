package plugin

import (
	"errors"
	"fmt"
	"os"
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
	targetDir := filepath.Join(pluginsDir, name)
	_ = os.RemoveAll(targetDir)

	if err := copyDir(srcPath, targetDir); err != nil {
		return fmt.Errorf("failed to copy plugin dir: %w", err)
	}

	man, err := LoadManifest(targetDir)
	if err != nil {
		return err
	}

	if err := CreateShims(envDir, man.Name, man.Expose); err != nil {
		return err
	}

	ip := InstalledPlugin{
		Name:        man.Name,
		Version:     man.Version,
		Source:      "local",
		Ref:         "",
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
	var srcType string // local|git|archive|url

	if src != "" && (strings.HasPrefix(src, ".") || strings.HasPrefix(src, "/")) {
		srcType = "local"
		name = filepath.Base(src)
	} else if optSource != "" {
		srcType = detectSourceType(optSource)
		name = inferNameFromSource(optSource)
	} else {
		if optRepo == "" {
			return errors.New("missing --repo for remote install")
		}
		srcType = "git"
		name = strings.TrimSuffix(filepath.Base(optRepo), ".git")
	}

	if strings.TrimSpace(overrideName) != "" {
		name = overrideName
	}

	targetDir := filepath.Join(pluginsDir, name)
	_ = os.RemoveAll(targetDir)

	man, err := LoadManifest(targetDir)
	if err != nil {
		return err
	}

	if err := CreateShims(envDir, man.Name, man.Expose); err != nil {
		return err
	}

	ip := InstalledPlugin{
		Name:        man.Name,
		Version:     man.Version,
		Source:      map[string]string{"local": "local", "git": "git", "archive": "archive", "url": "url"}[srcType],
		Ref:         optRef,
		InstalledAt: time.Now().UTC(),
	}
	if srcType == "git" {
		ip.Source = repoURL(optRepo, optProxy)
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
