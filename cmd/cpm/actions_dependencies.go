package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dank-del/cpm/internal/cpm"
	cli "github.com/urfave/cli/v3"
)

func initAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	name := cmd.String("name")
	if name == "" {
		name = filepath.Base(root)
	}
	path := filepath.Join(root, cpm.ManifestName)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", cpm.ManifestName)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := cpm.WriteManifest(root, cpm.Manifest{Format: 2, Project: cpm.Project{Name: name, Version: "0.1.0"}, Dependencies: map[string]string{}}); err != nil {
		return err
	}
	fmt.Fprintf(commandWriter(cmd), "created %s\n", cpm.ManifestName)
	return nil
}

func addAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 1); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	source := cmd.Args().First()
	parsed, err := cpm.ParseSource(source)
	if err != nil {
		return err
	}
	alias := cmd.String("name")
	if alias == "" {
		alias = cpm.AliasFromSource(parsed)
	}
	if err := cpm.ValidateAlias(alias); err != nil {
		return err
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	options := cmd.StringSlice("cmake-option")
	if err := cpm.ValidateCMakeOptions(options); err != nil {
		return err
	}
	if m.CMakeOptions == nil {
		m.CMakeOptions = map[string][]string{}
	}
	m.Dependencies[alias] = source
	if cmd.IsSet("cmake-option") {
		m.CMakeOptions[alias] = options
	}
	return resolvePrepareWrite(ctx, root, m, commandWriter(cmd), "added "+alias)
}

func removeAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 1); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	alias := cmd.Args().First()
	if _, ok := m.Dependencies[alias]; !ok {
		return fmt.Errorf("dependency %q is not declared", alias)
	}
	delete(m.Dependencies, alias)
	delete(m.CMakeOptions, alias)
	return resolvePrepareWrite(ctx, root, m, commandWriter(cmd), "removed "+alias)
}

func updateAction(ctx context.Context, cmd *cli.Command) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return err
	}
	for _, alias := range cmd.Args().Slice() {
		if _, ok := m.Dependencies[alias]; !ok {
			return fmt.Errorf("dependency %q is not declared", alias)
		}
	}
	return resolvePrepareWrite(ctx, root, m, commandWriter(cmd), "updated lockfile")
}

func installAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
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
		return fmt.Errorf("cpm.lock does not match cpm.toml; run cpm update")
	}
	preparing := newPackageProgress(commandWriter(cmd), len(lock.Packages), "Installing packages")
	err = cpm.NewResolver().Prepare(ctx, root, &lock, preparing)
	preparing.Stop(err == nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(commandWriter(cmd), "installed %d packages\n", len(lock.Packages))
	return nil
}
