package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dank-del/nodus/internal/nodus"
)

func about(root string, out io.Writer) {
	writeBanner(out)
	fmt.Fprintf(out, "Nodus %s\n", nodus.Version)
	fmt.Fprintln(out, "A native project companion. Nodus connects tools; it does not replace their build systems.")
	fmt.Fprintln(out, "CMake backend: CPM.cmake "+nodus.CPMCMakeVersion+" (vendored, pinned)")
	fmt.Fprintln(out, "Repository: https://github.com/Dank-del/nodus")
	fmt.Fprintln(out, "\nEnvironment")
	fmt.Fprintf(out, "  Host: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(out, "  CMake: %s\n", toolVersion("cmake", "--version"))
	fmt.Fprintf(out, "  C compiler: %s\n", compilerStatus("CC", []string{"cc", "gcc", "clang"}))
	fmt.Fprintf(out, "  C++ compiler: %s\n", compilerStatus("CXX", []string{"c++", "g++", "clang++"}))
	fmt.Fprintf(out, "  Git: %s\n", toolVersion("git", "--version"))
	fmt.Fprintln(out, "\nProject")
	fmt.Fprintf(out, "  Root: %s\n", root)
	fmt.Fprintf(out, "  Manifest: %s\n", fileStatus(filepath.Join(root, nodus.ManifestName)))
	fmt.Fprintf(out, "  CMake lock: %s\n", fileStatus(filepath.Join(root, "cmake", "nodus", "package-lock.cmake")))
	fmt.Fprintf(out, "  Local state: %s\n", fileStatus(filepath.Join(root, ".nodus")))
	if m, err := nodus.LoadManifest(root); err == nil {
		fmt.Fprintf(out, "  Backend: %s\n  Dependencies: %d\n", m.Project.Backend, len(m.Dependencies))
	}
	fmt.Fprintln(out, "\nNodus is 0.0.1-alpha. Manifest and backend interfaces may change before a stable release.")
}

func toolVersion(command string, args ...string) string {
	path, err := exec.LookPath(command)
	if err != nil {
		return "not found"
	}
	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return "found at " + path
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}
func compilerStatus(environment string, candidates []string) string {
	if selected := os.Getenv(environment); selected != "" {
		return environment + "=" + selected
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
	if err == nil {
		return "found"
	}
	if os.IsNotExist(err) {
		return "not found"
	}
	return "unavailable"
}
