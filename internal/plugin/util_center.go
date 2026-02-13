package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func fetchToTempOrUseLocal(regURL, proxy string) (string, error) {
	// 1) local file path -> use directly
	if st, err := os.Stat(regURL); err == nil && !st.IsDir() {
		return regURL, nil
	}

	// 2) remote URL -> download to temp
	tmp := filepath.Join(os.TempDir(), "lyenv-center-index")
	low := strings.ToLower(regURL)
	if strings.HasSuffix(low, ".json") {
		tmp += ".json"
	} else if strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".yaml") {
		tmp += ".yaml"
	} else {
		// default extension so config.LoadAny can parse as yaml
		tmp += ".yaml"
	}

	// Download using unified downloader (curl/wget or http fallback)
	if err := downloadToFile(regURL, tmp, proxy); err != nil {
		return "", err
	}
	return tmp, nil
}

func cloneSparseSubpath(repoURL, ref, proxy string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("'git' is not available")
	}

	// Ensure .git suffix for GitHub repo URL
	if strings.HasPrefix(repoURL, "https://github.com/") && !strings.HasSuffix(repoURL, ".git") {
		repoURL = repoURL + ".git"
	}

	// Apply mirror prefix from lyenv.yaml (DO NOT confuse with HTTP proxy)
	envDir := strings.TrimSpace(os.Getenv("LYENV_HOME"))
	if envDir == "" {
		envDir = "."
	}
	mirror := getMirrorPrefix(envDir)
	if mirror != "" && strings.HasPrefix(repoURL, "https://github.com/") {
		// mirror pattern: <mirror>/<original_url>
		// example: https://gh.llkk.cc/https://github.com/org/repo.git
		repoURL = mirror + repoURL
	}

	work := filepath.Join(os.TempDir(), "plugin-center-work")
	_ = os.RemoveAll(work)

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repoURL, work)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// HTTP proxy still supported for git via env
	if strings.TrimSpace(proxy) != "" {
		cmd.Env = append(os.Environ(),
			"HTTP_PROXY="+proxy,
			"HTTPS_PROXY="+proxy,
		)
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	return work, nil
}

