package cpm

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

type ProjectOptions struct {
	Type        string
	CPPStandard int
}

type BuildOptions struct {
	Release bool
	Tests   bool
}

func CreateManagedProject(root, name string, options ProjectOptions) (Manifest, error) {
	if options.Type == "" {
		options.Type = "executable"
	}
	if options.CPPStandard == 0 {
		options.CPPStandard = 20
	}
	if options.Type != "executable" && options.Type != "library" {
		return Manifest{}, fmt.Errorf("project type must be executable or library")
	}
	if _, err := os.Stat(root); err == nil {
		return Manifest{}, fmt.Errorf("destination %s already exists", root)
	} else if !os.IsNotExist(err) {
		return Manifest{}, err
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		return Manifest{}, err
	}
	m := NewManagedManifest(name, options.Type, options.CPPStandard)
	if err := WriteManifest(root, m); err != nil {
		return Manifest{}, err
	}
	if err := WriteLock(root, Lock{Version: 1, ManifestHash: ManifestHash(MarshalManifest(m))}); err != nil {
		return Manifest{}, err
	}
	if err := RenderManagedProject(root, m, Lock{}); err != nil {
		return Manifest{}, err
	}
	if err := writeScaffoldFiles(root, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func writeScaffoldFiles(root string, m Manifest) error {
	if err := atomicWrite(filepath.Join(root, ".gitignore"), []byte("/build/\n/.cpm/\n"), 0o644); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(root, "README.md"), []byte("# "+m.Project.Name+"\n\nCreated with CPM.\n"), 0o644); err != nil {
		return err
	}
	if m.Project.Type == "library" {
		name := m.TargetName()
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

func UpdateManagedManifest(root string, m Manifest) error {
	if !m.Managed() {
		return fmt.Errorf("this command requires a CPM-managed project")
	}
	normalizeManagedManifest(&m)
	if err := WriteManifest(root, m); err != nil {
		return err
	}
	lock, err := LoadLock(root)
	if err != nil {
		return fmt.Errorf("read lockfile: %w", err)
	}
	lock.ManifestHash = ManifestHash(MarshalManifest(m))
	if err := WriteLock(root, lock); err != nil {
		return err
	}
	return RenderManagedProject(root, m, lock)
}

func AddSources(root string, m *Manifest, paths []string) error {
	if !m.Managed() {
		return fmt.Errorf("this command requires a CPM-managed project")
	}
	for _, path := range paths {
		normalized, err := normalizedSource(root, path)
		if err != nil {
			return err
		}
		m.Build.Sources = append(m.Build.Sources, normalized)
	}
	return UpdateManagedManifest(root, *m)
}

func RemoveSources(root string, m *Manifest, paths []string) error {
	if !m.Managed() {
		return fmt.Errorf("this command requires a CPM-managed project")
	}
	remove := map[string]bool{}
	for _, path := range paths {
		normalized, err := normalizedSource(root, path)
		if err != nil {
			return err
		}
		remove[normalized] = true
	}
	remaining := make([]string, 0, len(m.Build.Sources))
	for _, source := range m.Build.Sources {
		if !remove[source] {
			remaining = append(remaining, source)
		}
	}
	m.Build.Sources = remaining
	return UpdateManagedManifest(root, *m)
}

func AddLinks(root string, m *Manifest, targets []string) error {
	available, err := AvailableTargets(root)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if !available[target] {
			return fmt.Errorf("unknown CPM target %q; available targets: %s", target, strings.Join(sortedTargetNames(available), ", "))
		}
		m.Build.Links = append(m.Build.Links, target)
	}
	return UpdateManagedManifest(root, *m)
}

func RemoveLinks(root string, m *Manifest, targets []string) error {
	remove := map[string]bool{}
	for _, target := range targets {
		remove[target] = true
	}
	remaining := make([]string, 0, len(m.Build.Links))
	for _, target := range m.Build.Links {
		if !remove[target] {
			remaining = append(remaining, target)
		}
	}
	m.Build.Links = remaining
	return UpdateManagedManifest(root, *m)
}

func BuildManagedProject(ctx context.Context, root string, m Manifest, options BuildOptions, out io.Writer) (string, error) {
	if !m.Managed() {
		return "", fmt.Errorf("cpm build requires a CPM-managed project")
	}
	if err := requireCMake(ctx); err != nil {
		return "", fmt.Errorf("CMake >= 3.14 is required to build managed projects; install cmake and retry: %w", err)
	}
	manifestBytes := MarshalManifest(m)
	lock, err := LoadLock(root)
	if err != nil {
		return "", fmt.Errorf("read lockfile: %w", err)
	}
	if lock.ManifestHash != ManifestHash(manifestBytes) {
		return "", fmt.Errorf("cpm.lock does not match cpm.toml; run cpm update")
	}
	if err := RenderManagedProject(root, m, lock); err != nil {
		return "", err
	}
	profile := "debug"
	buildType := "Debug"
	if options.Release {
		profile, buildType = "release", "Release"
	}
	buildDir := filepath.Join(root, "build", profile)
	testing := "OFF"
	if options.Tests {
		testing = "ON"
	}
	if err := runTool(ctx, out, "cmake", "-S", root, "-B", buildDir, "-DCMAKE_BUILD_TYPE="+buildType, "-DBUILD_TESTING="+testing); err != nil {
		return "", err
	}
	if err := runTool(ctx, out, "cmake", "--build", buildDir, "--parallel"); err != nil {
		return "", err
	}
	return buildDir, nil
}

func TestManagedProject(ctx context.Context, root string, m Manifest, options BuildOptions, out io.Writer) error {
	options.Tests = true
	buildDir, err := BuildManagedProject(ctx, root, m, options, out)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("ctest"); err != nil {
		return fmt.Errorf("ctest is required to run CPM tests: %w", err)
	}
	return runTool(ctx, out, "ctest", "--test-dir", buildDir, "--output-on-failure")
}

func InitializeGit(ctx context.Context, root string, out io.Writer) error {
	return runTool(ctx, out, "git", "init", "-b", "main", root)
}

func RunManagedProject(ctx context.Context, root string, m Manifest, options BuildOptions, args []string, out io.Writer) error {
	if m.Project.Type != "executable" {
		return fmt.Errorf("cpm run is available only for executable projects")
	}
	buildDir, err := BuildManagedProject(ctx, root, m, options, out)
	if err != nil {
		return err
	}
	return runTool(ctx, out, filepath.Join(buildDir, m.TargetName()), args...)
}

func AvailableTargets(root string) (map[string]bool, error) {
	lock, err := LoadLock(root)
	if err != nil {
		return nil, fmt.Errorf("read lockfile: %w", err)
	}
	available := map[string]bool{}
	for _, p := range lock.Packages {
		for _, target := range p.Targets {
			available[target] = true
		}
		if p.BuildSystem == "header-only" {
			source, err := ParseSource(p.Source)
			if err != nil {
				return nil, err
			}
			available["cpm::"+safePath(source.Owner)+"_"+safePath(source.Repo)] = true
		}
	}
	return available, nil
}

func normalizedSource(root, path string) (string, error) {
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, path)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("source %q: %w", path, err)
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
	extension := strings.ToLower(filepath.Ext(relative))
	if extension != ".c" && extension != ".cc" && extension != ".cpp" && extension != ".cxx" {
		return "", fmt.Errorf("source %q must be a C or C++ source file", path)
	}
	return filepath.ToSlash(relative), nil
}

func sortedTargetNames(targets map[string]bool) []string {
	result := make([]string, 0, len(targets))
	for target := range targets {
		result = append(result, target)
	}
	sort.Strings(result)
	return result
}

func runTool(ctx context.Context, out io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
