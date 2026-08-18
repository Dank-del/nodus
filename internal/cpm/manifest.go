package cpm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func LoadManifest(root string) (Manifest, []byte, error) {
	p := filepath.Join(root, ManifestName)
	b, err := os.ReadFile(p)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("read %s: %w", p, err)
	}
	m, err := ParseManifest(string(b))
	return m, b, err
}

func ParseManifest(text string) (Manifest, error) {
	m := Manifest{Dependencies: map[string]string{}}
	section := ""
	s := bufio.NewScanner(strings.NewReader(text))
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return m, fmt.Errorf("%s:%d: expected key = value", ManifestName, lineNo)
		}
		key := strings.TrimSpace(parts[0])
		raw := strings.TrimSpace(parts[1])
		value, err := strconv.Unquote(raw)
		if err != nil {
			return m, fmt.Errorf("%s:%d: values must be quoted strings: %w", ManifestName, lineNo, err)
		}
		switch section {
		case "project":
			if key == "name" {
				m.ProjectName = value
			} else if key == "version" {
				m.ProjectVersion = value
			}
		case "package":
			if key == "name" {
				m.PackageName = value
			} else if key == "version" {
				m.PackageVersion = value
			}
		case "dependencies":
			if err := ValidateAlias(key); err != nil {
				return m, err
			}
			m.Dependencies[key] = value
		default:
			return m, fmt.Errorf("%s:%d: unsupported section [%s]", ManifestName, lineNo, section)
		}
	}
	if err := s.Err(); err != nil {
		return m, err
	}
	if m.Name() == "" {
		return m, fmt.Errorf("%s must define [project].name or [package].name", ManifestName)
	}
	return m, nil
}

func WriteManifest(root string, m Manifest) error {
	return atomicWrite(filepath.Join(root, ManifestName), MarshalManifest(m), 0o644)
}

func MarshalManifest(m Manifest) []byte {
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	var b strings.Builder
	if m.ProjectName != "" {
		fmt.Fprintf(&b, "[project]\nname = %q\nversion = %q\n", m.ProjectName, valueOr(m.ProjectVersion, Version))
	} else {
		fmt.Fprintf(&b, "[package]\nname = %q\nversion = %q\n", m.PackageName, valueOr(m.PackageVersion, Version))
	}
	b.WriteString("\n[dependencies]\n")
	names := make([]string, 0, len(m.Dependencies))
	for name := range m.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "%s = %q\n", name, m.Dependencies[name])
	}
	return []byte(b.String())
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
