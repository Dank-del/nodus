package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Dank-del/nodus/internal/nodus"
)

func TestRootHelpAndVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	app := newApp(&out, &errOut)
	if err := app.Run(context.Background(), []string{"nodus", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "inspect") || !strings.Contains(out.String(), "CPM.cmake") {
		t.Fatalf("unexpected help:\n%s", out.String())
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"nodus", "version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != nodus.Version {
		t.Fatalf("version = %q", out.String())
	}
}

func TestAboutShowsBackendStatus(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := newApp(&out, &errOut).Run(context.Background(), []string{"nodus", "about"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Nodus " + nodus.Version, "CPM.cmake " + nodus.CPMCMakeVersion, "Environment", "C++ compiler"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("about missing %q:\n%s", expected, out.String())
		}
	}
}
