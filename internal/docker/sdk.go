package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
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
		exposed := exposedFromSummary(su.Ports)
		var nets []string
		if su.NetworkSettings != nil {
			for n := range su.NetworkSettings.Networks {
				nets = append(nets, n)
			}
			sort.Strings(nets)
		}
		var mounts []string
		for _, mp := range su.Mounts {
			switch {
			case string(mp.Type) == "volume" && mp.Name != "":
				mounts = append(mounts, mp.Name)
			case mp.Source != "":
				mounts = append(mounts, mp.Source)
			default:
				mounts = append(mounts, mp.Destination)
			}
		}
		out = append(out, Container{
			ID:             su.ID,
			Name:           name,
			Project:        su.Labels["com.docker.compose.project"],
			Service:        su.Labels["com.docker.compose.service"],
			Image:          su.Image,
			Status:         st,
			Health:         health,
			ExitCode:       exit,
			Ports:          ports,
			Exposed:        exposed,
			CreatedAt:      time.Unix(su.Created, 0),
			Networks:       nets,
			Mounts:         mounts,
			ComposeWorkdir: su.Labels["com.docker.compose.project.working_dir"],
			ComposeFiles:   splitComposeFiles(su.Labels["com.docker.compose.project.config_files"]),
		})
	}
	return out, nil
}

// portsFromSummary extracts unique host-published ports as ":<port>" strings
// from the Ports slice returned by ContainerList.
func portsFromSummary(ps []types.Port) []string {
	seen := map[uint16]bool{}
	nums := []int{}
	for _, p := range ps {
		if p.PublicPort == 0 || seen[p.PublicPort] {
			continue
		}
		seen[p.PublicPort] = true
		nums = append(nums, int(p.PublicPort))
	}
	// Docker returns ports in a non-deterministic order; sort ascending so the
	// card display is stable across refreshes.
	sort.Ints(nums)
	out := make([]string, 0, len(nums))
	for _, n := range nums {
		out = append(out, fmt.Sprintf(":%d", n))
	}
	return out
}

