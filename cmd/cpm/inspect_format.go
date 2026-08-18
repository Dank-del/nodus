package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Dank-del/cpm/internal/cpm"
)

func renderInspection(out io.Writer, pkg cpm.Package, source, buildSystem string, targets []string) {
	headerOnly := isHeaderOnlySource(source)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Repository\t%s\n", pkg.Source)
	fmt.Fprintf(tw, "Version\t%s\n", displayVersion(pkg.ResolvedRef))
	fmt.Fprintf(tw, "Commit\t%s\n", shortCommit(pkg.Commit))
	fmt.Fprintf(tw, "Language\t%s\n\n", detectLanguage(source))
	fmt.Fprintf(tw, "Build system\t%s\n", displayBuildSystem(buildSystem))
	if buildSystem == "cmake" {
		fmt.Fprintf(tw, "CMake targets\t%s\n", targetSummary(consumerTargets(targets)))
	}
	fmt.Fprintf(tw, "Header-only\t%s\n", yesNo(headerOnly))
	_ = tw.Flush()

	fmt.Fprintln(out, "\nPlatforms")
	for _, platform := range []string{"Linux", "macOS", "Windows"} {
		fmt.Fprintf(out, "  %-14s %s\n", platform, platformStatus(platform, headerOnly))
	}

	fmt.Fprintln(out, "\nDependencies")
	if len(pkg.Dependencies) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		for _, dependency := range pkg.Dependencies {
			fmt.Fprintf(out, "  %s\n", dependency)
		}
	}
	fmt.Fprintln(out, "\nCPM compatible  ✓")
}

func displayVersion(ref string) string {
	if tag, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
		return tag
	}
	if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return branch + " (branch)"
	}
	return ref
}

func displayBuildSystem(value string) string {
	switch value {
	case "cmake":
		return "CMake"
	case "header-only":
		return "Header-only"
	default:
		return value
	}
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func detectLanguage(source string) string {
	var c, cpp bool
	_ = filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if ignoreInspectionDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".cpp", ".cxx", ".cc", ".hpp", ".hxx", ".hh":
			cpp = true
		case ".c":
			c = true
		}
		return nil
	})
	switch {
	case c && cpp:
		return "C/C++"
	case cpp:
		return "C++"
	case c:
		return "C"
	default:
		return "not detected"
	}
}

func isHeaderOnlySource(source string) bool {
	hasHeader, hasSource := false, false
	_ = filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if ignoreInspectionDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".h", ".hh", ".hpp", ".hxx":
			hasHeader = true
		case ".c", ".cc", ".cpp", ".cxx":
			hasSource = true
		}
		return nil
	})
	return hasHeader && !hasSource
}

func ignoreInspectionDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "build", "test", "tests", "example", "examples", "benchmark", "benchmarks", "doc", "docs", "tool", "tools", "script", "scripts":
		return true
	default:
		return false
	}
}

func consumerTargets(targets []string) []string {
	var namespaced []string
	for _, target := range targets {
		lower := strings.ToLower(target)
		if strings.Contains(target, "::") && !strings.Contains(lower, "gtest") && !strings.Contains(lower, "catch") && !strings.Contains(lower, "doctest") {
			namespaced = append(namespaced, target)
		}
	}
	if len(namespaced) > 0 {
		return uniqueStrings(namespaced)
	}
	var candidates []string
	for _, target := range targets {
		lower := strings.ToLower(target)
		if strings.Contains(lower, "test") || strings.Contains(lower, "example") || strings.Contains(lower, "benchmark") || strings.Contains(lower, "continuous") || strings.Contains(lower, "experimental") || strings.Contains(lower, "nightly") {
			continue
		}
		candidates = append(candidates, target)
	}
	return uniqueStrings(candidates)
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func platformStatus(platform string, headerOnly bool) string {
	if headerOnly {
		return "✓ header-only C++"
	}
	if strings.EqualFold(platform, runtime.GOOS) || platform == "macOS" && runtime.GOOS == "darwin" {
		return "✓ configured on this host"
	}
	return "? not verified"
}
