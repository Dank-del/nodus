package cpm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Git struct{ Command string }

func NewGit() Git { return Git{Command: "git"} }

func (g Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, g.Command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}

func cacheRoot() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cpm", "git"), nil
}
func mirrorPath(root string, source Source) string {
	return filepath.Join(root, source.Host, source.Owner, source.Repo+".git")
}

func (g Git) EnsureMirror(ctx context.Context, source Source) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	mirror := mirrorPath(root, source)
	unlock, err := lockMirror(mirror)
	if err != nil {
		return "", err
	}
	defer unlock()
	if _, err := os.Stat(mirror); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
			return "", err
		}
		if _, err := g.run(ctx, "", "clone", "--mirror", source.URL, mirror); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if _, err := g.run(ctx, mirror, "remote", "set-url", "origin", source.URL); err != nil {
		return "", err
	}
	if _, err := g.run(ctx, mirror, "fetch", "--prune", "--tags", "origin"); err != nil {
		return "", err
	}
	return mirror, nil
}

func (g Git) Resolve(ctx context.Context, source Source) (resolvedRef, commit string, err error) {
	mirror, err := g.EnsureMirror(ctx, source)
	if err != nil {
		return "", "", err
	}
	if source.SelectorKind == "default" {
		tag, found, err := g.latestTag(ctx, mirror)
		if err != nil {
			return "", "", err
		}
		if found {
			source.Selector, source.SelectorKind = tag, "tag"
		} else {
			branch, err := g.defaultBranch(ctx, source.URL)
			if err != nil {
				return "", "", err
			}
			ref := "refs/heads/" + branch
			sha, err := g.revParse(ctx, mirror, ref+"^{commit}")
			return ref, sha, err
		}
	}
	if source.SelectorKind == "tag" {
		ref := "refs/tags/" + source.Selector
		sha, err := g.revParse(ctx, mirror, ref+"^{commit}")
		return ref, sha, err
	}
	// Explicit revisions can be a branch or commit. Fetching the ref makes a
	// short branch name available even when the mirror was created earlier.
	_, _ = g.run(ctx, mirror, "fetch", "origin", source.Selector)
	sha, err := g.revParse(ctx, mirror, source.Selector+"^{commit}")
	if err == nil {
		return source.Selector, sha, nil
	}
	sha, err = g.revParse(ctx, mirror, "FETCH_HEAD^{commit}")
	return source.Selector, sha, err
}

func (g Git) latestTag(ctx context.Context, mirror string) (string, bool, error) {
	out, err := g.run(ctx, mirror, "for-each-ref", "--format=%(refname:short)", "refs/tags")
	if err != nil {
		return "", false, err
	}
	var candidates []semverTag
	for _, tag := range strings.Fields(out) {
		if v, ok := parseStableTag(tag); ok {
			candidates = append(candidates, semverTag{tag, v})
		}
	}
	if len(candidates) == 0 {
		return "", false, nil
	}
	sort.Slice(candidates, func(i, j int) bool { return compareSemver(candidates[i].version, candidates[j].version) > 0 })
	return candidates[0].tag, true, nil
}

func (g Git) defaultBranch(ctx context.Context, url string) (string, error) {
	out, err := g.run(ctx, "", "ls-remote", "--symref", url, "HEAD")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		// git ls-remote --symref emits: ref: refs/heads/main HEAD
		if len(fields) >= 3 && fields[0] == "ref:" && fields[2] == "HEAD" {
			return strings.TrimPrefix(fields[1], "refs/heads/"), nil
		}
	}
	return "", fmt.Errorf("could not determine default branch for %s", url)
}

func (g Git) revParse(ctx context.Context, mirror, rev string) (string, error) {
	return g.run(ctx, mirror, "rev-parse", rev)
}

func (g Git) Materialize(ctx context.Context, root string, p Package) (string, error) {
	source, err := ParseSource(p.Source)
	if err != nil {
		return "", err
	}
	source.URL = p.URL
	mirror, err := g.EnsureMirror(ctx, source)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(root, ".cpm", "packages", source.Host, source.Owner, source.Repo, p.Commit)
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		sha, revErr := g.revParse(ctx, dest, "HEAD")
		if revErr == nil && sha == p.Commit {
			return dest, nil
		}
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if _, err := g.run(ctx, "", "clone", "--no-checkout", mirror, dest); err != nil {
		return "", err
	}
	if _, err := g.run(ctx, dest, "checkout", "--detach", "--force", p.Commit); err != nil {
		return "", err
	}
	return dest, nil
}

type semverTag struct {
	tag     string
	version []int
}

func parseStableTag(tag string) ([]int, bool) {
	v := strings.TrimPrefix(tag, "v")
	if strings.ContainsAny(v, "-+") {
		return nil, false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil, false
	}
	result := make([]int, 3)
	for i, part := range parts {
		if part == "" {
			return nil, false
		}
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, false
			}
			n = n*10 + int(r-'0')
		}
		result[i] = n
	}
	return result, true
}
func compareSemver(a, b []int) int {
	for i := range a {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}
