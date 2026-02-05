// internal/platform/oscommon.go
// Platform-neutral helpers. Keep imports portable.
package platform

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DecorateName adds .exe on Windows, otherwise returns as-is.
func DecorateName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}

func EnsureDir(dir string, mode os.FileMode) error {
	if dir == "" {
		return errors.New("empty dir")
	}
	return os.MkdirAll(dir, mode)
}

func CopyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}

func RemoveIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.Remove(path)
}

func DirInPATH(dir string) (bool, error) {
	path := os.Getenv("PATH")
	if path == "" {
		return false, errors.New("PATH is empty")
	}
	parts := filepath.SplitList(path)
	clean := filepath.Clean(dir)
	for _, p := range parts {
		if filepath.Clean(p) == clean {
			return true, nil
		}
	}
	return false, nil
}
