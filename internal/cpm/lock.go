package cpm

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func ManifestHash(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return fmt.Sprintf("sha256:%x", sum[:])
}

func LoadLock(root string) (Lock, error) {
	b, err := os.ReadFile(filepath.Join(root, LockName))
	if err != nil {
		return Lock{}, fmt.Errorf("read %s: %w", LockName, err)
	}
	return ParseLock(string(b))
}

func ParseLock(text string) (Lock, error) {
	var lock Lock
	var current *Package
	s := bufio.NewScanner(strings.NewReader(text))
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		if line == "[[package]]" {
			lock.Packages = append(lock.Packages, Package{})
			current = &lock.Packages[len(lock.Packages)-1]
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return lock, fmt.Errorf("%s:%d: expected key = value", LockName, lineNo)
		}
		key, raw := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if current == nil {
			switch key {
			case "version":
				n, err := strconv.Atoi(raw)
				if err != nil {
					return lock, err
				}
				lock.Version = n
			case "manifest_hash":
				v, err := strconv.Unquote(raw)
				if err != nil {
					return lock, err
				}
				lock.ManifestHash = v
			default:
				return lock, fmt.Errorf("%s:%d: unknown root key %q", LockName, lineNo, key)
			}
			continue
		}
		if err := setPackageField(current, key, raw); err != nil {
			return lock, fmt.Errorf("%s:%d: %w", LockName, lineNo, err)
		}
	}
	if err := s.Err(); err != nil {
		return lock, err
	}
	if lock.Version != 1 {
		return lock, fmt.Errorf("unsupported %s version %d", LockName, lock.Version)
	}
	for _, p := range lock.Packages {
		if p.ID == "" || p.Commit == "" || p.Source == "" {
			return lock, fmt.Errorf("invalid lock package: id, source, and commit are required")
		}
	}
	return lock, nil
}

func setPackageField(p *Package, key, raw string) error {
	if key == "dependencies" || key == "targets" || key == "cmake_options" {
		v, err := parseStringArray(raw)
		if err != nil {
			return err
		}
		if key == "dependencies" {
			p.Dependencies = v
		} else if key == "targets" {
			p.Targets = v
		} else {
			p.CMakeOptions = v
		}
		return nil
	}
	v, err := strconv.Unquote(raw)
	if err != nil {
		return err
	}
	switch key {
	case "id":
		p.ID = v
	case "name":
		p.Name = v
	case "source":
		p.Source = v
	case "url":
		p.URL = v
	case "requested":
		p.Requested = v
	case "resolved_ref":
		p.ResolvedRef = v
	case "commit":
		p.Commit = v
	case "build_system":
		p.BuildSystem = v
	default:
		return fmt.Errorf("unknown package key %q", key)
	}
	return nil
}

func parseStringArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("expected string array")
	}
	raw = strings.TrimSpace(raw[1 : len(raw)-1])
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v, err := strconv.Unquote(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func WriteLock(root string, lock Lock) error {
	lock.Version = 1
	sort.Slice(lock.Packages, func(i, j int) bool { return lock.Packages[i].ID < lock.Packages[j].ID })
	var b strings.Builder
	fmt.Fprintf(&b, "version = 1\nmanifest_hash = %q\n", lock.ManifestHash)
	for _, p := range lock.Packages {
		fmt.Fprintf(&b, "\n[[package]]\nid = %q\nname = %q\nsource = %q\nurl = %q\nrequested = %q\nresolved_ref = %q\ncommit = %q\nbuild_system = %q\n", p.ID, p.Name, p.Source, p.URL, p.Requested, p.ResolvedRef, p.Commit, p.BuildSystem)
		fmt.Fprintf(&b, "targets = %s\ndependencies = %s\ncmake_options = %s\n", quoteArray(p.Targets), quoteArray(p.Dependencies), quoteArray(p.CMakeOptions))
	}
	return atomicWrite(filepath.Join(root, LockName), []byte(b.String()), 0o644)
}

func quoteArray(values []string) string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	q := make([]string, len(values))
	for i, v := range values {
		q[i] = strconv.Quote(v)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cpm-tmp-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
