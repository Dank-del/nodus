package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Dank-del/cpm/internal/cpm"
	cli "github.com/urfave/cli/v3"
)

func listAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	return list(root, commandWriter(cmd))
}
func treeAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	return tree(root, commandWriter(cmd))
}
func inspectAction(ctx context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 1); err != nil {
		return err
	}
	return inspect(ctx, cmd.Args().First(), commandWriter(cmd))
}
func cleanAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	return clean(root, commandWriter(cmd))
}
func aboutAction(_ context.Context, cmd *cli.Command) error {
	about(mustProjectRoot(), commandWriter(cmd))
	return nil
}
func versionAction(_ context.Context, cmd *cli.Command) error {
	if err := requireArgCount(cmd, 0); err != nil {
		return err
	}
	fmt.Fprintln(commandWriter(cmd), cpm.Version)
	return nil
}

func projectRoot() (string, error) { return os.Getwd() }
func mustProjectRoot() string {
	root, err := projectRoot()
	if err != nil {
		return "unknown"
	}
	return root
}
func commandWriter(cmd *cli.Command) io.Writer {
	if cmd.Writer != nil {
		return cmd.Writer
	}
	if root := cmd.Root(); root != nil && root.Writer != nil {
		return root.Writer
	}
	return os.Stdout
}
func requireArgCount(cmd *cli.Command, count int) error {
	if cmd.NArg() != count {
		return fmt.Errorf("expected %d argument(s), got %d; see cpm %s --help", count, cmd.NArg(), cmd.Name)
	}
	return nil
}

func requireAtLeastOneArg(cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return fmt.Errorf("expected at least one argument; see cpm %s --help", cmd.Name)
	}
	return nil
}
