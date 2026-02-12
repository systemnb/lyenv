package guictl

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	guiDirName  = "gui"
	pidFileName = "gui.pid"
	logsDirName = "logs"
	logFileName = "gui.log"
)

func StartGlobal(addr string) error {
	homeDir, err := globalHome()
	if err != nil {
		return err
	}
	rtDir := filepath.Join(homeDir, guiDirName)
	logDir := filepath.Join(rtDir, logsDirName)
	_ = os.MkdirAll(logDir, 0o755)

	// Idempotent
	if running, _ := isRunning(rtDir); running {
		return nil
	}

	guiBin, err := findGuiBin()
	if err != nil {
		return err
	}

	logPath := filepath.Join(logDir, logFileName)
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open gui log: %w", err)
	}
	defer lf.Close()

	cmd := exec.Command(guiBin, "--addr", addr)
	cmd.Dir = rtDir
	cmd.Stdout = lf
	cmd.Stderr = lf

	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gui process: %w", err)
	}

	pidPath := filepath.Join(rtDir, pidFileName)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("write pid file: %w", err)
	}

	waitPort(addr, 3*time.Second)
	return nil
}

func StopGlobal() error {
	homeDir, err := globalHome()
	if err != nil {
		return err
	}
	rtDir := filepath.Join(homeDir, guiDirName)

	pid, pidPath, err := readPid(rtDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	_ = killPid(pid)
	_ = os.Remove(pidPath)
	return nil
}

func StatusGlobal() (string, error) {
	homeDir, err := globalHome()
	if err != nil {
		return "", err
	}
	rtDir := filepath.Join(homeDir, guiDirName)

	running, pid := isRunning(rtDir)
	if running {
		return fmt.Sprintf("running (pid=%d)", pid), nil
	}
	return "stopped", nil
}

func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "android":
		// Termux preferred
		if _, err := exec.LookPath("termux-open-url"); err == nil {
			cmd = exec.Command("termux-open-url", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func globalHome() (string, error) {
	if v := strings.TrimSpace(os.Getenv("LYENV_GLOBAL_HOME")); v != "" {
		if err := os.MkdirAll(v, 0o755); err != nil {
			return "", fmt.Errorf("create LYENV_GLOBAL_HOME: %w", err)
		}
		return v, nil
	}
	hd, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	p := filepath.Join(hd, ".lyenv")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", fmt.Errorf("create default global home: %w", err)
	}
	return p, nil
}

func findGuiBin() (string, error) {
	if v := strings.TrimSpace(os.Getenv("LYENV_GUI_BIN")); v != "" {
		if st, err := os.Stat(v); err == nil && !st.IsDir() {
			return v, nil
		}
		return "", fmt.Errorf("LYENV_GUI_BIN not found: %s", v)
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		name := "lyenv-gui"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		cand := filepath.Join(dir, name)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}

	if p, err := exec.LookPath("lyenv-gui"); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("lyenv-gui.exe"); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("gui binary not found. Build it: make build-gui (or set LYENV_GUI_BIN)")
}

func readPid(rtDir string) (pid int, pidPath string, err error) {
	pidPath = filepath.Join(rtDir, pidFileName)
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, pidPath, err
	}
	s := strings.TrimSpace(string(b))
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, pidPath, fmt.Errorf("invalid pid in %s: %q", pidPath, s)
	}
	return n, pidPath, nil
}

func isRunning(rtDir string) (bool, int) {
	pid, _, err := readPid(rtDir)
	if err != nil {
		return false, 0
	}
	if runtime.GOOS != "windows" {
		p, _ := os.FindProcess(pid)
		if p == nil {
			return false, 0
		}
		if err := p.Signal(syscall.Signal(0)); err == nil {
			return true, pid
		}
		return false, 0
	}
	// Windows best-effort
	return true, pid
}

func killPid(pid int) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	_ = p.Signal(syscall.SIGTERM)
	time.Sleep(250 * time.Millisecond)
	return p.Kill()
}

func waitPort(addr string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
}
