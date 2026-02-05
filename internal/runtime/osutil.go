package runtime

import (
	"os"
	"os/exec"
	stdruntime "runtime"
)

// IsWindows reports whether current OS is Windows.
func IsWindows() bool { return stdruntime.GOOS == "windows" }

// IsAndroid reports whether current OS is Android (incl. Termux).
func IsAndroid() bool { return stdruntime.GOOS == "android" }

// Minimal interface to accept context without importing context here.
type execContext interface {
	Done() <-chan struct{}
	Err() error
}

// BuildShellCommandCtx returns *exec.Cmd to execute a shell line with context.
// - On Windows: prefer cmd.exe /C; fallback to powershell.
// - On Unix-like: prefer bash -c; fallback to sh -c.
func BuildShellCommandCtx(ctx execContext, workdir string, env []string, line string) *exec.Cmd {
	var cmd *exec.Cmd
	if IsWindows() {
		if _, err := exec.LookPath("cmd.exe"); err == nil {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", line)
		} else {
			cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", line)
		}
	} else {
		if _, err := exec.LookPath("bash"); err == nil {
			cmd = exec.CommandContext(ctx, "bash", "-c", line)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", line)
		}
	}
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}
