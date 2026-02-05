package platform

import (
	"os"
	"path/filepath"
)

// InstallPlan describes where and how to install lyenv.
type InstallPlan struct {
	TargetDir string      // destination directory
	BinName   string      // logical base name (no extension)
	FileMode  os.FileMode // target file mode (ignored on Windows but kept consistent)
	Notes     []string
}

// Install copies src → plan.TargetDir/Decorate(plan.BinName)
func Install(src string, plan InstallPlan) error {
	if err := EnsureDir(plan.TargetDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(plan.TargetDir, DecorateName(plan.BinName))
	return CopyFileMode(src, dst, plan.FileMode)
}

// Uninstall removes plan.TargetDir/Decorate(plan.BinName)
func Uninstall(plan InstallPlan) error {
	dst := filepath.Join(plan.TargetDir, DecorateName(plan.BinName))
	return RemoveIfExists(dst)
}
