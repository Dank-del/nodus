package cpm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

func LoadManifest(root string) (Manifest, []byte, error) {
	p := filepath.Join(root, ManifestName)
	b, err := os.ReadFile(p)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("read %s: %w", p, err)
	}
	m, err := ParseManifest(string(b))
	return m, b, err
}

func ParseManifest(text string) (Manifest, error) {
	var m Manifest
	if err := toml.Unmarshal([]byte(text), &m); err != nil {
		return m, fmt.Errorf("parse %s: %w", ManifestName, err)
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	if m.CMakeOptions == nil {
		m.CMakeOptions = map[string][]string{}
	}
	if m.Name() == "" {
		return m, fmt.Errorf("%s must define [project].name or [package].name", ManifestName)
	}
	if m.Format != 0 && m.Format != 2 {
		return m, fmt.Errorf("unsupported %s format %d", ManifestName, m.Format)
	}
	if m.Project.Managed {
		if m.Format != 2 {
			return m, fmt.Errorf("managed projects require format = 2")
		}
		if m.Project.Type != "executable" && m.Project.Type != "library" {
			return m, fmt.Errorf("managed project type must be executable or library")
		}
		if m.Build.CPPStandard < 11 {
			return m, fmt.Errorf("managed projects require cpp_standard >= 11")
		}
		if err := validatePaths(m.Build.Sources); err != nil {
			return m, err
		}
	}
	for alias := range m.Dependencies {
		if err := ValidateAlias(alias); err != nil {
			return m, err
		}
	}
	for alias, options := range m.CMakeOptions {
		if _, ok := m.Dependencies[alias]; !ok {
			return m, fmt.Errorf("CMake options are configured for undeclared dependency %q", alias)
		}
		if err := ValidateCMakeOptions(options); err != nil {
			return m, err
		}
	}
	return m, nil
}

func WriteManifest(root string, m Manifest) error {
	return atomicWrite(filepath.Join(root, ManifestName), MarshalManifest(m), 0o644)
}

func MarshalManifest(m Manifest) []byte {
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	if m.CMakeOptions == nil {
		m.CMakeOptions = map[string][]string{}
	}
	if m.Format == 0 {
		m.Format = 2
	}
	if m.Project.Version == "" {
		m.Project.Version = "0.1.0"
	}
	if m.Project.Managed {
		normalizeManagedManifest(&m)
	}
	data, err := toml.Marshal(m)
	if err != nil {
		panic(fmt.Sprintf("marshal manifest: %v", err))
	}
	return data
}

func NewManagedManifest(name, projectType string, standard int) Manifest {
	m := Manifest{Format: 2, Project: Project{Name: name, Version: "0.1.0", Managed: true, Type: projectType}, Build: Build{CPPStandard: standard, AutoDiscoverSources: true, Sources: []string{}, Links: []string{}}, Dependencies: map[string]string{}, CMakeOptions: map[string][]string{}}
	normalizeManagedManifest(&m)
	return m
}

func normalizeManagedManifest(m *Manifest) {
	m.Build.Sources = uniqueSorted(m.Build.Sources)
	m.Build.Links = uniqueSorted(m.Build.Links)
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

func validatePaths(paths []string) error {
	for _, path := range paths {
		if filepath.IsAbs(path) || path == ".." || len(path) >= 3 && path[:3] == "../" {
			return fmt.Errorf("managed source path %q must be project-relative", path)
		}
	}
	return nil
}
