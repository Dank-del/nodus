package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Dank-del/cpm/internal/cpm"
	cli "github.com/urfave/cli/v3"
)

func newAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 1); err != nil {
		return err
	}
	root, err := filepath.Abs(cmd.Args().First())
	if err != nil {
		return err
	}
	_, err = cpm.CreateManagedProject(root, filepath.Base(root), cpm.ProjectOptions{Type: cmd.String("type"), CPPStandard: cmd.Int("cpp-standard")})
	if err != nil {
		return err
	}
	if cmd.Bool("git") {
		if err := cpm.InitializeGit(ctx, root, commandWriter(cmd)); err != nil {
			return err
		}
	}
	fmt.Fprintf(commandWriter(cmd), "created managed CPM project at %s\n", root)
	return nil
}

func sourceAddAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOneArg(cmd); err != nil {
		return err
	}
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	if err := cpm.AddSources(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(commandWriter(cmd), "updated explicit sources")
	return nil
}

func sourceRemoveAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOneArg(cmd); err != nil {
		return err
	}
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	if err := cpm.RemoveSources(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(commandWriter(cmd), "updated explicit sources")
	return nil
}

func linkAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOneArg(cmd); err != nil {
		return err
	}
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	if err := cpm.AddLinks(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(commandWriter(cmd), "updated linked targets")
	return nil
}

func unlinkAction(_ context.Context, cmd *cli.Command) error {
	if err := requireAtLeastOneArg(cmd); err != nil {
		return err
	}
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	if err := cpm.RemoveLinks(root, &m, cmd.Args().Slice()); err != nil {
		return err
	}
	fmt.Fprintln(commandWriter(cmd), "updated linked targets")
	return nil
}

func buildAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	_, err = cpm.BuildManagedProject(ctx, root, m, managedBuildOptions(cmd), commandWriter(cmd))
	return err
}

func runAction(ctx context.Context, cmd *cli.Command) error {
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	return cpm.RunManagedProject(ctx, root, m, managedBuildOptions(cmd), cmd.Args().Slice(), commandWriter(cmd))
}

func testAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, m, err := managedProject()
	if err != nil {
		return err
	}
	return cpm.TestManagedProject(ctx, root, m, managedBuildOptions(cmd), commandWriter(cmd))
}

func profileFlags() []cli.Flag {
	return []cli.Flag{&cli.BoolFlag{Name: "release", Usage: "use the Release build profile"}}
}
func managedBuildOptions(cmd *cli.Command) cpm.BuildOptions {
	return cpm.BuildOptions{Release: cmd.Bool("release")}
}

func managedProject() (string, cpm.Manifest, error) {
	root, err := projectRoot()
	if err != nil {
		return "", cpm.Manifest{}, err
	}
	m, _, err := cpm.LoadManifest(root)
	if err != nil {
		return "", cpm.Manifest{}, err
	}
	if !m.Managed() {
		return "", cpm.Manifest{}, fmt.Errorf("this command requires a project created with cpm new")
	}
	return root, m, nil
}
