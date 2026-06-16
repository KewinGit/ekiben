package docker

import "time"

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
	Ports     []string // host-facing ports, e.g. []string{":5432"} or {":80", ":443"}
	CreatedAt time.Time
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
