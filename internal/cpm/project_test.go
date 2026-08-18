package cpm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateManagedExecutableProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hello")
	m, err := CreateManagedProject(root, "hello", ProjectOptions{Type: "executable", CPPStandard: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Managed() || m.TargetName() != "hello" {
		t.Fatalf("unexpected manifest: %#v", m)
	}
	for _, path := range []string{"cpm.toml", "cpm.lock", "CMakeLists.txt", "src/main.cpp", ".cpm/generated/dependencies.cmake"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"project(hello VERSION 0.1.0", "add_executable(hello", "add_test(NAME hello_smoke"} {
		if !strings.Contains(string(b), expected) {
			t.Fatalf("generated CMake missing %q", expected)
		}
	}
}

func TestExplicitSourcesAreNormalizedAndRendered(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hello")
	m, err := CreateManagedProject(root, "hello", ProjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "extras", "feature.cpp")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("int feature() { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddSources(root, &m, []string{path}); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Build.Sources) != 1 || loaded.Build.Sources[0] != "extras/feature.cpp" {
		t.Fatalf("sources: %#v", loaded.Build.Sources)
	}
	b, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "extras/feature.cpp") {
		t.Fatalf("explicit source missing from CMake:\n%s", b)
	}
}

func TestManagedLinksRequireKnownTargets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hello")
	m, err := CreateManagedProject(root, "hello", ProjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AddLinks(root, &m, []string{"fmt::fmt"}); err == nil || !strings.Contains(err.Error(), "unknown CPM target") {
		t.Fatalf("unexpected error: %v", err)
	}
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Packages = []Package{{ID: "github.com/fmtlib/fmt", Source: "github.com/fmtlib/fmt", Commit: "abc", BuildSystem: "cmake", Targets: []string{"fmt::fmt"}}}
	if err := WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	if err := AddLinks(root, &m, []string{"fmt::fmt"}); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Build.Links) != 1 || loaded.Build.Links[0] != "fmt::fmt" {
		t.Fatalf("links: %#v", loaded.Build.Links)
	}
}

func TestDiscoverCMakeAliases(t *testing.T) {
	root := t.TempDir()
	contents := "project(example)\nset(TARGET_NAME ${PROJECT_NAME})\nadd_library(${PROJECT_NAME}::${TARGET_NAME} ALIAS example)\nadd_executable(example_tool ALIAS tool)\n"
	if err := os.WriteFile(filepath.Join(root, "CMakeLists.txt"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	aliases, err := discoverCMakeAliases(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(aliases, ",") != "example::example,example_tool" {
		t.Fatalf("aliases: %#v", aliases)
	}
}
