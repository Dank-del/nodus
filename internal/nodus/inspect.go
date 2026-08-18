package nodus

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

func Inspect(ctx context.Context, source string, ref string, options map[string]string) (Inspection, error) {
	if err := RequireCMake(ctx); err != nil {
		return Inspection{}, err
	}
	tmp, err := os.MkdirTemp("", "nodus-inspect-")
	if err != nil {
		return Inspection{}, err
	}
	defer os.RemoveAll(tmp)
	manifest := NewManifest("nodus-inspect", false, ProjectOptions{})
	manifest.Dependencies["package"] = Dependency{Source: source, Ref: ref, CMake: CMakePackage{Options: options}}
	if err := atomicWrite(filepath.Join(tmp, "CPM.cmake"), embeddedCPMCMake, 0o644); err != nil {
		return Inspection{}, err
	}
	cmake := "cmake_minimum_required(VERSION 3.14)\nproject(nodus_inspect LANGUAGES C CXX)\ninclude(\"${CMAKE_CURRENT_LIST_DIR}/CPM.cmake\")\n" + string(renderDependencies(manifest)) + "file(WRITE \"${CMAKE_BINARY_DIR}/nodus-source.txt\" \"${package_SOURCE_DIR}\")\n"
	if err := atomicWrite(filepath.Join(tmp, "CMakeLists.txt"), []byte(cmake), 0o644); err != nil {
		return Inspection{}, err
	}
	build := filepath.Join(tmp, "build")
	query := filepath.Join(build, ".cmake", "api", "v1", "query")
	if err := os.MkdirAll(query, 0o755); err != nil {
		return Inspection{}, err
	}
	if err := atomicWrite(filepath.Join(query, "codemodel-v2"), nil, 0o644); err != nil {
		return Inspection{}, err
	}
	cmd := exec.CommandContext(ctx, "cmake", "-S", tmp, "-B", build)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Inspection{}, fmt.Errorf("CMake inspection failed:\n%s", tailLines(string(output), 30))
	}
	targets, err := readTargets(filepath.Join(build, ".cmake", "api", "v1", "reply"))
	if err != nil {
		return Inspection{}, err
	}
	sourcePathBytes, _ := os.ReadFile(filepath.Join(build, "nodus-source.txt"))
	sourcePath := strings.TrimSpace(string(sourcePathBytes))
	targets = uniqueTargetNames(append(targets, discoverAliases(sourcePath)...))
	return Inspection{Repository: source, BuildSystem: "CMake", Targets: consumerTargets(targets), HeaderOnly: isHeaderOnly(sourcePath), Language: detectLanguage(sourcePath)}, nil
}

func discoverAliases(root string) []string {
	var aliases []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "CMakeLists.txt" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			lower := strings.ToLower(line)
			if !strings.HasPrefix(lower, "add_library(") || !strings.Contains(lower, " alias ") {
				continue
			}
			open, close := strings.Index(line, "("), strings.LastIndex(line, ")")
			if open < 0 || close <= open {
				continue
			}
			fields := strings.Fields(line[open+1 : close])
			if len(fields) >= 2 && strings.EqualFold(fields[1], "alias") {
				aliases = append(aliases, fields[0])
			}
		}
		return nil
	})
	return aliases
}

func uniqueTargetNames(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func readTargets(reply string) ([]string, error) {
	entries, err := os.ReadDir(reply)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "codemodel-v2-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(reply, entry.Name()))
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
		if err := json.Unmarshal(data, &model); err != nil {
			return nil, err
		}
		for _, configuration := range model.Configurations {
			for _, target := range configuration.Targets {
				if target.Name != "" {
					set[target.Name] = true
				}
			}
		}
	}
	result := make([]string, 0, len(set))
	for target := range set {
		result = append(result, target)
	}
	sort.Strings(result)
	return result, nil
}

func consumerTargets(targets []string) []string {
	var namespaced []string
	for _, target := range targets {
		lower := strings.ToLower(target)
		if strings.Contains(target, "::") && !strings.Contains(lower, "test") && !strings.Contains(lower, "benchmark") {
			namespaced = append(namespaced, target)
		}
	}
	if len(namespaced) > 0 {
		return namespaced
	}
	var result []string
	for _, target := range targets {
		lower := strings.ToLower(target)
		if !strings.Contains(lower, "test") && !strings.Contains(lower, "example") && !strings.Contains(lower, "benchmark") {
			result = append(result, target)
		}
	}
	return result
}

func detectLanguage(root string) string {
	var c, cpp bool
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".cpp", ".cxx", ".cc", ".hpp", ".hxx", ".hh":
			cpp = true
		case ".c":
			c = true
		}
		return nil
	})
	if c && cpp {
		return "C/C++"
	}
	if cpp {
		return "C++"
	}
	if c {
		return "C"
	}
	return "not detected"
}
func isHeaderOnly(root string) bool {
	var header, source bool
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".h", ".hh", ".hpp", ".hxx":
			header = true
		case ".c", ".cc", ".cpp", ".cxx":
			source = true
		}
		return nil
	})
	return header && !source
}
func tailLines(value string, count int) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n")
}
