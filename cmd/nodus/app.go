package main

import (
	"context"
	"io"

	"github.com/Dank-del/nodus/internal/nodus"
	cli "github.com/urfave/cli/v3"
)

func newApp(out, errOut io.Writer) *cli.Command {
	return &cli.Command{
		Name: "nodus", Usage: "native project companion with CPM.cmake-backed dependencies",
		Description: "Nodus connects native projects to their build backends. The alpha ships a CMake backend powered by CPM.cmake.",
		Version:     nodus.Version, Writer: out, ErrWriter: errOut, Suggest: true,
		Action: func(_ context.Context, cmd *cli.Command) error { return cli.ShowRootCommandHelp(cmd) },
		Commands: []*cli.Command{
			{Name: "new", Usage: "create a Nodus-managed C/C++ project", ArgsUsage: "NAME", Flags: []cli.Flag{&cli.StringFlag{Name: "type", Value: "executable"}, &cli.IntFlag{Name: "cpp-standard", Value: 20}, &cli.BoolFlag{Name: "git"}}, Action: newAction},
			{Name: "init", Usage: "add Nodus to an existing CMake project", Flags: []cli.Flag{&cli.StringFlag{Name: "name"}}, Action: initAction},
			{Name: "add", Usage: "add a CPM.cmake-compatible dependency and update its lock", ArgsUsage: "SOURCE", Flags: dependencyFlags(), Action: addAction},
			{Name: "remove", Usage: "remove a dependency and update the CMake lock", ArgsUsage: "ALIAS", Action: removeAction},
			{Name: "install", Usage: "configure exactly the native CMake package lock", Action: installAction},
			{Name: "update", Usage: "refresh the native CMake package lock", Action: updateAction},
			{Name: "list", Usage: "show declared direct dependencies", Action: listAction},
			{Name: "source", Usage: "manage explicit source files in a Nodus-managed project", Commands: []*cli.Command{{Name: "add", ArgsUsage: "PATH...", Action: sourceAddAction}, {Name: "remove", ArgsUsage: "PATH...", Action: sourceRemoveAction}}},
			{Name: "link", Usage: "link CMake targets in a Nodus-managed project", ArgsUsage: "TARGET...", Action: linkAction},
			{Name: "unlink", Usage: "remove CMake target links", ArgsUsage: "TARGET...", Action: unlinkAction},
			{Name: "build", Usage: "configure and build with CMake", Flags: profileFlags(), Action: buildAction},
			{Name: "run", Usage: "build and run a Nodus-managed executable", ArgsUsage: "[-- ARGS...]", Flags: profileFlags(), Action: runAction},
			{Name: "test", Usage: "build and run CTest", Flags: profileFlags(), Action: testAction},
			{Name: "inspect", Usage: "inspect a package through the CMake backend", ArgsUsage: "SOURCE", Flags: inspectFlags(), Action: inspectAction},
			{Name: "clean", Usage: "remove local Nodus build artifacts", Action: cleanAction},
			{Name: "about", Usage: "show Nodus, backend, CMake, and compiler status", Action: aboutAction},
			{Name: "version", Usage: "print the Nodus version", Action: versionAction},
		},
	}
}

func dependencyFlags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{Name: "name", Usage: "dependency alias"}, &cli.StringFlag{Name: "ref", Usage: "Git tag, branch, or commit"}, &cli.StringSliceFlag{Name: "cmake-option", Usage: "CMake option KEY=VALUE (repeatable)"}, &cli.StringSliceFlag{Name: "cmake-arg", Usage: "advanced CPM.cmake argument KEY=VALUE (repeatable)"}}
}
func inspectFlags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{Name: "ref"}, &cli.StringSliceFlag{Name: "cmake-option", Usage: "CMake option KEY=VALUE (repeatable)"}}
}
