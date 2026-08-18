package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Dank-del/cpm/internal/cpm"
)

func resolvePrepareWrite(ctx context.Context, root string, m cpm.Manifest, out io.Writer, message string) error {
	b := cpm.MarshalManifest(m)
	resolver := cpm.NewResolver()
	lock, err := resolver.Resolve(ctx, root, m, b)
	if err != nil {
		return err
	}
	if err := resolver.Prepare(ctx, root, &lock); err != nil {
		return err
	}
	if err := cpm.WriteManifest(root, m); err != nil {
		return err
	}
	if err := cpm.WriteLock(root, lock); err != nil {
		return err
	}
	if m.Managed() {
		if err := cpm.RenderManagedProject(root, m, lock); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "%s (%d packages)\n", message, len(lock.Packages))
	return nil
}

func list(root string, out io.Writer) error {
	lock, err := cpm.LoadLock(root)
	if err != nil {
		return err
	}
	for _, p := range lock.Packages {
		fmt.Fprintf(out, "%s %s (%s)\n", p.Name, p.Commit[:min(12, len(p.Commit))], p.Source)
	}
	return nil
}

func tree(root string, out io.Writer) error {
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	lock, err := cpm.LoadLock(root)
	if err != nil {
		return err
	}
	byID := map[string]cpm.Package{}
	for _, p := range lock.Packages {
		byID[p.ID] = p
	}
	fmt.Fprintln(out, m.Name())
	names := make([]string, 0, len(m.Dependencies))
	for n := range m.Dependencies {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, name := range names {
		s, err := cpm.ParseSource(m.Dependencies[name])
		if err != nil {
			return err
		}
		printTree(out, byID, s.ID(), "", i == len(names)-1)
	}
	return nil
}

func printTree(out io.Writer, byID map[string]cpm.Package, id, prefix string, last bool) {
	p, ok := byID[id]
	if !ok {
		fmt.Fprintf(out, "%s%s %s (missing)\n", prefix, branch(last), id)
		return
	}
	fmt.Fprintf(out, "%s%s %s %s\n", prefix, branch(last), p.Name, p.Requested)
	deps := append([]string(nil), p.Dependencies...)
	sort.Strings(deps)
	next := prefix
	if last {
		next += "   "
	} else {
		next += "│  "
	}
	for i, dep := range deps {
		printTree(out, byID, dep, next, i == len(deps)-1)
	}
}
func branch(last bool) string {
	if last {
		return "└── "
	}
	return "├── "
}

func inspect(ctx context.Context, source string, out io.Writer) error {
	s, err := cpm.ParseSource(source)
	if err != nil {
		return err
	}
	ref, sha, err := cpm.NewGit().Resolve(ctx, s)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "cpm-inspect-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	p := cpm.Package{ID: s.ID(), Name: s.Repo, Source: s.ID(), URL: s.URL, Requested: s.Display(), ResolvedRef: ref, Commit: sha}
	path, err := cpm.NewGit().Materialize(ctx, tmp, p)
	if err != nil {
		return err
	}
	kind, targets, err := cpm.ConfigurePackage(ctx, tmp, p, path)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Repository: %s\nResolved ref: %s\nCommit: %s\nBuild system: %s\n", p.Source, ref, sha, kind)
	if len(targets) > 0 {
		fmt.Fprintf(out, "Targets: %s\n", strings.Join(targets, ", "))
	}
	return nil
}

func clean(root string, out io.Writer) error {
	if err := os.RemoveAll(filepath.Join(root, ".cpm")); err != nil {
		return err
	}
	fmt.Fprintln(out, "removed .cpm project artifacts")
	return nil
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
