package nodus

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func CreateProject(root, name string, options ProjectOptions) (Manifest, error) {
	if _, err := os.Stat(root); err == nil {
		return Manifest{}, fmt.Errorf("destination %s already exists", root)
	} else if !os.IsNotExist(err) {
		return Manifest{}, err
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		return Manifest{}, err
	}
	m := NewManifest(name, true, options)
	if err := WriteManifest(root, m); err != nil {
		return Manifest{}, err
	}
	if err := atomicWrite(filepath.Join(root, "CMakeLists.txt"), []byte(managedCMake(m)), 0o644); err != nil {
		return Manifest{}, err
	}
	if err := NewCMakeBackend().Ensure(root, m); err != nil {
		return Manifest{}, err
	}
	if err := writeScaffold(root, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func InitProject(root, name string) (Manifest, error) {
	if _, err := os.Stat(filepath.Join(root, ManifestName)); err == nil {
		return Manifest{}, fmt.Errorf("%s already exists", ManifestName)
	} else if !os.IsNotExist(err) {
		return Manifest{}, err
	}
	if name == "" {
		name = filepath.Base(root)
	}
	m := NewManifest(name, false, ProjectOptions{})
	if err := WriteManifest(root, m); err != nil {
		return Manifest{}, err
	}
	if err := NewCMakeBackend().Ensure(root, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func SyncDependencies(ctx context.Context, root string, next Manifest, out io.Writer) error {
	backend := NewCMakeBackend()
	depPath := filepath.Join(root, "cmake", "nodus", "dependencies.cmake")
	oldDependencies, oldErr := os.ReadFile(depPath)
	if err := backend.Ensure(root, next); err != nil {
		return err
	}
	if err := backend.RefreshLock(ctx, root, out); err != nil {
		if oldErr == nil {
			_ = atomicWrite(depPath, oldDependencies, 0o644)
		}
		return err
	}
	if err := WriteManifest(root, next); err != nil {
		return err
	}
	return nil
}

func UpdateManagedProject(root string, m Manifest) error {
	if !m.Project.Managed {
		return fmt.Errorf("this command requires a project created with nodus new")
	}
	if err := WriteManifest(root, m); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(root, "CMakeLists.txt"), []byte(managedCMake(m)), 0o644)
}

func AddSources(root string, m *Manifest, paths []string) error {
	if !m.Project.Managed {
		return fmt.Errorf("this command requires a project created with nodus new")
	}
	for _, path := range paths {
		relative, err := normalizedSource(root, path)
		if err != nil {
			return err
		}
		m.Build.Sources = append(m.Build.Sources, relative)
	}
	m.Build.Sources = uniqueSorted(m.Build.Sources)
	return UpdateManagedProject(root, *m)
}

func RemoveSources(root string, m *Manifest, paths []string) error {
	remove := map[string]bool{}
	for _, path := range paths {
		relative, err := normalizedSource(root, path)
		if err != nil {
			return err
		}
		remove[relative] = true
	}
	filtered := m.Build.Sources[:0]
	for _, path := range m.Build.Sources {
		if !remove[path] {
			filtered = append(filtered, path)
		}
	}
	m.Build.Sources = filtered
	return UpdateManagedProject(root, *m)
}

func AddLinks(root string, m *Manifest, links []string) error {
	m.Build.Links = uniqueSorted(append(m.Build.Links, links...))
	return UpdateManagedProject(root, *m)
}
func RemoveLinks(root string, m *Manifest, links []string) error {
	remove := map[string]bool{}
	for _, link := range links {
		remove[link] = true
	}
	filtered := m.Build.Links[:0]
	for _, link := range m.Build.Links {
		if !remove[link] {
			filtered = append(filtered, link)
		}
	}
	m.Build.Links = filtered
	return UpdateManagedProject(root, *m)
}

func BuildProject(ctx context.Context, root string, options BuildOptions, out io.Writer) (string, error) {
	if err := RequireCMake(ctx); err != nil {
		return "", err
	}
	profile, buildType := buildProfile(options)
	build := filepath.Join(root, "build", profile)
	testing := "OFF"
	if options.Tests {
		testing = "ON"
	}
	if err := runTool(ctx, out, "cmake", "-S", root, "-B", build, "-DCMAKE_BUILD_TYPE="+buildType, "-DBUILD_TESTING="+testing); err != nil {
		return "", err
	}
	if err := runTool(ctx, out, "cmake", "--build", build, "--parallel"); err != nil {
		return "", err
	}
	return build, nil
}

func TestProject(ctx context.Context, root string, options BuildOptions, out io.Writer) error {
	options.Tests = true
	build, err := BuildProject(ctx, root, options, out)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("ctest"); err != nil {
		return fmt.Errorf("ctest is required: %w", err)
	}
	_, buildType := buildProfile(options)
	return runTool(ctx, out, "ctest", "--test-dir", build, "-C", buildType, "--output-on-failure")
}

func RunProject(ctx context.Context, root string, m Manifest, options BuildOptions, args []string, out io.Writer) error {
	if !m.Project.Managed || m.Project.Type != "executable" {
		return fmt.Errorf("nodus run is available for executable projects created with nodus new")
	}
	build, err := BuildProject(ctx, root, options, out)
	if err != nil {
		return err
	}
	target := targetName(m.Project.Name)
	_, buildType := buildProfile(options)
	for _, candidate := range []string{filepath.Join(build, target), filepath.Join(build, buildType, target)} {
		if executable, err := exec.LookPath(candidate); err == nil {
			return runTool(ctx, out, executable, args...)
		}
	}
	return fmt.Errorf("built executable %q was not found under %s", target, build)
}

func buildProfile(options BuildOptions) (profile, buildType string) {
	if options.Release {
		return "release", "Release"
	}
	return "debug", "Debug"
}

func InitializeGit(ctx context.Context, root string, out io.Writer) error {
	return runTool(ctx, out, "git", "init", "-b", "main", root)
}

func writeScaffold(root string, m Manifest) error {
	if err := atomicWrite(filepath.Join(root, ".gitignore"), []byte("/build/\n/.nodus/\n"), 0o644); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(root, "README.md"), []byte("# "+m.Project.Name+"\n\nCreated with Nodus.\n"), 0o644); err != nil {
		return err
	}
	name := targetName(m.Project.Name)
	if m.Project.Type == "library" {
		if err := os.MkdirAll(filepath.Join(root, "include", name), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
			return err
		}
		if err := atomicWrite(filepath.Join(root, "include", name, name+".hpp"), []byte("#pragma once\n\nint "+name+"_answer();\n"), 0o644); err != nil {
			return err
		}
		if err := atomicWrite(filepath.Join(root, "src", name+".cpp"), []byte("#include <"+name+"/"+name+".hpp>\n\nint "+name+"_answer() { return 42; }\n"), 0o644); err != nil {
			return err
		}
		return atomicWrite(filepath.Join(root, "tests", "smoke.cpp"), []byte("#include <"+name+"/"+name+".hpp>\n\nint main() { return "+name+"_answer() == 42 ? 0 : 1; }\n"), 0o644)
	}
	return atomicWrite(filepath.Join(root, "src", "main.cpp"), []byte("#include <iostream>\n\nint main() {\n  std::cout << \"Hello from "+m.Project.Name+"!\\n\";\n  return 0;\n}\n"), 0o644)
}

func managedCMake(m Manifest) string {
	target := targetName(m.Project.Name)
	var b strings.Builder
	fmt.Fprintf(&b, "# Generated by Nodus. Use Nodus commands to manage this file.\ncmake_minimum_required(VERSION 3.14)\nproject(%s VERSION %s LANGUAGES C CXX)\n\nset(CMAKE_CXX_STANDARD %d)\nset(CMAKE_CXX_STANDARD_REQUIRED ON)\nset(CMAKE_CXX_EXTENSIONS OFF)\n\n", target, m.Project.Version, m.Project.CPPStandard)
	b.WriteString(nodusBlock() + "\nfile(GLOB_RECURSE NODUS_SOURCES CONFIGURE_DEPENDS \"${CMAKE_CURRENT_SOURCE_DIR}/src/*.c\" \"${CMAKE_CURRENT_SOURCE_DIR}/src/*.cc\" \"${CMAKE_CURRENT_SOURCE_DIR}/src/*.cpp\" \"${CMAKE_CURRENT_SOURCE_DIR}/src/*.cxx\")\n")
	for _, source := range m.Build.Sources {
		fmt.Fprintf(&b, "list(APPEND NODUS_SOURCES \"${CMAKE_CURRENT_SOURCE_DIR}/%s\")\n", filepath.ToSlash(source))
	}
	if m.Project.Type == "library" {
		fmt.Fprintf(&b, "add_library(%s STATIC ${NODUS_SOURCES})\ntarget_include_directories(%s PUBLIC \"${CMAKE_CURRENT_SOURCE_DIR}/include\")\n", target, target)
	} else {
		fmt.Fprintf(&b, "add_executable(%s ${NODUS_SOURCES})\n", target)
	}
	if len(m.Build.Links) > 0 {
		fmt.Fprintf(&b, "target_link_libraries(%s PRIVATE\n", target)
		for _, link := range m.Build.Links {
			fmt.Fprintf(&b, "  %s\n", link)
		}
		b.WriteString(")\n")
	}
	b.WriteString("\ninclude(CTest)\nif(BUILD_TESTING)\n")
	if m.Project.Type == "library" {
		fmt.Fprintf(&b, "  add_executable(%s_smoke \"${CMAKE_CURRENT_SOURCE_DIR}/tests/smoke.cpp\")\n  target_link_libraries(%s_smoke PRIVATE %s)\n  add_test(NAME %s_smoke COMMAND %s_smoke)\n", target, target, target, target, target)
	} else {
		fmt.Fprintf(&b, "  add_test(NAME %s_smoke COMMAND %s)\n", target, target)
	}
	b.WriteString("endif()\n")
	return b.String()
}

func normalizedSource(root, path string) (string, error) {
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, path)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("source %q is a directory", path)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source %q must be inside the project", path)
	}
	switch strings.ToLower(filepath.Ext(relative)) {
	case ".c", ".cc", ".cpp", ".cxx":
	default:
		return "", fmt.Errorf("source %q must be a C or C++ source", path)
	}
	return filepath.ToSlash(relative), nil
}
func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
