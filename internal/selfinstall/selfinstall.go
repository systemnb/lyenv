package selfinstall

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallOptions controls install behavior.
type InstallOptions struct {
	BinDir  string // optional override
	GuiPath string // optional explicit lyenv-gui path
	InitGUI bool   // default true
}

type UninstallOptions struct {
	BinDir   string // optional override
	PurgeGUI bool   // remove ~/.lyenv/gui if true
}

func userHome() (string, error) { return os.UserHomeDir() }

func userLocalBin() (string, error) {
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func globalGuiDir() (string, error) {
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".lyenv", "gui"), nil
}

func defaultSystemBin() string {
	// Termux/Android: $PREFIX/bin
	if runtime.GOOS == "android" {
		prefix := os.Getenv("PREFIX")
		if strings.TrimSpace(prefix) != "" {
			return filepath.Join(prefix, "bin")
		}
		// typical Termux prefix
		return "/data/data/com.termux/files/usr/bin"
	}
	// Unix-like default
	return "/usr/local/bin"
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func findSibling(exeDir, name string) string {
	cand := filepath.Join(exeDir, name)
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand
	}
	return ""
}

// ensureGuiRuntime matches Makefile behavior:
// - mkdir ~/.lyenv/gui/logs
// - create ~/.lyenv/gui/config.yaml if missing
func ensureGuiRuntime() error {
	gdir, err := globalGuiDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(gdir, "logs"), 0o755); err != nil {
		return err
	}

	cfg := filepath.Join(gdir, "config.yaml")
	if _, err := os.Stat(cfg); err == nil {
		// keep existing
		return nil
	}

	home, _ := userHome()
	// Keep it simple; users can edit later.
	content := "" +
		"version: 1\n" +
		"envs:\n" +
		"  pinned: []\n" +
		"  scan:\n" +
		"    - root: \"" + filepath.ToSlash(filepath.Join(home, "lyenv-envs")) + "\"\n" +
		"      depth: 3\n" +
		"      pattern: \"lyenv.yaml\"\n" +
		"    - root: \"" + filepath.ToSlash(filepath.Join(home, "projects")) + "\"\n" +
		"      depth: 3\n" +
		"      pattern: \"lyenv.yaml\"\n" +
		"server:\n" +
		"  addr: \"127.0.0.1:18888\"\n"

	return os.WriteFile(cfg, []byte(content), 0o644)
}

// chooseBinDir implements option C:
// - if opt.BinDir specified -> use it
// - else try system dir -> if cannot write, fallback to ~/.local/bin
func chooseBinDir(optBinDir string) (dir string, usedFallback bool, err error) {
	if strings.TrimSpace(optBinDir) != "" {
		return optBinDir, false, nil
	}

	sys := defaultSystemBin()
	// probe write: try mkdir and create temp file
	if err := os.MkdirAll(sys, 0o755); err == nil {
		test := filepath.Join(sys, ".lyenv_write_test")
		if f, e2 := os.OpenFile(test, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644); e2 == nil {
			_ = f.Close()
			_ = os.Remove(test)
			return sys, false, nil
		}
	}

	// fallback
	ulb, err := userLocalBin()
	if err != nil {
		return "", true, err
	}
	if err := os.MkdirAll(ulb, 0o755); err != nil {
		return "", true, err
	}
	return ulb, true, nil
}

// Install installs both lyenv and lyenv-gui and initializes GUI runtime dir.
func Install(opt InstallOptions) error {
	bindir, usedFallback, err := chooseBinDir(opt.BinDir)
	if err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(self)

	lyName := "lyenv"
	guiName := "lyenv-gui"
	if runtime.GOOS == "windows" {
		lyName += ".exe"
		guiName += ".exe"
	}

	// 1) Install lyenv (current executable)
	dstLy := filepath.Join(bindir, lyName)
	if err := copyFile(self, dstLy, 0o755); err != nil {
		return fmt.Errorf("install lyenv failed: %w", err)
	}

	// 2) Locate and install lyenv-gui (required)
	guiSrc := strings.TrimSpace(opt.GuiPath)
	if guiSrc == "" {
		guiSrc = strings.TrimSpace(os.Getenv("LYENV_GUI_BIN"))
	}
	if guiSrc == "" {
		guiSrc = findSibling(exeDir, guiName)
	}
	if guiSrc == "" {
		if p, e := exec.LookPath(guiName); e == nil {
			guiSrc = p
		}
	}
	if guiSrc == "" {
		return fmt.Errorf("cannot find %s. Put it next to lyenv, or pass --gui=<path>, or set LYENV_GUI_BIN", guiName)
	}

	dstGui := filepath.Join(bindir, guiName)
	if err := copyFile(guiSrc, dstGui, 0o755); err != nil {
		return fmt.Errorf("install lyenv-gui failed: %w", err)
	}

	// 3) Init GUI runtime dir (Makefile-compatible)
	if opt.InitGUI {
		if err := ensureGuiRuntime(); err != nil {
			return err
		}
	}

	fmt.Printf("✔ Installed lyenv:     %s\n", dstLy)
	fmt.Printf("✔ Installed lyenv-gui: %s\n", dstGui)
	if usedFallback {
		fmt.Printf("NOTE: No permission for system bin; installed into user bin: %s\n", bindir)
		fmt.Println("If this directory is not in PATH, add it manually.")
	} else {
		fmt.Printf("✔ Installed into system bin dir: %s\n", bindir)
	}
	return nil
}

// Uninstall removes lyenv and lyenv-gui. If BinDir not given, try system then user.
func Uninstall(opt UninstallOptions) error {
	lyName := "lyenv"
	guiName := "lyenv-gui"
	if runtime.GOOS == "windows" {
		lyName += ".exe"
		guiName += ".exe"
	}

	// If bindir explicitly provided, remove only there
	if strings.TrimSpace(opt.BinDir) != "" {
		_ = os.Remove(filepath.Join(opt.BinDir, lyName))
		_ = os.Remove(filepath.Join(opt.BinDir, guiName))
		fmt.Printf("✔ Removed from: %s\n", opt.BinDir)
		return maybePurgeGUI(opt.PurgeGUI)
	}

	// Otherwise, try system dir first then user dir
	sys := defaultSystemBin()
	_ = os.Remove(filepath.Join(sys, lyName))
	_ = os.Remove(filepath.Join(sys, guiName))

	ulb, _ := userLocalBin()
	_ = os.Remove(filepath.Join(ulb, lyName))
	_ = os.Remove(filepath.Join(ulb, guiName))

	fmt.Printf("✔ Uninstall attempted in: %s and %s\n", sys, ulb)
	return maybePurgeGUI(opt.PurgeGUI)
}

func maybePurgeGUI(purge bool) error {
	gdir, _ := globalGuiDir()
	if purge {
		_ = os.RemoveAll(gdir)
		fmt.Printf("✔ Removed GUI runtime dir: %s\n", gdir)
	} else {
		fmt.Printf("NOTE: GUI runtime kept at %s (use --purge-gui=1 to remove)\n", gdir)
	}
	return nil
}
