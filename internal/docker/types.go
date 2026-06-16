package docker

import "time"

// Image is the projection of a Docker image that ekiben needs.
type Image struct {
	ID      string
	Repo    string
	Tag     string
	Size    int64
	Created time.Time
}

// Volume is the projection of a Docker volume that ekiben needs.
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Size       int64
}

// Network is the projection of a Docker network that ekiben needs.
type Network struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

type Status string

const (
	StatusUp         Status = "up"
	StatusExited     Status = "exited"
	StatusRestarting Status = "restarting"
	StatusPaused     Status = "paused"
	StatusCreated    Status = "created"
	StatusDead       Status = "dead"
)

type Health string

const (
	HealthNone      Health = "none" // no healthcheck defined
	HealthStarting  Health = "starting"
	HealthHealthy   Health = "healthy"
	HealthUnhealthy Health = "unhealthy"
)

// Container is the projection of a Docker container that ekiben needs.
type Container struct {
	ID        string
	Name      string // full name, e.g. "hydra-dev-postgres"
	Project   string // compose project label; "" if none
	Service   string // compose service label
	Image     string
	Status    Status
	Health    Health
	ExitCode  int
	Ports     []string // host-published ports, e.g. {":5432"} or {":80", ":443"}
	Exposed   []string // exposed/internal container ports, e.g. {"80", "443"}
	CreatedAt time.Time
	Networks  []string // names of networks the container is attached to
	Mounts    []string // volume names / bind sources the container uses

	// compose project metadata (from labels), used to run `docker compose`
	ComposeWorkdir string
	ComposeFiles   []string
}

// Running reports whether the container is currently executing.
func (c Container) Running() bool { return c.Status == StatusUp || c.Status == StatusRestarting }

// Problem reports whether the card should get the red border.
func (c Container) Problem() bool {
	switch c.Status {
	case StatusExited, StatusDead, StatusRestarting:
		return true
	}
	return c.Health == HealthUnhealthy
}

// InspectInfo holds the extra per-container details ekiben surfaces in the
// detail view (fetched on demand via inspect).
type InspectInfo struct {
	RestartCount  int
	OOMKilled     bool
	Privileged    bool
	RestartPolicy string
	HealthReason  string // last healthcheck output, if any
}

// SystemInfo is a subset of `docker info` shown in the Info tab.
type SystemInfo struct {
	Name              string
	ServerVersion     string
	OperatingSystem   string
	Architecture      string
	KernelVersion     string
	StorageDriver     string
	NCPU              int
	MemTotal          int64
	ContainersRunning int
	ContainersStopped int
	Images            int
}

// DiskUsageInfo summarizes `docker system df`: total and reclaimable bytes per
// category (reclaimable ≈ what `docker system prune` would free).
type DiskUsageInfo struct {
	ImagesSize, ImagesReclaim         int64
	ContainersSize, ContainersReclaim int64
	VolumesSize, VolumesReclaim       int64
	BuildCacheSize, BuildCacheReclaim int64
}

func (d DiskUsageInfo) TotalSize() int64 {
	return d.ImagesSize + d.ContainersSize + d.VolumesSize + d.BuildCacheSize
}

func (d DiskUsageInfo) TotalReclaim() int64 {
	return d.ImagesReclaim + d.ContainersReclaim + d.VolumesReclaim + d.BuildCacheReclaim
}

// Stats is a single sample of live container metrics.
type Stats struct {
	ID       string
	CPUPerc  float64
	MemUsage uint64
	MemLimit uint64
	NetRx    uint64
	NetTx    uint64
	PIDs     uint64
	BlkRead  uint64
	BlkWrite uint64
}

// EventKind enumerates the container lifecycle changes we react to.
type EventKind string

const (
	EventStart        EventKind = "start"
	EventStop         EventKind = "stop"
	EventDie          EventKind = "die"
	EventPause        EventKind = "pause"
	EventUnpause      EventKind = "unpause"
	EventHealthStatus EventKind = "health_status"
)

type Event struct {
	ContainerID string
	Kind        EventKind
}
