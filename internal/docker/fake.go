package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

// Fake is an in-memory Client for tests.
type Fake struct {
	mu         sync.Mutex
	containers []Container
	StatsByID  map[string]Stats
	events     chan Event
	errs       chan error
	// ActionErr, when non-nil, is returned by all action methods (Stop/Start/etc.).
	ActionErr error
}

func NewFake(cs []Container) *Fake {
	return &Fake{
		containers: cs,
		StatsByID:  map[string]Stats{},
		events:     make(chan Event, 16),
		errs:       make(chan error, 1),
	}
}

func (f *Fake) List(_ context.Context, _ bool) ([]Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Container, len(f.containers))
	copy(out, f.containers)
	return out, nil
}

func (f *Fake) Stats(_ context.Context, id string) (Stats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.StatsByID[id], nil
}

func (f *Fake) Events(_ context.Context) (<-chan Event, <-chan error) { return f.events, f.errs }

func (f *Fake) Logs(_ context.Context, _ string, _ bool, _ int) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("line one\nline two\n")), nil
}

func (f *Fake) Inspect(_ context.Context, _ string) (string, error) { return "{}", nil }

func (f *Fake) setStatus(id string, s Status) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.containers {
		if f.containers[i].ID == id {
			f.containers[i].Status = s
			return nil
		}
	}
	return errors.New("not found")
}

func (f *Fake) actionErr() error {
	if f.ActionErr != nil {
		return f.ActionErr
	}
	return nil
}

func (f *Fake) Start(_ context.Context, id string) error {
	if err := f.actionErr(); err != nil {
		return err
	}
	return f.setStatus(id, StatusUp)
}
func (f *Fake) Stop(_ context.Context, id string) error {
	if err := f.actionErr(); err != nil {
		return err
	}
	return f.setStatus(id, StatusExited)
}
func (f *Fake) Restart(_ context.Context, id string) error {
	if err := f.actionErr(); err != nil {
		return err
	}
	return f.setStatus(id, StatusUp)
}
func (f *Fake) Pause(_ context.Context, id string) error {
	if err := f.actionErr(); err != nil {
		return err
	}
	return f.setStatus(id, StatusPaused)
}
func (f *Fake) Unpause(_ context.Context, id string) error {
	if err := f.actionErr(); err != nil {
		return err
	}
	return f.setStatus(id, StatusUp)
}
func (f *Fake) Remove(_ context.Context, id string) error {
	if err := f.actionErr(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.containers[:0]
	for _, c := range f.containers {
		if c.ID != id {
			out = append(out, c)
		}
	}
	f.containers = out
	return nil
}
func (f *Fake) Close() error { return nil }
