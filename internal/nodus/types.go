// Package nodus contains Nodus' backend-neutral project model and its built-in
// CMake backend. Dependency fetching and CMake package locks are delegated to
// CPM.cmake; Nodus deliberately does not maintain a second resolver.
package nodus

const (
	ManifestName      = "nodus.toml"
	BackendCMake      = "cmake"
	Version           = "0.0.1-alpha"
	CPMCMakeVersion   = "0.43.1"
	CPMCMakeSHA256    = "aa1640d27e0944332c3319d9427f8b8d711c5c95000c050ed1e46d29bcd4e763"
	managedBlockStart = "# >>> nodus:begin"
	managedBlockEnd   = "# <<< nodus:end"
)

type Manifest struct {
	Format       int                   `toml:"format"`
	Project      Project               `toml:"project"`
	Build        Build                 `toml:"build"`
	Dependencies map[string]Dependency `toml:"dependencies"`
}

type Project struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Language    string `toml:"language"`
	Backend     string `toml:"backend"`
	Managed     bool   `toml:"managed"`
	Type        string `toml:"type"`
	CPPStandard int    `toml:"cpp_standard"`
}

type Build struct {
	Sources []string `toml:"sources,omitempty"`
	Links   []string `toml:"links,omitempty"`
}

// Dependency is portable at the source/ref level. CMake-specific escape hatches
// live below CMake, keeping the root model usable by future adapters.
type Dependency struct {
	Source string       `toml:"source"`
	Ref    string       `toml:"ref,omitempty"`
	CMake  CMakePackage `toml:"cmake,omitempty"`
}

type CMakePackage struct {
	Options   map[string]string `toml:"options,omitempty"`
	Arguments map[string]string `toml:"arguments,omitempty"`
}

type ProjectOptions struct {
	Type        string
	CPPStandard int
}

type BuildOptions struct {
	Release bool
	Tests   bool
}

type PackageInfo struct {
	Alias  string
	Source string
	Ref    string
}

type Inspection struct {
	Repository  string
	BuildSystem string
	Targets     []string
	HeaderOnly  bool
	Language    string
	LogPath     string
}
