package nodus

import (
	"context"
	"io"
)

// Backend is the stable seam between Nodus' portable project model and a build
// system. The alpha ships only CMake; future backends can implement this
// contract without changing manifests or CLI intent.
type Backend interface {
	Name() string
	Ensure(root string, manifest Manifest) error
	RefreshLock(ctx context.Context, root string, out io.Writer) error
	Install(ctx context.Context, root string, out io.Writer) error
}
