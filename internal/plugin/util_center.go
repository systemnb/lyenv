package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fetchToTempOrUseLocal downloads regURL to a temp file (curl/wget) or returns regURL if local file.
func fetchToTempOrUseLocal(regURL, proxy string) (string, error) {
	if _, err := os.Stat(regURL); err == nil {
		return regURL, nil
	}
	tmp := filepath.Join(os.TempDir(), "plugin-center-index")
	if strings.HasSuffix(strings.ToLower(regURL), ".json") {
		tmp += ".json"
	} else {
		tmp += ".yaml"
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("curl"); err == nil {
		args := []string{"-L", "-o", tmp, regURL}
		if strings.TrimSpace(proxy) != "" {
			args = append([]string{"-x", proxy}, args...)
		}
		cmd = exec.Command("curl", args...)
	} else if _, err := exec.LookPath("wget"); err == nil {
		cmd = exec.Command("wget", "-O", tmp, regURL)
	} else {
		return "", fmt.Errorf("no downloader found (curl/wget)")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	return tmp, nil
}

func cloneSparseSubpath(repoURL, ref, proxy string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("'git' is not available")
	}
	if strings.TrimSpace(proxy) != "" && strings.HasPrefix(repoURL, "https://github.com/") {
		repoURL = proxy + "/" + repoURL + ".git"
	} else if !strings.HasSuffix(repoURL, ".git") && strings.HasPrefix(repoURL, "https://github.com/") {
		repoURL = repoURL + ".git"
	}
	work := filepath.Join(os.TempDir(), "plugin-center-work")
	_ = os.RemoveAll(work)

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repoURL, work)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	return work, nil
}
