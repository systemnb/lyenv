// cmd/lyenv/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/systemnb/lyenv/internal/core"
	"github.com/systemnb/lyenv/internal/platform"
)

var version = "0.2.0" // bump after adding create

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "install":
		runInstall(os.Args[2:])
	case "uninstall":
		runUninstall(os.Args[2:])
	case "create":
		runCreate(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `lyenv %s

Usage:
  lyenv install    [--prefix DIR] [--name NAME] [--non-interactive]
  lyenv uninstall  [--prefix DIR] [--name NAME]
  lyenv create     [--root DIR] [--non-interactive]
  lyenv version

`, version)
}

func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	root := fs.String("root", ".", "ENV_ROOT directory")
	nonit := fs.Bool("non-interactive", false, "Do not ask; proceed best-effort")
	_ = fs.Parse(args)

	sum, err := core.CreateProject(core.CreateOptions{
		Root:           *root,
		NonInteractive: *nonit,
	})
	if err != nil {
		exitErr("create failed: %v", err)
	}

	// Summary to STDOUT
	fmt.Println("== lyenv create summary ==")
	fmt.Printf("ENV_ROOT        : %s\n", sum.EnvRoot)
	fmt.Printf("Platform        : %s\n", sum.Platform)
	if sum.PackageMgr != "" {
		fmt.Printf("Package Manager : %s\n", sum.PackageMgr)
	} else {
		fmt.Printf("Package Manager : (unknown)\n")
	}
	fmt.Printf("Config (YAML)   : %s\n", sum.ConfigYAML)
	fmt.Printf("Runtime Cache   : %s\n", sum.RuntimeCache)

	if ok, _ := platform.DirInPATH(filepath.Dir(os.Args[0])); !ok {
		// best-effort hint; installer already hints PATH after install
	}

	fmt.Println("Skeleton        :")
	for _, d := range sum.CreatedDirs {
		fmt.Printf("  - %s\n", d)
	}
}

//
// install / uninstall (unchanged from previous version, kept for completeness)
//

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	prefix := fs.String("prefix", "", "Install directory override")
	name := fs.String("name", "lyenv", "Binary base name (no extension)")
	nonit := fs.Bool("non-interactive", false, "Do not ask; just proceed or fail")
	_ = fs.Parse(args)

	exe, err := os.Executable()
	if err != nil {
		exitErr("cannot locate current executable: %v", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	plan, err := platform.DecidePlan(*prefix, *name)
	if err != nil {
		exitErr("cannot decide install plan: %v", err)
	}
	if !*nonit && len(plan.Notes) > 0 {
		for _, n := range plan.Notes {
			fmt.Fprintf(os.Stderr, "[INFO] %s\n", n)
		}
	}
	if err := platform.Install(exe, plan); err != nil {
		exitErr("install failed: %v", err)
	}
	if ok, _ := platform.DirInPATH(plan.TargetDir); !ok {
		fmt.Fprintf(os.Stderr, "[WARN] %s is not in PATH. Please update your PATH.\n", plan.TargetDir)
	}
	fmt.Printf("Installed: %s\n", filepath.Join(plan.TargetDir, platform.DecorateName(*name)))
}

func runUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	prefix := fs.String("prefix", "", "Install directory override")
	name := fs.String("name", "lyenv", "Binary base name (no extension)")
	_ = fs.Parse(args)

	plan, err := platform.DecidePlan(*prefix, *name)
	if err != nil {
		exitErr("cannot decide uninstall plan: %v", err)
	}
	target := filepath.Join(plan.TargetDir, platform.DecorateName(*name))
	if _, err := os.Stat(target); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] target not found: %s (nothing to remove)\n", target)
		os.Exit(0)
	}
	if !confirm(fmt.Sprintf("Remove %s ?", target)) {
		fmt.Fprintln(os.Stderr, "Cancelled.")
		return
	}
	if err := platform.Uninstall(plan); err != nil {
		exitErr("uninstall failed: %v", err)
	}
	fmt.Printf("Removed: %s\n", target)
}

//
// shared small helpers
//

func confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	var s string
	_, _ = fmt.Fscanln(os.Stdin, &s)
	return len(s) > 0 && (s[0] == 'y' || s[0] == 'Y')
}

func exitErr(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", a...)
	os.Exit(1)
}
