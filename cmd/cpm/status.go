package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dank-del/cpm/internal/cpm"
)

func about(root string, out io.Writer) {
	writeBanner(out)
	fmt.Fprintf(out, "CPM %s (pre-release)\n", cpm.Version)
	fmt.Fprintln(out, "Git-native C/C++ source dependency manager with reproducible lockfiles and CMake source integration.")
	fmt.Fprintln(out, "Created and maintained by Sayan Biswas (Dank-del on GitHub).")
	fmt.Fprintln(out, "Repository: https://github.com/Dank-del/cpm")
	fmt.Fprintln(out, "\nEnvironment")
	fmt.Fprintf(out, "  Host: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(out, "  Git: %s\n", toolVersion("git", "--version"))
	fmt.Fprintf(out, "  CMake: %s\n", cmakeStatus())
	fmt.Fprintf(out, "  C compiler: %s\n", compilerStatus("CC", []string{"cc", "gcc", "clang"}))
	fmt.Fprintf(out, "  C++ compiler: %s\n", compilerStatus("CXX", []string{"c++", "g++", "clang++"}))
	fmt.Fprintf(out, "\nProject: %s\n", root)
	fmt.Fprintf(out, "  Manifest: %s\n", fileStatus(filepath.Join(root, cpm.ManifestName)))
	fmt.Fprintf(out, "  Lockfile: %s\n", fileStatus(filepath.Join(root, cpm.LockName)))
	fmt.Fprintf(out, "  Local CPM state: %s\n", fileStatus(filepath.Join(root, ".cpm")))
	if manifest, _, err := cpm.LoadManifest(root); err == nil {
		if manifest.Managed() {
			fmt.Fprintf(out, "  Managed target: %s (%s, C++%d)\n", manifest.TargetName(), manifest.Project.Type, manifest.Build.CPPStandard)
		} else {
			fmt.Fprintln(out, "  Managed target: not a CPM-managed project")
		}
	}
	fmt.Fprintln(out, "\nToolchain entries are detected only; CPM does not compile or configure a project for this command.")
	fmt.Fprintln(out, "This is an alpha release. CLI, manifest, and lockfile formats may change before the first stable release.")
}

func toolVersion(command string, args ...string) string {
	path, err := exec.LookPath(command)
	if err != nil {
		return "not found"
	}
	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return "found at " + path + " (version unavailable)"
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if line == "" {
		return "found at " + path
	}
	return line
}
func cmakeStatus() string {
	version := toolVersion("cmake", "--version")
	if version == "not found" {
		return "not found (CMake packages cannot be configured)"
	}
	if err := cpm.RequireCMake(); err != nil {
		return version + " (requires >= 3.14)"
	}
	return version + " (compatible)"
}
func compilerStatus(environment string, candidates []string) string {
	if selected := os.Getenv(environment); selected != "" {
		fields := strings.Fields(selected)
		if len(fields) > 0 {
			return environment + "=" + selected + ": " + toolVersion(fields[0], "--version")
		}
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate + ": " + toolVersion(candidate, "--version")
		}
	}
	return "not found"
}
func fileStatus(path string) string {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "not found"
	}
	if err != nil {
		return "unavailable (" + err.Error() + ")"
	}
	return "found"
}
