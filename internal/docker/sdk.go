package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

// Compile-time assertion: *SDK must implement Client.
var _ Client = (*SDK)(nil)

// SDK wraps the official Docker client and implements the Client interface.
type SDK struct{ cli *client.Client }

// NewSDK creates an SDK client configured from the environment (DOCKER_HOST,
// DOCKER_CERT_PATH, DOCKER_TLS_VERIFY) and negotiates API version with the daemon.
func NewSDK() (*SDK, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &SDK{cli: cli}, nil
}

func (s *SDK) List(ctx context.Context, all bool) ([]Container, error) {
	sums, err := s.cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(sums))
	for _, su := range sums {
		name := ""
		if len(su.Names) > 0 {
			name = strings.TrimPrefix(su.Names[0], "/")
		}
		st, health, exit := ParseState(su.State, su.Status)
		ports := portsFromSummary(su.Ports)
		out = append(out, Container{
			ID:       su.ID,
			Name:     name,
			Project:  su.Labels["com.docker.compose.project"],
			Service:  su.Labels["com.docker.compose.service"],
			Image:    su.Image,
			Status:   st,
			Health:   health,
			ExitCode: exit,
			Ports:    ports,
		})
	}
	return out, nil
}

// portsFromSummary extracts unique host-published ports as ":<port>" strings
// from the Ports slice returned by ContainerList.
func portsFromSummary(ps []types.Port) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range ps {
		if p.PublicPort == 0 {
			continue
		}
		s := fmt.Sprintf(":%d", p.PublicPort)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func (s *SDK) Stats(ctx context.Context, id string) (Stats, error) {
	resp, err := s.cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return Stats{}, err
	}
	defer resp.Body.Close()
	var v container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return Stats{}, err
	}
	var rx, tx uint64
	for _, n := range v.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	var blkR, blkW uint64
	for _, e := range v.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(e.Op) {
		case "read":
			blkR += e.Value
		case "write":
			blkW += e.Value
		}
	}
	return Stats{
		ID: id,
		CPUPerc: CPUPercent(
			v.CPUStats.CPUUsage.TotalUsage, v.PreCPUStats.CPUUsage.TotalUsage,
			v.CPUStats.SystemUsage, v.PreCPUStats.SystemUsage,
			v.CPUStats.OnlineCPUs,
		),
		MemUsage: v.MemoryStats.Usage,
		MemLimit: v.MemoryStats.Limit,
		NetRx:    rx, NetTx: tx,
		PIDs:    v.PidsStats.Current,
		BlkRead: blkR, BlkWrite: blkW,
	}, nil
}

func (s *SDK) Events(ctx context.Context) (<-chan Event, <-chan error) {
	out := make(chan Event, 16)
	errOut := make(chan error, 1)
	msgs, errs := s.cli.Events(ctx, events.ListOptions{})
	go func() {
		defer close(out)
		for {
			select {
			case m := <-msgs:
				if m.Type != events.ContainerEventType {
					continue
				}
				out <- Event{ContainerID: m.Actor.ID, Kind: EventKind(string(m.Action))}
			case e := <-errs:
				if e != nil && e != io.EOF {
					errOut <- e
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errOut
}

func (s *SDK) Logs(ctx context.Context, id string, follow bool, tail int) (io.ReadCloser, error) {
	opts := container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: follow, Timestamps: false}
	if tail > 0 {
		opts.Tail = fmt.Sprintf("%d", tail)
	}
	return s.cli.ContainerLogs(ctx, id, opts)
}

func (s *SDK) Inspect(ctx context.Context, id string) (string, error) {
	raw, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	return string(b), err
}

func (s *SDK) Start(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}
func (s *SDK) Stop(ctx context.Context, id string) error {
	return s.cli.ContainerStop(ctx, id, container.StopOptions{})
}
func (s *SDK) Restart(ctx context.Context, id string) error {
	return s.cli.ContainerRestart(ctx, id, container.StopOptions{})
}
func (s *SDK) Pause(ctx context.Context, id string) error   { return s.cli.ContainerPause(ctx, id) }
func (s *SDK) Unpause(ctx context.Context, id string) error { return s.cli.ContainerUnpause(ctx, id) }
func (s *SDK) Remove(ctx context.Context, id string) error {
	return s.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}
func (s *SDK) Close() error { return s.cli.Close() }
