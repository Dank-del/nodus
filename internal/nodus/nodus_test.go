package nodus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVendoredCPMCMakeMatchesPinnedChecksum(t *testing.T) {
	canonical := bytes.ReplaceAll(embeddedCPMCMake, []byte("\r\n"), []byte("\n"))
	sum := sha256.Sum256(canonical)
	if got := hex.EncodeToString(sum[:]); got != CPMCMakeSHA256 {
		t.Fatalf("CPM.cmake checksum = %s, want %s", got, CPMCMakeSHA256)
	}
}

func TestLocalPathsUseCMakeSeparators(t *testing.T) {
	sources := []string{`..\shared\library`}
	if runtime.GOOS == "windows" {
		sources = append(sources, `C:\Users\developer\library`)
	}
	for _, source := range sources {
		if !isLocalSource(source) {
			t.Fatalf("%q was not recognized as a local source", source)
		}
		rendered := strings.Join(renderSource(Dependency{Source: source}), "\n")
		if strings.Contains(rendered, `\`) {
			t.Fatalf("rendered local path contains a backslash: %s", rendered)
		}
	}
}

func TestManifestAndCPMRender(t *testing.T) {
	m := NewManifest("demo", false, ProjectOptions{})
	m.Dependencies["fmt"] = Dependency{Source: "github.com/fmtlib/fmt", Ref: "11.1.4", CMake: CMakePackage{Options: map[string]string{"FMT_TEST": "OFF"}}}
	root := t.TempDir()
	if err := WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(renderDependencies(parsed))
	for _, expected := range []string{"CPMAddPackage(", "GITHUB_REPOSITORY \"fmtlib/fmt\"", "GIT_TAG \"11.1.4\"", "\"FMT_TEST OFF\""} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("render missing %q:\n%s", expected, rendered)
		}
	}
}

func TestNodusBlockIsIdempotentAndSafe(t *testing.T) {
	contents := "cmake_minimum_required(VERSION 3.14)\nproject(demo)\n"
	first, err := upsertNodusBlock(contents)
	if err != nil {
		t.Fatal(err)
	}
	second, err := upsertNodusBlock(first)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || strings.Count(second, managedBlockStart) != 1 {
		t.Fatalf("managed block was not idempotent:\n%s", second)
	}
	if _, err := upsertNodusBlock(managedBlockStart + "\n"); err == nil {
		t.Fatal("expected malformed markers error")
	}
}

func TestLockHarnessUsesOnlyGeneratedDependencyFiles(t *testing.T) {
	harness := lockHarness("/tmp/project")
	for _, expected := range []string{"project(nodus_lock", "/tmp/project/cmake/nodus/CPM.cmake", "/tmp/project/cmake/nodus/dependencies.cmake"} {
		if !strings.Contains(harness, expected) {
			t.Fatalf("harness missing %q:\n%s", expected, harness)
		}
	}
}

func TestInitAddsBlockWithoutReplacingExistingCMake(t *testing.T) {
	root := t.TempDir()
	original := "cmake_minimum_required(VERSION 3.14)\nproject(existing LANGUAGES CXX)\nadd_executable(existing main.cpp)\n"
	if err := os.WriteFile(filepath.Join(root, "CMakeLists.txt"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InitProject(root, "existing"); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), nodusBlock()) || !strings.Contains(string(updated), "add_executable(existing main.cpp)") {
		t.Fatalf("existing CMake content was not preserved:\n%s", updated)
	}
}

func TestLocalCPMCMakeDependencyLocksAndBuilds(t *testing.T) {
	if err := RequireCMake(context.Background()); err != nil {
		t.Skipf("CMake unavailable: %v", err)
	}
	root := filepath.Join(t.TempDir(), "app")
	m, err := CreateProject(root, "app", ProjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(filepath.Dir(root), "dep")
	if err := os.MkdirAll(dependency, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dependency, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.14)\nproject(dep)\nadd_library(dep INTERFACE)\nadd_library(dep::dep ALIAS dep)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := m
	next.Dependencies["dep"] = Dependency{Source: dependency}
	if err := SyncDependencies(context.Background(), root, next, io.Discard); err != nil {
		t.Fatal(err)
	}
	lock, err := os.ReadFile(filepath.Join(root, "cmake", "nodus", "package-lock.cmake"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lock), "CPMDeclarePackage(dep") {
		t.Fatalf("CPM lock did not contain local dependency:\n%s", lock)
	}
	if err := AddLinks(root, &next, []string{"dep::dep"}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildProject(context.Background(), root, BuildOptions{}, io.Discard); err != nil {
		t.Fatal(err)
	}
	var runOutput bytes.Buffer
	if err := RunProject(context.Background(), root, next, BuildOptions{}, nil, &runOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runOutput.String(), "Hello from app!") {
		t.Fatalf("unexpected executable output: %q", runOutput.String())
	}
	if err := TestProject(context.Background(), root, BuildOptions{}, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestFailedRefreshDoesNotPersistManifest(t *testing.T) {
	if err := RequireCMake(context.Background()); err != nil {
		t.Skipf("CMake unavailable: %v", err)
	}
	root := filepath.Join(t.TempDir(), "app")
	m, err := CreateProject(root, "app", ProjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	next := m
	next.Dependencies["missing"] = Dependency{Source: "./does-not-exist"}
	if err := SyncDependencies(context.Background(), root, next, io.Discard); err == nil {
		t.Fatal("expected failed CMake refresh")
	}
	loaded, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Dependencies) != 0 {
		t.Fatalf("failed dependency persisted: %#v", loaded.Dependencies)
	}
}

func TestDependencyCanBeAddedBeforeExistingProjectLinksResolve(t *testing.T) {
	if err := RequireCMake(context.Background()); err != nil {
		t.Skipf("CMake unavailable: %v", err)
	}
	root := t.TempDir()
	cmake := "cmake_minimum_required(VERSION 3.14)\nproject(existing)\nadd_library(app INTERFACE)\ntarget_link_libraries(app INTERFACE dep::dep)\n"
	if err := os.WriteFile(filepath.Join(root, "CMakeLists.txt"), []byte(cmake), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := InitProject(root, "existing")
	if err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(root, "dependency")
	if err := os.MkdirAll(dependency, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dependency, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.14)\nproject(dep)\nadd_library(dep INTERFACE)\nadd_library(dep::dep ALIAS dep)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Dependencies["dep"] = Dependency{Source: dependency}
	if err := SyncDependencies(context.Background(), root, m, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestInspectUsesCPMCMakeForLocalPackage(t *testing.T) {
	if err := RequireCMake(context.Background()); err != nil {
		t.Skipf("CMake unavailable: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.14)\nproject(inspected)\nadd_library(inspected INTERFACE)\nadd_library(inspected::library ALIAS inspected)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(context.Background(), root, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.BuildSystem != "CMake" || !strings.Contains(strings.Join(inspection.Targets, ","), "inspected::library") {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
}
