package nodus

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed assets/CPM.cmake
var embeddedCPMCMake []byte

type CMakeBackend struct{}

var _ Backend = CMakeBackend{}

func NewCMakeBackend() CMakeBackend { return CMakeBackend{} }

func (CMakeBackend) Name() string { return BackendCMake }

func (CMakeBackend) Ensure(root string, manifest Manifest) error {
	if manifest.Project.Backend != BackendCMake {
		return fmt.Errorf("project uses backend %q", manifest.Project.Backend)
	}
	if err := atomicWrite(filepath.Join(root, "cmake", "nodus", "CPM.cmake"), embeddedCPMCMake, 0o644); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(root, "cmake", "nodus", "dependencies.cmake"), renderDependencies(manifest), 0o644); err != nil {
		return err
	}
	return ensureNodusBlock(root, manifest)
}

// RefreshLock lets CPM.cmake do its own normal configure and package-lock update.
// The old lock stays untouched when CMake fails, so callers can safely stage a
// manifest/dependency change before committing it.
func (CMakeBackend) RefreshLock(ctx context.Context, root string, out io.Writer) error {
	if err := RequireCMake(ctx); err != nil {
		return err
	}
	// Locking intentionally configures only the generated dependency graph. An
	// existing application may link several packages before each declaration has
	// been added; configuring the application itself would make `nodus add`
	// unable to migrate it one dependency at a time.
	harness := filepath.Join(root, ".nodus", "lock-source", "CMakeLists.txt")
	if err := atomicWrite(harness, []byte(lockHarness(root)), 0o644); err != nil {
		return err
	}
	build := filepath.Join(root, ".nodus", "lock-build")
	if err := runTool(ctx, out, "cmake", "-S", filepath.Dir(harness), "-B", build); err != nil {
		return fmt.Errorf("CMake configuration failed; the existing lock was kept: %w", err)
	}
	if err := runTool(ctx, out, "cmake", "--build", build, "--target", "cpm-update-package-lock"); err != nil {
		return fmt.Errorf("CPM.cmake could not update its package lock: %w", err)
	}
	return nil
}

func lockHarness(root string) string {
	base := filepath.ToSlash(root)
	return "cmake_minimum_required(VERSION 3.14)\nproject(nodus_lock LANGUAGES C CXX)\n" +
		"set(EXTRACTED_CPM_VERSION \"" + CPMCMakeVersion + "\")\n" +
		"include(\"" + base + "/cmake/nodus/CPM.cmake\")\n" +
		"CPMUsePackageLock(\"" + base + "/cmake/nodus/package-lock.cmake\")\n" +
		"include(\"" + base + "/cmake/nodus/dependencies.cmake\")\n"
}

func (CMakeBackend) Install(ctx context.Context, root string, out io.Writer) error {
	if _, err := os.Stat(filepath.Join(root, "cmake", "nodus", "package-lock.cmake")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no CMake package lock found; run nodus update")
		}
		return err
	}
	if err := RequireCMake(ctx); err != nil {
		return err
	}
	return runTool(ctx, out, "cmake", "-S", root, "-B", filepath.Join(root, ".nodus", "install"))
}

func RequireCMake(ctx context.Context) error {
	if _, err := exec.LookPath("cmake"); err != nil {
		return fmt.Errorf("CMake >= 3.14 is required: %w", err)
	}
	out, err := exec.CommandContext(ctx, "cmake", "--version").Output()
	if err != nil {
		return err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return fmt.Errorf("unrecognized CMake version")
	}
	var major, minor int
	if _, err := fmt.Sscanf(fields[2], "%d.%d", &major, &minor); err != nil {
		return err
	}
	if major < 3 || major == 3 && minor < 14 {
		return fmt.Errorf("CMake %s is too old; need >= 3.14", fields[2])
	}
	return nil
}

func runTool(ctx context.Context, out io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
