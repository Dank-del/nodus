package main

import (
	"bytes"
	"testing"

	"github.com/Dank-del/cpm/internal/cpm"
)

func TestProgressIsSilentForNonTerminalWriters(t *testing.T) {
	var out bytes.Buffer
	activity := startActivityProgress(&out, "Resolving dependencies")
	activity.Stop(true)

	packages := newPackageProgress(&out, 1, "Preparing packages")
	packages.StartPackage(cpm.Package{Name: "fmt"})
	packages.CompletePackage(cpm.Package{Name: "fmt"})
	packages.Stop(true)

	if out.Len() != 0 {
		t.Fatalf("non-terminal writer received progress output: %q", out.String())
	}
}
