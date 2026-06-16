package ui

import (
	"context"
	"os/exec"
	"time"

	"github.com/KewinGit/ekiben/internal/docker"
	tea "github.com/charmbracelet/bubbletea"
)

// composeRef identifies a compose project for `docker compose` invocations.
type composeRef struct {
	project string
	workdir string
	files   []string
}

// execDoneMsg is returned after an interactive exec/compose process exits.
type execDoneMsg struct{ err error }

// composeUpAfterDownMsg chains `up` after a `down` for the restart action.
type composeUpAfterDownMsg composeRef

// execShellCmd suspends the TUI and opens a shell inside the container (bash if
// available, else sh), via the docker CLI.
func (m *Model) execShellCmd(id string) tea.Cmd {
	c := exec.Command("docker", "exec", "-it", id, "sh", "-c",
		"command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDoneMsg{err} })
}

func composeArgs(g composeRef, sub ...string) []string {
	args := []string{"compose", "--project-name", g.project}
	if g.workdir != "" {
		args = append(args, "--project-directory", g.workdir)
	}
	for _, f := range g.files {
		args = append(args, "-f", f)
	}
	return append(args, sub...)
}

func (m *Model) composeUpCmd(g composeRef) tea.Cmd {
	return tea.ExecProcess(exec.Command("docker", composeArgs(g, "up", "-d")...),
		func(err error) tea.Msg { return execDoneMsg{err} })
}

func (m *Model) composeDownCmd(g composeRef) tea.Cmd {
	return tea.ExecProcess(exec.Command("docker", composeArgs(g, "down")...),
		func(err error) tea.Msg { return execDoneMsg{err} })
}

// composeRestartCmd runs `down` then chains `up -d` via composeUpAfterDownMsg.
func (m *Model) composeRestartCmd(g composeRef) tea.Cmd {
	return tea.ExecProcess(exec.Command("docker", composeArgs(g, "down")...),
		func(err error) tea.Msg { return composeUpAfterDownMsg(g) })
}

// refreshCmd lists containers once.
func (m *Model) refreshCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		cs, err := client.List(context.Background(), true)
		if err != nil {
			return errMsg{err}
		}
		return containersMsg(cs)
	}
}

// pollCmd waits one interval, then fetches stats for all running containers.
func (m *Model) pollCmd() tea.Cmd {
	client := m.client
	interval := m.cfg.Interval()
	ids := []string{}
	for _, g := range m.groups {
		for _, c := range g.Containers {
			if c.Running() {
				ids = append(ids, c.ID)
			}
		}
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		out := statsMsg{}
		for _, id := range ids {
			if s, err := client.Stats(context.Background(), id); err == nil {
				out[id] = s
			}
		}
		return out
	})
}

// waitForEvent reads the next event from the already-open event channel.
// It must NOT call m.client.Events again — the channel is opened once in Init.
func (m *Model) waitForEvent() tea.Cmd {
	ch := m.eventCh
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(ev)
	}
}

// actionResultMsg carries the result of an async container action.
type actionResultMsg struct{ err error }

// doActionCmd runs a container action asynchronously and returns actionResultMsg.
func (m *Model) doActionCmd(action, id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var err error
		ctx := context.Background()
		switch action {
		case "stop":
			err = client.Stop(ctx, id)
		case "restart":
			err = client.Restart(ctx, id)
		case "pause":
			err = client.Pause(ctx, id)
		case "unpause":
			err = client.Unpause(ctx, id)
		case "start":
			err = client.Start(ctx, id)
		case "delete":
			err = client.Remove(ctx, id)
		}
		return actionResultMsg{err}
	}
}

// removeImageCmd / removeVolumeCmd / removeNetworkCmd run a resource removal
// asynchronously and report the result via actionResultMsg.
func (m *Model) removeImageCmd(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg { return actionResultMsg{client.RemoveImage(context.Background(), id, false)} }
}

func (m *Model) removeVolumeCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg { return actionResultMsg{client.RemoveVolume(context.Background(), name, false)} }
}

func (m *Model) removeNetworkCmd(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg { return actionResultMsg{client.RemoveNetwork(context.Background(), id)} }
}

// focusLogsMsg carries the recent log output for the detail view.
type focusLogsMsg struct {
	id      string
	content string
}

// focusInspectMsg carries inspect details for the detail view.
type focusInspectMsg struct {
	id   string
	info docker.InspectInfo
}

// loadFocusInspectCmd inspects the selected container for the detail view.
func (m *Model) loadFocusInspectCmd() tea.Cmd {
	client := m.client
	id := m.SelectedID()
	return func() tea.Msg {
		info, err := client.InspectInfo(context.Background(), id)
		if err != nil {
			return focusInspectMsg{id: id}
		}
		return focusInspectMsg{id: id, info: info}
	}
}

// focusTickMsg drives the focus-view log refresh loop.
type focusTickMsg struct{}

// loadFocusLogsCmd fetches the recent log lines for the detail view.
func (m *Model) loadFocusLogsCmd() tea.Cmd {
	client := m.client
	id := m.SelectedID()
	return func() tea.Msg {
		rc, err := client.Logs(context.Background(), id, false, 2000)
		if err != nil {
			return focusLogsMsg{id: id, content: ""}
		}
		defer rc.Close()
		b, _ := readAllDemux(rc)
		return focusLogsMsg{id: id, content: string(b)}
	}
}

// focusTickCmd schedules a focusTickMsg after 2 seconds.
func (m *Model) focusTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return focusTickMsg{}
	})
}

// imagesMsg carries the result of listing images.
type imagesMsg []docker.Image

// volumesMsg carries the result of listing volumes.
type volumesMsg []docker.Volume

// networksMsg carries the result of listing networks.
type networksMsg []docker.Network

// loadImagesCmd fetches the images list.
func (m *Model) loadImagesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		imgs, err := client.Images(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return imagesMsg(imgs)
	}
}

// loadVolumesCmd fetches the volumes list.
func (m *Model) loadVolumesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		vols, err := client.Volumes(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return volumesMsg(vols)
	}
}

// loadNetworksCmd fetches the networks list.
func (m *Model) loadNetworksCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		nets, err := client.Networks(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return networksMsg(nets)
	}
}
