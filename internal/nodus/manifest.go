package nodus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func NewManifest(name string, managed bool, options ProjectOptions) Manifest {
	if options.Type == "" {
		options.Type = "executable"
	}
	if options.CPPStandard == 0 {
		options.CPPStandard = 20
	}
	return Manifest{Format: 1, Project: Project{Name: name, Version: "0.1.0", Language: "cxx", Backend: BackendCMake, Managed: managed, Type: options.Type, CPPStandard: options.CPPStandard}, Build: Build{Sources: []string{}, Links: []string{}}, Dependencies: map[string]Dependency{}}
}

func LoadManifest(root string) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", ManifestName, err)
	}
	return ParseManifest(b)
}

func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse %s: %w", ManifestName, err)
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]Dependency{}
	}
	if m.Format != 1 {
		return m, fmt.Errorf("unsupported %s format %d", ManifestName, m.Format)
	}
	if m.Project.Name == "" {
		return m, fmt.Errorf("%s must define [project].name", ManifestName)
	}
	if m.Project.Backend != BackendCMake {
		return m, fmt.Errorf("backend %q is not available in this alpha", m.Project.Backend)
	}
	if m.Project.Type != "" && m.Project.Type != "executable" && m.Project.Type != "library" {
		return m, fmt.Errorf("project type must be executable or library")
	}
	if m.Project.Managed && m.Project.CPPStandard < 11 {
		return m, fmt.Errorf("managed projects require cpp_standard >= 11")
	}
	for alias, dep := range m.Dependencies {
		if err := ValidateAlias(alias); err != nil {
			return m, err
		}
		if strings.TrimSpace(dep.Source) == "" {
			return m, fmt.Errorf("dependency %q has no source", alias)
		}
		if err := validateCMakeMap(dep.CMake.Options, "option"); err != nil {
			return m, fmt.Errorf("dependency %q: %w", alias, err)
		}
		if err := validateCMakeMap(dep.CMake.Arguments, "argument"); err != nil {
			return m, fmt.Errorf("dependency %q: %w", alias, err)
		}
	}
	return m, nil
}

func WriteManifest(root string, m Manifest) error {
	if m.Dependencies == nil {
		m.Dependencies = map[string]Dependency{}
	}
	data, err := toml.Marshal(m)
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(root, ManifestName), data, 0o644)
}

func ValidateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("dependency name must not be empty")
	}
	for _, r := range alias {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return fmt.Errorf("dependency name %q may only contain letters, digits, '_' and '-'", alias)
		}
	}
	return nil
}

func validateCMakeMap(values map[string]string, kind string) error {
	for key, value := range values {
		if !validCMakeIdentifier(key) || strings.TrimSpace(value) == "" {
			return fmt.Errorf("CMake %s %q must use a non-empty identifier and value", kind, key)
		}
	}
	return nil
}

func validCMakeIdentifier(value string) bool {
	for index, r := range value {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || index > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return value != ""
}

func SortedDependencies(m Manifest) []PackageInfo {
	aliases := make([]string, 0, len(m.Dependencies))
	for alias := range m.Dependencies {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	items := make([]PackageInfo, 0, len(aliases))
	for _, alias := range aliases {
		dep := m.Dependencies[alias]
		items = append(items, PackageInfo{Alias: alias, Source: dep.Source, Ref: dep.Ref})
	}
	return items
}
