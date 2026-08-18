package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Dank-del/nodus/internal/nodus"
	cli "github.com/urfave/cli/v3"
)

func newAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 1); err != nil {
		return err
	}
	root, err := filepath.Abs(cmd.Args().First())
	if err != nil {
		return err
	}
	m, err := nodus.CreateProject(root, filepath.Base(root), nodus.ProjectOptions{Type: cmd.String("type"), CPPStandard: cmd.Int("cpp-standard")})
	if err != nil {
		return err
	}
	if cmd.Bool("git") {
		if err := nodus.InitializeGit(ctx, root, writer(cmd)); err != nil {
			return err
		}
	}
	if err := nodus.NewCMakeBackend().RefreshLock(ctx, root, writer(cmd)); err != nil {
		return err
	}
	fmt.Fprintf(writer(cmd), "created Nodus project %s (%s)\n", m.Project.Name, root)
	return nil
}

func initAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	m, err := nodus.InitProject(root, cmd.String("name"))
	if err != nil {
		return err
	}
	if err := nodus.NewCMakeBackend().RefreshLock(ctx, root, writer(cmd)); err != nil {
		return err
	}
	fmt.Fprintf(writer(cmd), "initialized %s for %s\n", nodus.ManifestName, m.Project.Name)
	return nil
}

func addAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 1); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	source, sourceRef, err := splitSourceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	ref := cmd.String("ref")
	if ref != "" && sourceRef != "" {
		return fmt.Errorf("use either SOURCE@ref / SOURCE#ref or --ref, not both")
	}
	if ref == "" {
		ref = sourceRef
	}
	alias := cmd.String("name")
	if alias == "" {
		alias = aliasFromSource(source)
	}
	if err := nodus.ValidateAlias(alias); err != nil {
		return err
	}
	options, err := keyValues(cmd.StringSlice("cmake-option"))
	if err != nil {
		return fmt.Errorf("--cmake-option: %w", err)
	}
	arguments, err := keyValues(cmd.StringSlice("cmake-arg"))
	if err != nil {
		return fmt.Errorf("--cmake-arg: %w", err)
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]nodus.Dependency{}
	}
	m.Dependencies[alias] = nodus.Dependency{Source: source, Ref: ref, CMake: nodus.CMakePackage{Options: options, Arguments: arguments}}
	if err := nodus.SyncDependencies(ctx, root, m, writer(cmd)); err != nil {
		return err
	}
	fmt.Fprintf(writer(cmd), "added %s (%s)\n", alias, source)
	return nil
}

func removeAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 1); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	alias := cmd.Args().First()
	if _, ok := m.Dependencies[alias]; !ok {
		return fmt.Errorf("dependency %q is not declared", alias)
	}
	delete(m.Dependencies, alias)
	if err := nodus.SyncDependencies(ctx, root, m, writer(cmd)); err != nil {
		return err
	}
	fmt.Fprintf(writer(cmd), "removed %s\n", alias)
	return nil
}

func installAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	root, _, err := project()
	if err != nil {
		return err
	}
	return nodus.NewCMakeBackend().Install(ctx, root, writer(cmd))
}
func updateAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	if err := nodus.NewCMakeBackend().Ensure(root, m); err != nil {
		return err
	}
	return nodus.NewCMakeBackend().RefreshLock(ctx, root, writer(cmd))
}

func listAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	_, m, err := project()
	if err != nil {
		return err
	}
	items := nodus.SortedDependencies(m)
	if len(items) == 0 {
		fmt.Fprintln(writer(cmd), "no dependencies")
		return nil
	}
	for _, item := range items {
		ref := item.Ref
		if ref == "" {
			ref = "unversioned"
		}
		fmt.Fprintf(writer(cmd), "%s\t%s\t%s\n", item.Alias, item.Source, ref)
	}
	return nil
}

func sourceAddAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOne(cmd); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	if err := nodus.AddSources(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(writer(cmd), "updated sources")
	return nil
}
func sourceRemoveAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOne(cmd); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	if err := nodus.RemoveSources(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(writer(cmd), "updated sources")
	return nil
}
func linkAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOne(cmd); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	if err := nodus.AddLinks(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(writer(cmd), "updated CMake links")
	return nil
}
func unlinkAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOne(cmd); err != nil {
		return err
	}
	root, m, err := project()
	if err != nil {
		return err
	}
	if err := nodus.RemoveLinks(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(writer(cmd), "updated CMake links")
	return nil
}

func buildAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	root, _, err := project()
	if err != nil {
		return err
	}
	_, err = nodus.BuildProject(ctx, root, buildOptions(cmd), writer(cmd))
	return err
}
func runAction(ctx context.Context, cmd *cli.Command) error {
	root, m, err := project()
	if err != nil {
		return err
	}
	return nodus.RunProject(ctx, root, m, buildOptions(cmd), cmd.Args().Slice(), writer(cmd))
}
func testAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	root, _, err := project()
	if err != nil {
		return err
	}
	return nodus.TestProject(ctx, root, buildOptions(cmd), writer(cmd))
}

func inspectAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 1); err != nil {
		return err
	}
	source, ref, err := splitSourceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	if cmd.String("ref") != "" {
		if ref != "" {
			return fmt.Errorf("use one reference form")
		}
		ref = cmd.String("ref")
	}
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		absolute, absErr := filepath.Abs(source)
		if absErr != nil {
			return absErr
		}
		source = absolute
	}
	options, err := keyValues(cmd.StringSlice("cmake-option"))
	if err != nil {
		return err
	}
	activity := startActivityProgress(writer(cmd), "Inspecting CMake package")
	result, err := nodus.Inspect(ctx, source, ref, options)
	activity.Stop(err == nil)
	if err != nil {
		return err
	}
	renderInspection(writer(cmd), result)
	return nil
}

func cleanAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	root, _, err := project()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(root, ".nodus")); err != nil {
		return err
	}
	fmt.Fprintln(writer(cmd), "removed .nodus artifacts")
	return nil
}
func aboutAction(_ context.Context, cmd *cli.Command) error {
	about(mustProjectRoot(), writer(cmd))
	return nil
}
func versionAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgs(cmd, 0); err != nil {
		return err
	}
	fmt.Fprintln(writer(cmd), nodus.Version)
	return nil
}

func profileFlags() []cli.Flag {
	return []cli.Flag{&cli.BoolFlag{Name: "release", Usage: "use the Release CMake profile"}}
}
func buildOptions(cmd *cli.Command) nodus.BuildOptions {
	return nodus.BuildOptions{Release: cmd.Bool("release")}
}
func projectRoot() (string, error) { return os.Getwd() }
func mustProjectRoot() string {
	root, err := projectRoot()
	if err != nil {
		return "unknown"
	}
	return root
}
func project() (string, nodus.Manifest, error) {
	root, err := projectRoot()
	if err != nil {
		return "", nodus.Manifest{}, err
	}
	m, err := nodus.LoadManifest(root)
	return root, m, err
}
func writer(cmd *cli.Command) io.Writer {
	if cmd.Writer != nil {
		return cmd.Writer
	}
	if root := cmd.Root(); root != nil && root.Writer != nil {
		return root.Writer
	}
	return os.Stdout
}
func requireArgs(cmd *cli.Command, count int) error {
	if cmd.NArg() != count {
		return fmt.Errorf("expected %d argument(s); see nodus %s --help", count, cmd.Name)
	}
	return nil
}
func requireAtLeastOne(cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return fmt.Errorf("expected at least one argument; see nodus %s --help", cmd.Name)
	}
	return nil
}

func splitSourceRef(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", fmt.Errorf("dependency source must not be empty")
	}
	if index := strings.LastIndex(value, "#"); index >= 0 {
		if index == len(value)-1 {
			return "", "", fmt.Errorf("source has an empty reference")
		}
		return value[:index], value[index+1:], nil
	}
	slash := strings.LastIndex(value, "/")
	if index := strings.LastIndex(value, "@"); index > slash {
		if index == len(value)-1 {
			return "", "", fmt.Errorf("source has an empty reference")
		}
		return value[:index], value[index+1:], nil
	}
	return value, "", nil
}
func aliasFromSource(source string) string {
	value := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(source, "/"), ".git"), ".zip")
	value = strings.TrimSuffix(value, ".tar.gz")
	value = filepath.Base(value)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "package"
	}
	return b.String()
}
func keyValues(values []string) (map[string]string, error) {
	result := map[string]string{}
	for _, value := range values {
		key, value, ok := strings.Cut(value, "=")
		if !ok || !validKey(key) || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%q must use KEY=VALUE", value)
		}
		result[key] = value
	}
	return result, nil
}
func validKey(key string) bool {
	for index, r := range key {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || index > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return key != ""
}

func renderInspection(out io.Writer, inspection nodus.Inspection) {
	fmt.Fprintf(out, "Repository       %s\nLanguage         %s\n\nBuild system     %s\n", inspection.Repository, inspection.Language, inspection.BuildSystem)
	if len(inspection.Targets) == 0 {
		fmt.Fprintln(out, "CMake targets    not detected")
	} else {
		fmt.Fprintf(out, "CMake targets    %s\n", strings.Join(inspection.Targets, ", "))
	}
	fmt.Fprintf(out, "Header-only      %s\n\nNodus compatible ✓ (CPM.cmake backend)\n", yesNo(inspection.HeaderOnly))
}
func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

var _ = sort.Strings
