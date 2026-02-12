package plugin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func httpDownload(url, outPath, proxy string) error {
    client := &http.Client{
        Timeout: 60 * time.Second,
    }
    req, err := http.NewRequest("GET", url, nil)
    if err != nil { return err }

    // Optional: proxy support (simple). If you already have proxy parsing util, reuse it.
    // If proxy is empty, default transport is fine.
    if strings.TrimSpace(proxy) != "" {
        // Minimal: rely on HTTP_PROXY/HTTPS_PROXY env, or implement Transport.Proxy parsing.
        // For now: set env so default transport can pick it up in many setups.
        _ = os.Setenv("HTTPS_PROXY", proxy)
        _ = os.Setenv("HTTP_PROXY", proxy)
    }

    resp, err := client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("http download failed: status=%s", resp.Status)
    }

    if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { return err }
    f, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
    if err != nil { return err }
    defer f.Close()

    _, err = io.Copy(f, resp.Body)
    return err
}

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

    // 1) curl
    if _, err := exec.LookPath("curl"); err == nil {
        args := []string{"-L", "-o", tmp, regURL}
        if strings.TrimSpace(proxy) != "" {
            args = append([]string{"-x", proxy}, args...)
        }
        cmd := exec.Command("curl", args...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err == nil {
            return tmp, nil
        }
    }

    // 2) wget
    if _, err := exec.LookPath("wget"); err == nil {
        cmd := exec.Command("wget", "-O", tmp, regURL)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err == nil {
            return tmp, nil
        }
    }

    // 3) Go HTTP fallback (cross-platform)
    if err := httpDownload(regURL, tmp, proxy); err != nil {
        return "", fmt.Errorf("download failed (curl/wget missing): %w", err)
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