// splitComposeFiles splits the compose config_files label into individual paths.
func splitComposeFiles(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// exposedFromSummary returns the distinct container (private) ports as "<port>",
// sorted ascending — these are exposed regardless of host publishing.
func exposedFromSummary(ps []types.Port) []string {
	seen := map[uint16]bool{}
	nums := []int{}
	for _, p := range ps {
		if p.PrivatePort == 0 || seen[p.PrivatePort] {
			continue
		}
		seen[p.PrivatePort] = true
		nums = append(nums, int(p.PrivatePort))
	}
	sort.Ints(nums)
	out := make([]string, 0, len(nums))
	for _, n := range nums {
		out = append(out, fmt.Sprintf("%d", n))
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

func (s *SDK) InspectInfo(ctx context.Context, id string) (InspectInfo, error) {
	raw, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return InspectInfo{}, err
	}
	info := InspectInfo{RestartCount: raw.RestartCount}
	if raw.State != nil {
		info.OOMKilled = raw.State.OOMKilled
		if raw.State.Health != nil && len(raw.State.Health.Log) > 0 {
			info.HealthReason = strings.TrimSpace(raw.State.Health.Log[len(raw.State.Health.Log)-1].Output)
		}
	}
	if raw.HostConfig != nil {
		info.Privileged = raw.HostConfig.Privileged
		info.RestartPolicy = string(raw.HostConfig.RestartPolicy.Name)
	}
	return info, nil
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
func (s *SDK) Images(ctx context.Context) ([]Image, error) {
	sums, err := s.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Image, 0, len(sums))
	for _, su := range sums {
		repo, tag := "<none>", su.ID
		if len(su.ID) > 12 {
			tag = su.ID[7:19] // strip "sha256:" prefix for short ID fallback
		}
		if len(su.RepoTags) > 0 && su.RepoTags[0] != "<none>:<none>" {
			parts := strings.SplitN(su.RepoTags[0], ":", 2)
			repo = parts[0]
			if len(parts) == 2 {
				tag = parts[1]
			} else {
				tag = "latest"
			}
		}
		out = append(out, Image{
			ID:      su.ID,
			Repo:    repo,
			Tag:     tag,
			Size:    su.Size,
			Created: time.Unix(su.Created, 0),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Repo+":"+out[i].Tag < out[j].Repo+":"+out[j].Tag
	})
	return out, nil
}

func sortVolumes(out []Volume) []Volume {
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *SDK) Volumes(ctx context.Context) ([]Volume, error) {
	// Try DiskUsage first so we get UsageData.Size per volume.
	du, err := s.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err == nil && len(du.Volumes) > 0 {
		out := make([]Volume, 0, len(du.Volumes))
		for _, v := range du.Volumes {
			var sz int64
			if v.UsageData != nil {
				sz = v.UsageData.Size
			}
			out = append(out, Volume{
				Name:       v.Name,
				Driver:     v.Driver,
				Mountpoint: v.Mountpoint,
				Size:       sz,
			})
		}
		return sortVolumes(out), nil
	}
	// Fallback: VolumeList with Size 0.
	resp, err2 := s.cli.VolumeList(ctx, volume.ListOptions{})
	if err2 != nil {
		return nil, err2
	}
	out := make([]Volume, 0, len(resp.Volumes))
	for _, v := range resp.Volumes {
		out = append(out, Volume{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
		})
	}
	return sortVolumes(out), nil
}

func (s *SDK) Networks(ctx context.Context) ([]Network, error) {
	nets, err := s.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Network, 0, len(nets))
	for _, n := range nets {
		id := n.ID
		if len(id) > 12 {
			id = id[:12]
		}
		out = append(out, Network{
			ID:     id,
			Name:   n.Name,
			Driver: n.Driver,
			Scope:  n.Scope,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *SDK) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := s.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force, PruneChildren: true})
	return err
}

func (s *SDK) RemoveVolume(ctx context.Context, name string, force bool) error {
	return s.cli.VolumeRemove(ctx, name, force)
}

func (s *SDK) RemoveNetwork(ctx context.Context, id string) error {
	return s.cli.NetworkRemove(ctx, id)
}

func (s *SDK) DiskUsage(ctx context.Context) (DiskUsageInfo, error) {
	du, err := s.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return DiskUsageInfo{}, err
	}
	info := DiskUsageInfo{ImagesSize: du.LayersSize}
	for _, img := range du.Images {
		if img.Containers == 0 {
			r := img.Size - img.SharedSize
			if r < 0 {
				r = img.Size
			}
			info.ImagesReclaim += r
		}
	}
	for _, c := range du.Containers {
		info.ContainersSize += c.SizeRw
		if c.State != "running" {
			info.ContainersReclaim += c.SizeRw
		}
	}
	for _, v := range du.Volumes {
		if v.UsageData != nil {
			info.VolumesSize += v.UsageData.Size
			if v.UsageData.RefCount == 0 {
				info.VolumesReclaim += v.UsageData.Size
			}
		}
	}
	for _, bc := range du.BuildCache {
		info.BuildCacheSize += bc.Size
		if !bc.InUse {
			info.BuildCacheReclaim += bc.Size
		}
	}
	return info, nil
}

func (s *SDK) SystemInfo(ctx context.Context) (SystemInfo, error) {
	i, err := s.cli.Info(ctx)
	if err != nil {
		return SystemInfo{}, err
	}
	return SystemInfo{
		Name:              i.Name,
		ServerVersion:     i.ServerVersion,
		OperatingSystem:   i.OperatingSystem,
		Architecture:      i.Architecture,
		KernelVersion:     i.KernelVersion,
		StorageDriver:     i.Driver,
		NCPU:              i.NCPU,
		MemTotal:          i.MemTotal,
		ContainersRunning: i.ContainersRunning,
		ContainersStopped: i.ContainersStopped,
		Images:            i.Images,
	}, nil
}

func (s *SDK) Close() error { return s.cli.Close() }
