package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dank-del/cpm/internal/cpm"
)

func TestRootHelpAndVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	app := newApp(&out, &errOut)
	if err := app.Run(context.Background(), []string{"cpm", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "COMMANDS:") || !strings.Contains(out.String(), "inspect") {
		t.Fatalf("unexpected help:\n%s", out.String())
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"cpm", "--version"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "cpm version "+cpm.Version {
		t.Fatalf("version = %q", got)
	}
}

func TestInitUsesFrameworkFlag(t *testing.T) {
	tmp := t.TempDir()
	withWorkingDirectory(t, tmp)
	var out, errOut bytes.Buffer
	if err := newApp(&out, &errOut).Run(context.Background(), []string{"cpm", "init", "--name", "demo"}); err != nil {
		t.Fatal(err)
	}
	m, _, err := cpm.LoadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.Name != "demo" || m.Project.Version != "0.1.0" {
		t.Fatalf("unexpected manifest: %#v", m)
	}
	if !strings.Contains(out.String(), "created cpm.toml") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestNewScaffoldsManagedProject(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	project := filepath.Join(root, "hello")
	if err := newApp(&out, &errOut).Run(context.Background(), []string{"cpm", "new", project, "--type", "library"}); err != nil {
		t.Fatal(err)
	}
	m, _, err := cpm.LoadManifest(project)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Managed() || m.Project.Type != "library" {
		t.Fatalf("unexpected manifest: %#v", m)
	}
}

func TestCommandArgumentValidation(t *testing.T) {
	var out, errOut bytes.Buffer
	for _, args := range [][]string{{"cpm", "add"}, {"cpm", "install", "extra"}, {"cpm", "inspect"}} {
		if err := newApp(&out, &errOut).Run(context.Background(), args); err == nil {
			t.Fatalf("%v unexpectedly succeeded", args)
		}
	}
}

func TestAboutIncludesToolchainStatus(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := newApp(&out, &errOut).Run(context.Background(), []string{"cpm", "about"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"___", "CPM " + cpm.Version, "Environment", "CMake:", "C++ compiler:"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("about output missing %q:\n%s", expected, out.String())
		}
	}
}

func TestTargetSummaryTruncatesLargeTargetLists(t *testing.T) {
	targets := make([]string, 13)
	for i := range targets {
		targets[i] = "target" + string(rune('a'+i))
	}
	summary := targetSummary(targets)
	if !strings.Contains(summary, "targeta") || !strings.Contains(summary, "(+1 more)") {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func withWorkingDirectory(t *testing.T, dir string) {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(current); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, ".")); err != nil {
		t.Fatal(err)
	}
}
