package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	app := newApp(os.Stdout, os.Stderr)
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "nodus:", err)
		os.Exit(1)
	}
}
