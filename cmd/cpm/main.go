package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Dank-del/cpm/internal/cpm"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "cpm:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		usage(out)
		return nil
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	switch args[0] {
	case "init":
		return initProject(root, args[1:], out)
	case "add":
		return add(ctx, root, args[1:], out)
	case "remove":
		return remove(ctx, root, args[1:], out)
	case "install":
		return install(ctx, root, args[1:], out)
	case "update":
		return update(ctx, root, args[1:], out)
	case "list":
		return list(root, out)
	case "tree":
		return tree(root, out)
	case "inspect":
		return inspect(ctx, args[1:], out)
	case "clean":
		return clean(root, args[1:], out)
	case "help", "--help", "-h":
		usage(out)
		return nil
	default:
		return fmt.Errorf("unknown command %q; run cpm help", args[0])
	}
}

func usage(out io.Writer) {
	fmt.Fprint(out, `Git-native C/C++ source dependency manager

Usage: cpm <command>
  init [--name NAME]       create cpm.toml
  add [--name ALIAS] SRC   add and resolve a GitHub dependency
  remove ALIAS             remove and re-resolve a dependency
  install                  materialize exactly cpm.lock
  update [ALIAS...]        re-resolve dependencies
  list | tree              show locked dependencies
  inspect SRC              inspect a GitHub package
  clean                    remove project-local .cpm artifacts
`)
}

func initProject(root string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", filepath.Base(root), "project name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: cpm init [--name NAME]")
	}
	path := filepath.Join(root, cpm.ManifestName)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", cpm.ManifestName)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := cpm.WriteManifest(root, cpm.Manifest{ProjectName: *name, ProjectVersion: "0.1.0", Dependencies: map[string]string{}}); err != nil {
		return err
	}
	fmt.Fprintf(out, "created %s\n", cpm.ManifestName)
	return nil
}

func add(ctx context.Context, root string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	alias := fs.String("name", "", "dependency alias")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: cpm add [--name ALIAS] github.com/owner/repo[@tag|#ref]")
	}
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	source := fs.Arg(0)
	parsed, err := cpm.ParseSource(source)
	if err != nil {
		return err
	}
	if *alias == "" {
		*alias = cpm.AliasFromSource(parsed)
	}
	if err := cpm.ValidateAlias(*alias); err != nil {
		return err
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	m.Dependencies[*alias] = source
	return resolvePrepareWrite(ctx, root, m, out, "added "+*alias)
}

func remove(ctx context.Context, root string, args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: cpm remove ALIAS")
	}
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	if _, ok := m.Dependencies[args[0]]; !ok {
		return fmt.Errorf("dependency %q is not declared", args[0])
	}
	delete(m.Dependencies, args[0])
	return resolvePrepareWrite(ctx, root, m, out, "removed "+args[0])
}

func update(ctx context.Context, root string, args []string, out io.Writer) error {
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	for _, alias := range args {
		if _, ok := m.Dependencies[alias]; !ok {
			return fmt.Errorf("dependency %q is not declared", alias)
		}
	}
	// Resolution always computes a coherent graph. Arguments validate intent and
	// are reserved for targeted re-resolution when version ranges are added.
	return resolvePrepareWrite(ctx, root, m, out, "updated lockfile")
}

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
	fmt.Fprintf(out, "%s (%d packages)\n", message, len(lock.Packages))
	return nil
}

func install(ctx context.Context, root string, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: cpm install")
	}
	_, manifestBytes, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	lock, err := cpm.LoadLock(root)
	if err != nil {
		return fmt.Errorf("%w; run cpm update", err)
	}
	if lock.ManifestHash != cpm.ManifestHash(manifestBytes) {
		return errors.New("cpm.lock does not match cpm.toml; run cpm update")
	}
	if err := cpm.NewResolver().Prepare(ctx, root, &lock); err != nil {
		return err
	}
	fmt.Fprintf(out, "installed %d packages\n", len(lock.Packages))
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

func inspect(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: cpm inspect github.com/owner/repo[@tag|#ref]")
	}
	s, err := cpm.ParseSource(args[0])
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

func clean(root string, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: cpm clean")
	}
	path := filepath.Join(root, ".cpm")
	if err := os.RemoveAll(path); err != nil {
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
