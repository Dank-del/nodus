package cpm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func ConfigurePackage(ctx context.Context, root string, p Package, source string) (string, []string, error) {
	if _, err := os.Stat(filepath.Join(source, "CMakeLists.txt")); err == nil {
		return configureCMake(ctx, root, p, source)
	}
	if info, err := os.Stat(filepath.Join(source, "include")); err == nil && info.IsDir() {
		return "header-only", nil, nil
	}
	return "", nil, fmt.Errorf("could not determine how to consume package; no CMakeLists.txt or conventional include/ directory found")
}

func configureCMake(ctx context.Context, root string, p Package, source string) (string, []string, error) {
	if err := requireCMake(ctx); err != nil {
		return "", nil, fmt.Errorf("CMake >= 3.14 is required to configure CMake dependencies; install cmake and retry: %w", err)
	}
	build := filepath.Join(root, ".cpm", "build", safePath(p.ID), p.Commit)
	query := filepath.Join(build, ".cmake", "api", "v1", "query")
	if err := os.MkdirAll(query, 0o755); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(query, "codemodel-v2"), nil, 0o644); err != nil {
		return "", nil, err
	}
	args := []string{"-S", source, "-B", build}
	for _, option := range p.CMakeOptions {
		args = append(args, "-D"+option)
	}
	cmd := exec.CommandContext(ctx, "cmake", args...)
	out, err := cmd.CombinedOutput()
	_ = os.WriteFile(filepath.Join(build, "cpm-configure.log"), out, 0o644)
	if err != nil {
		return "", nil, fmt.Errorf("cmake configuration failed (log: %s): %w\n%s", filepath.Join(build, "cpm-configure.log"), err, tailLines(string(out), 20))
	}
	targets, err := readTargets(filepath.Join(build, ".cmake", "api", "v1", "reply"))
	if err != nil {
		return "", nil, err
	}
	aliases, err := discoverCMakeAliases(source)
	if err != nil {
		return "", nil, err
	}
	targets = uniqueSorted(append(targets, aliases...))
	return "cmake", targets, nil
}

func tailLines(value string, count int) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) <= count {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-count:], "\n")
}

func requireCMake(ctx context.Context) error {
	if _, err := exec.LookPath("cmake"); err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "cmake", "--version").Output()
	if err != nil {
		return err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return fmt.Errorf("unrecognized CMake version")
	}
	parts := strings.Split(fields[2], ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	major, minor := 0, 0
	if _, err := fmt.Sscanf(parts[0]+" "+parts[1], "%d %d", &major, &minor); err != nil {
		return err
	}
	if major < 3 || (major == 3 && minor < 14) {
		return fmt.Errorf("CMake %s is too old", fields[2])
	}
	return nil
}

// RequireCMake checks the system CMake prerequisite without configuring a
// dependency. It is used by `cpm about` to report CMake readiness.
func RequireCMake() error { return requireCMake(context.Background()) }

func readTargets(reply string) ([]string, error) {
	entries, err := os.ReadDir(reply)
	if err != nil {
		return nil, fmt.Errorf("read CMake File API reply: %w", err)
	}
	set := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "codemodel-v2-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(reply, entry.Name()))
		if err != nil {
			return nil, err
		}
		var model struct {
			Configurations []struct {
				Targets []struct {
					Name string `json:"name"`
				} `json:"targets"`
			} `json:"configurations"`
		}
		if err := json.Unmarshal(b, &model); err != nil {
			return nil, err
		}
		for _, config := range model.Configurations {
			for _, target := range config.Targets {
				if target.Name != "" {
					set[target.Name] = true
				}
			}
		}
	}
	targets := make([]string, 0, len(set))
	for t := range set {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	return targets, nil
}

func safePath(value string) string {
	return strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(value)
}

func discoverCMakeAliases(source string) ([]string, error) {
	var cmakeFiles []string
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "build") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && entry.Name() == "CMakeLists.txt" {
			cmakeFiles = append(cmakeFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(cmakeFiles)
	variables := map[string]string{}
	aliases := []string{}
	for _, path := range cmakeFiles {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(contents), "\n") {
			line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "project(") {
				fields := cmakeFields(line)
				if len(fields) > 0 {
					variables["PROJECT_NAME"] = expandCMakeVariables(fields[0], variables)
				}
				continue
			}
			if strings.HasPrefix(lower, "set(") {
				fields := cmakeFields(line)
				if len(fields) >= 2 {
					variables[fields[0]] = expandCMakeVariables(fields[1], variables)
				}
				continue
			}
			if !(strings.HasPrefix(lower, "add_library(") || strings.HasPrefix(lower, "add_executable(")) || !strings.Contains(strings.ToUpper(line), " ALIAS") {
				continue
			}
			fields := cmakeFields(line)
			if len(fields) >= 2 && strings.EqualFold(fields[1], "ALIAS") {
				aliases = append(aliases, expandCMakeVariables(fields[0], variables))
			}
		}
	}
	return uniqueSorted(aliases), nil
}

func cmakeFields(line string) []string {
	open, close := strings.Index(line, "("), strings.LastIndex(line, ")")
	if open < 0 || close <= open {
		return nil
	}
	return strings.Fields(strings.TrimSpace(line[open+1 : close]))
}

func expandCMakeVariables(value string, variables map[string]string) string {
	for range len(variables) + 1 {
		start := strings.Index(value, "${")
		if start < 0 {
			return value
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return value
		}
		end += start
		name := value[start+2 : end]
		replacement, ok := variables[name]
		if !ok {
			return value
		}
		value = value[:start] + replacement + value[end+1:]
	}
	return value
}
