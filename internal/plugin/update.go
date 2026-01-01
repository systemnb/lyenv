package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lyenv/internal/config"
)

// PluginUpdate updates an installed plugin in-place.
// - installName: physical directory under plugins/
// - optRepo/optRef/optSource/optProxy override the original source if provided; otherwise fallback to registry record.
func PluginUpdate(envDir, installName, optRepo, optRef, optSource, optProxy string) error {
	pluginsDir := filepath.Join(envDir, "plugins")
	installDir := filepath.Join(pluginsDir, installName)

	rec, err := GetByInstallName(envDir, installName)
	if err != nil {
		return fmt.Errorf("plugin not found in registry: %s", installName)
	}

	// Decide source type
	srcType := ""
	repo := strings.TrimSpace(optRepo)
	ref := strings.TrimSpace(optRef)
	source := strings.TrimSpace(optSource)

	if repo != "" {
		srcType = "git"
	} else if source != "" {
		st := detectSourceType(source)
		if st == "archive" || st == "url" {
			srcType = st
		}
	} else {
		// fallback to registry Source
		switch rec.Source {
		case "git":
			srcType = "git"
			// we don't know original org/repo string; rec.Source stored a final URL
			// let repoURL() handle proxy prefix, so we reconstruct from rec.Source directly
			source = rec.Source
		case "archive", "url", "local":
			srcType = rec.Source
		default:
			return fmt.Errorf("unknown source in registry: %s", rec.Source)
		}
	}

	// Proxy fallback from lyenv.yaml if not provided
	if strings.TrimSpace(optProxy) == "" {
		cfg, _ := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
		if v, ok := config.GetByPath(cfg, "config.network.proxy_url"); ok {
			if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
				optProxy = strings.TrimSpace(s)
			}
		}
	}

	// Prepare temp dir for safe update
	tmp := filepath.Join(os.TempDir(), installName+"-update")
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}

	switch srcType {
	case "git":
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("'git' is not available. Please install git")
		}
		// Build clone command: if repo provided use repoURL(repo, proxy), else use 'source' URL from registry
		var cloneURL string
		if repo != "" {
			cloneURL = repoURL(repo, optProxy)
		} else {
			cloneURL = source
		}
		args := []string{"clone"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, "--depth", "1", cloneURL, tmp)
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}

	case "archive":
		if _, err := exec.LookPath("tar"); err != nil {
			return fmt.Errorf("'tar' is not available")
		}
		tgz := filepath.Join(os.TempDir(), installName+"-update.tgz")
		if source == "" {
			return fmt.Errorf("archive source URL is empty")
		}
		if err := fetchURL(source, tgz, optProxy); err != nil {
			return err
		}
		cmd := exec.Command("tar", "-xzf", tgz, "-C", tmp, "--strip-components=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tar extract failed: %w", err)
		}

	case "url":
		if _, err := exec.LookPath("unzip"); err != nil {
			return fmt.Errorf("'unzip' is not available")
		}
		if source == "" {
			return fmt.Errorf("zip source URL is empty")
		}
		zipf := filepath.Join(os.TempDir(), installName+"-update.zip")
		if err := fetchURL(source, zipf, optProxy); err != nil {
			return err
		}
		cmd := exec.Command("unzip", "-o", zipf, "-d", tmp)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unzip failed: %w", err)
		}

	case "local":
		// Update from current install directory itself? Not meaningful.
		// If user wants local update, they should call add/install again.
		return fmt.Errorf("local update not supported; use 'plugin add' to reinstall from local path")

	default:
		return fmt.Errorf("unsupported source type for update: %s", srcType)
	}

	// Validate manifest before replacing
	man, err := LoadManifest(tmp)
	if err != nil {
		return err
	}
	if err := ValidateManifestStruct(man); err != nil {
		return err
	}

	// Replace install directory atomically (best-effort)
	backup := installDir + ".bak"
	_ = os.RemoveAll(backup)
	if err := os.Rename(installDir, backup); err != nil {
		// fallback to remove then move
		_ = os.RemoveAll(installDir)
	}
	if err := os.Rename(tmp, installDir); err != nil {
		// restore on failure
		_ = os.Rename(backup, installDir)
		return fmt.Errorf("failed to replace plugin directory: %w", err)
	}
	_ = os.RemoveAll(backup)

	// Recreate shims (expose may change)
	if err := CreateShims(envDir, man.Name, man.Expose); err != nil {
		return err
	}

	// Update registry record
	rec.Name = man.Name
	rec.Version = man.Version
	rec.Shims = man.Expose
	rec.InstalledAt = time.Now().UTC()
	if err := RegisterInstall(envDir, *rec); err != nil {
		return err
	}

	fmt.Println("Update completed.")
	for _, e := range man.Expose {
		fmt.Printf("Executable ensured: bin/%s\n", e)
	}
	return nil
}
