package plugin

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// downloadToFile downloads rawURL into outPath.
// Strategy: curl -> wget -> Go net/http fallback.
// proxy: optional proxy URL, e.g. http://127.0.0.1:7890
func downloadToFile(rawURL, outPath, proxy string) error {
	_ = os.MkdirAll(filepath.Dir(outPath), 0o755)

	// 1) curl
	if _, err := exec.LookPath("curl"); err == nil {
		args := []string{"-L", "-o", outPath, rawURL}
		if strings.TrimSpace(proxy) != "" {
			args = append([]string{"-x", proxy}, args...)
		}
		cmd := exec.Command("curl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// 2) wget
	if _, err := exec.LookPath("wget"); err == nil {
		cmd := exec.Command("wget", "-O", outPath, rawURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if strings.TrimSpace(proxy) != "" {
			cmd.Env = append(os.Environ(), "HTTP_PROXY="+proxy, "HTTPS_PROXY="+proxy)
		}
		return cmd.Run()
	}

	// 3) net/http fallback (works on Windows)
	tr := &http.Transport{}
	if strings.TrimSpace(proxy) != "" {
		pu, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("invalid proxy url: %w", err)
		}
		tr.Proxy = http.ProxyURL(pu)
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: tr,
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http download failed: status=%s", resp.Status)
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
