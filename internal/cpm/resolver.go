package cpm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
)

type Resolver struct{ Git Git }

func NewResolver() Resolver { return Resolver{Git: NewGit()} }

// Resolve walks only CPM manifests. It deliberately never guesses dependency
// edges from CMake files, whose syntax and side effects are project-specific.
func (r Resolver) Resolve(ctx context.Context, root string, manifest Manifest, manifestBytes []byte) (Lock, error) {
	packages := map[string]Package{}
	requestedAt := map[string]string{}
	var visit func(alias, raw, parent string) (string, error)
	visit = func(alias, raw, parent string) (string, error) {
		source, err := ParseSource(raw)
		if err != nil {
			return "", fmt.Errorf("%s dependency %q: %w", parent, alias, err)
		}
		ref, sha, err := r.Git.Resolve(ctx, source)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", source.Display(), err)
		}
		id := source.ID()
		if existing, ok := packages[id]; ok {
			if existing.Commit != sha {
				return "", fmt.Errorf("version conflict for %s: %s resolved to %s, but %s resolved to %s; align the manifests", id, requestedAt[id], existing.Commit, source.Display(), sha)
			}
			return id, nil
		}
		p := Package{ID: id, Name: alias, Source: id, URL: source.URL, Requested: source.Display(), ResolvedRef: ref, Commit: sha}
		packages[id] = p
		requestedAt[id] = source.Display()

		// Check out first so a dependency's optional manifest is read from the
		// immutable source commit, never from a remote branch after resolution.
		path, err := r.Git.Materialize(ctx, root, p)
		if err != nil {
			return "", fmt.Errorf("materialize %s: %w", id, err)
		}
		childManifest, _, err := LoadManifest(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("read upstream manifest for %s: %w", id, err)
		}
		if err == nil {
			names := sortedKeys(childManifest.Dependencies)
			for _, childAlias := range names {
				childID, err := visit(childAlias, childManifest.Dependencies[childAlias], id)
				if err != nil {
					return "", err
				}
				p.Dependencies = append(p.Dependencies, childID)
			}
		}
		sort.Strings(p.Dependencies)
		packages[id] = p
		return id, nil
	}
	for _, alias := range sortedKeys(manifest.Dependencies) {
		if _, err := visit(alias, manifest.Dependencies[alias], manifest.Name()); err != nil {
			return Lock{}, err
		}
	}
	result := make([]Package, 0, len(packages))
	for _, p := range packages {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return Lock{Version: 1, ManifestHash: ManifestHash(manifestBytes), Packages: result}, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (r Resolver) Prepare(ctx context.Context, root string, lock *Lock) error {
	for i := range lock.Packages {
		p := &lock.Packages[i]
		path, err := r.Git.Materialize(ctx, root, *p)
		if err != nil {
			return err
		}
		kind, targets, err := ConfigurePackage(ctx, root, *p, path)
		if err != nil {
			return fmt.Errorf("configure %s: %w", p.ID, err)
		}
		p.BuildSystem, p.Targets = kind, targets
	}
	return GenerateCMake(root, *lock)
}

func PackagePath(root string, p Package) (string, error) {
	s, err := ParseSource(p.Source)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".cpm", "packages", s.Host, s.Owner, s.Repo, p.Commit), nil
}
