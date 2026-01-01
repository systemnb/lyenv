package plugin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func PluginAdd(envDir, src string, optSource string, optRepo string, optRef string, optProxy string) error {
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

	targetDir := filepath.Join(pluginsDir, name)
	_ = os.RemoveAll(targetDir)

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
		if optRef != "" {
			args = append(args, "--branch", optRef)
		}
		args = append(args, "--depth", "1", repoURL(optRepo, optProxy), targetDir)
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	case "archive":
		tmp := filepath.Join(os.TempDir(), name+"-plugin.tgz")
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
		if strings.HasSuffix(optSource, ".zip") {
			tmp := filepath.Join(os.TempDir(), name+"-plugin.zip")
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
		} else {
			return fmt.Errorf("unsupported URL type: %s", optSource)
		}
	default:
		return fmt.Errorf("unsupported source type: %s", srcType)
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
