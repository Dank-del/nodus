package main

import (
	"context"
	"io"

	"github.com/Dank-del/cpm/internal/cpm"
	cli "github.com/urfave/cli/v3"
)

func newApp(out, errOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:        "cpm",
		Usage:       "Git-native C/C++ source dependency manager",
		Description: "Resolve GitHub-hosted C/C++ source dependencies, lock them to commits, and generate CMake integration.",
		Version:     cpm.Version,
		Writer:      out,
		ErrWriter:   errOut,
		Suggest:     true,
		Action: func(_ context.Context, cmd *cli.Command) error {
			return cli.ShowRootCommandHelp(cmd)
		},
		Commands: []*cli.Command{
			{
				Name:   "init",
				Usage:  "create cpm.toml",
				Flags:  []cli.Flag{&cli.StringFlag{Name: "name", Usage: "project name (defaults to the current directory name)"}},
				Action: initAction,
			},
			{
				Name:      "add",
				Usage:     "add, resolve, validate, and lock a dependency",
				ArgsUsage: "SOURCE",
				Flags:     []cli.Flag{&cli.StringFlag{Name: "name", Usage: "dependency alias (defaults to the repository name)"}},
				Action:    addAction,
			},
			{Name: "remove", Usage: "remove a direct dependency and rebuild the lock graph", ArgsUsage: "ALIAS", Action: removeAction},
			{Name: "install", Usage: "materialize exactly cpm.lock", Action: installAction},
			{Name: "update", Usage: "re-resolve dependencies", ArgsUsage: "[ALIAS...]", Action: updateAction},
			{Name: "list", Usage: "show locked packages", Action: listAction},
			{Name: "tree", Usage: "show the locked dependency tree", Action: treeAction},
			{Name: "inspect", Usage: "resolve and inspect a remote package", ArgsUsage: "SOURCE", Action: inspectAction},
			{Name: "clean", Usage: "remove this project's .cpm artifacts", Action: cleanAction},
			{Name: "about", Usage: "show CPM, project, and toolchain status", Action: aboutAction},
			{Name: "version", Usage: "print the CPM version", Action: versionAction},
		},
	}
}
