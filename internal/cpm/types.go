package cpm

import "fmt"

const (
	ManifestName = "cpm.toml"
	LockName     = "cpm.lock"
	// Version remains pre-release until the package format and CLI contract are stable.
	Version = "0.0.1-alpha"
)

type Manifest struct {
	ProjectName    string
	ProjectVersion string
	PackageName    string
	PackageVersion string
	Dependencies   map[string]string
}

func (m Manifest) Name() string {
	if m.ProjectName != "" {
		return m.ProjectName
	}
	return m.PackageName
}

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
