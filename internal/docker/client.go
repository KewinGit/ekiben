package docker

import (
	"context"
	"io"
)

// Client is the surface ekiben needs from a container runtime.
type Client interface {
	List(ctx context.Context, all bool) ([]Container, error)
	Stats(ctx context.Context, id string) (Stats, error)
	Events(ctx context.Context) (<-chan Event, <-chan error)
	Logs(ctx context.Context, id string, follow bool, tail int) (io.ReadCloser, error)
	Inspect(ctx context.Context, id string) (string, error) // pretty JSON
	InspectInfo(ctx context.Context, id string) (InspectInfo, error)

	Images(ctx context.Context) ([]Image, error)
	Volumes(ctx context.Context) ([]Volume, error)
	Networks(ctx context.Context) ([]Network, error)
	DiskUsage(ctx context.Context) (DiskUsageInfo, error)

	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Pause(ctx context.Context, id string) error
	Unpause(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error

	RemoveImage(ctx context.Context, id string, force bool) error
	RemoveVolume(ctx context.Context, name string, force bool) error
	RemoveNetwork(ctx context.Context, id string) error

	Close() error
}
