package cpm

import (
	"fmt"
	"strings"
)

const (
	ManifestName = "cpm.toml"
	LockName     = "cpm.lock"
	// Version remains pre-release until the package format and CLI contract are stable.
	Version = "0.0.1-alpha"
)

type Manifest struct {
	Format       int                 `toml:"format"`
	Project      Project             `toml:"project"`
	Package      PackageMetadata     `toml:"package,omitempty"`
	Build        Build               `toml:"build"`
	Dependencies map[string]string   `toml:"dependencies"`
	CMakeOptions map[string][]string `toml:"cmake_options,omitempty"`
}

type Project struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
	Managed bool   `toml:"managed"`
	Type    string `toml:"type"`
}

type PackageMetadata struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type Build struct {
	CPPStandard         int      `toml:"cpp_standard"`
	AutoDiscoverSources bool     `toml:"auto_discover_sources"`
	Sources             []string `toml:"sources"`
	Links               []string `toml:"links"`
}

func (m Manifest) Name() string {
	if m.Project.Name != "" {
		return m.Project.Name
	}
	return m.Package.Name
}

func (m Manifest) Managed() bool { return m.Format == 2 && m.Project.Managed }

type Source struct {
	Host, Owner, Repo string
	URL               string
	Selector          string
	SelectorKind      string // tag, revision, default
}

func (s Source) ID() string { return s.Host + "/" + s.Owner + "/" + s.Repo }
func (s Source) Display() string {
	if s.Selector == "" {
		return s.ID()
	}
	if s.SelectorKind == "tag" {
		return s.ID() + "@" + s.Selector
	}
	return s.ID() + "#" + s.Selector
}

type Package struct {
	ID           string
	Name         string
	Source       string
	URL          string
	Requested    string
	ResolvedRef  string
	Commit       string
	BuildSystem  string
	Targets      []string
	Dependencies []string
	CMakeOptions []string
}

type Lock struct {
	Version      int
	ManifestHash string
	Packages     []Package
}

func (l Lock) Package(id string) (Package, bool) {
	for _, p := range l.Packages {
		if p.ID == id {
			return p, true
		}
	}
	return Package{}, false
}

func AliasFromSource(s Source) string {
	if s.Repo == "" {
		return ""
	}
	return s.Repo
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

// ValidateCMakeOptions accepts the portable subset shared by CMake's -D flag
// and the CPM manifest: KEY=VALUE. Values remain opaque to CPM.
func ValidateCMakeOptions(options []string) error {
	for _, option := range options {
		key, value, ok := strings.Cut(option, "=")
		if !ok || key == "" || value == "" {
			return fmt.Errorf("CMake option %q must use KEY=VALUE", option)
		}
		for index, r := range key {
			if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || index > 0 && r >= '0' && r <= '9') {
				return fmt.Errorf("CMake option key %q may only contain letters, digits, and '_'", key)
			}
		}
	}
	return nil
}
