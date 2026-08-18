package cpm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSourceNormalizesGitHubForms(t *testing.T) {
	for _, input := range []string{
		"github.com/fmtlib/fmt@v11.1.4",
		"https://github.com/fmtlib/fmt.git@v11.1.4",
		"git@github.com:fmtlib/fmt.git@v11.1.4",
	} {
		s, err := ParseSource(input)
		if err != nil {
			t.Fatalf("ParseSource(%q): %v", input, err)
		}
		if s.ID() != "github.com/fmtlib/fmt" || s.Selector != "v11.1.4" || s.SelectorKind != "tag" {
			t.Fatalf("unexpected source for %q: %#v", input, s)
		}
	}
	s, err := ParseSource("github.com/fmtlib/fmt#main")
	if err != nil || s.SelectorKind != "revision" {
		t.Fatalf("revision: %#v, %v", s, err)
	}
}

func TestManifestAndLockRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	m := Manifest{Format: 2, Project: Project{Name: "demo", Version: "0.1.0"}, Dependencies: map[string]string{"fmt": "github.com/fmtlib/fmt@v11.1.4"}}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatal(err)
	}
	loaded, bytes, err := LoadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Dependencies["fmt"] != m.Dependencies["fmt"] {
		t.Fatalf("got %#v", loaded)
	}
	lock := Lock{ManifestHash: ManifestHash(bytes), Packages: []Package{{ID: "github.com/fmtlib/fmt", Name: "fmt", Source: "github.com/fmtlib/fmt", URL: "https://github.com/fmtlib/fmt.git", Requested: "github.com/fmtlib/fmt@v11.1.4", ResolvedRef: "refs/tags/v11.1.4", Commit: "0123456789abcdef", BuildSystem: "cmake", Targets: []string{"fmt::fmt"}}}}
	if err := WriteLock(tmp, lock); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLock(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if got.ManifestHash != lock.ManifestHash || len(got.Packages) != 1 || got.Packages[0].Commit != lock.Packages[0].Commit {
		t.Fatalf("unexpected lock: %#v", got)
	}
}

func TestGenerateCMakeOrdersDependenciesBeforeConsumers(t *testing.T) {
	tmp := t.TempDir()
	lock := Lock{Packages: []Package{
		{ID: "github.com/acme/appdep", Source: "github.com/acme/appdep", Commit: "a", BuildSystem: "cmake", Dependencies: []string{"github.com/acme/base"}},
		{ID: "github.com/acme/base", Source: "github.com/acme/base", Commit: "b", BuildSystem: "header-only"},
	}}
	if err := GenerateCMake(tmp, lock); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(tmp, ".cpm", "generated", "dependencies.cmake"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	base, app := strings.Index(text, "acme/base/b"), strings.Index(text, "acme/appdep/a")
	if base < 0 || app < 0 || base > app {
		t.Fatalf("dependencies not ordered:\n%s", text)
	}
}

func TestGenerateCMakeRejectsCycles(t *testing.T) {
	lock := Lock{Packages: []Package{{ID: "github.com/a/a", Dependencies: []string{"github.com/b/b"}}, {ID: "github.com/b/b", Dependencies: []string{"github.com/a/a"}}}}
	if err := GenerateCMake(t.TempDir(), lock); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestStableTagOrdering(t *testing.T) {
	if _, ok := parseStableTag("v2.0.0-rc1"); ok {
		t.Fatal("pre-release should not be default candidate")
	}
	a, _ := parseStableTag("v11.1.4")
	b, _ := parseStableTag("v11.1.3")
	if compareSemver(a, b) <= 0 {
		t.Fatal("expected newer version to compare higher")
	}
}
