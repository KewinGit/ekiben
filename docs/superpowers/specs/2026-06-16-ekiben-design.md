# ekiben — design

> Status: design approved in brainstorming, pending implementation plan.
> Date: 2026-06-16

## What it is

`ekiben` is a terminal UI (TUI) to **monitor and perform basic management** of
Docker containers on a Linux server (primary target: Debian / Ubuntu server).

Its differentiator vs `ctop` (read-only table) and `lazydocker` (multi-panel
cockpit) is the **layout**: containers are shown as **cards grouped by Docker
Compose project**, in a **fluid responsive grid** (no fixed column count).
Think of a bento/ekiben box: compartments (compose projects) each filled with
tiles (containers).

Distributed as a **single static Go binary** — copy one file to a server and run.

All UI text is in **English**.

## Scope

In scope (v1):

- Live monitoring: per-container status, health, CPU, memory, network I/O, ports.
- Grouping by compose project, with configurable group order and collapse.
- Fluid responsive card grid.
- Basic actions: start, stop, restart, pause/unpause, inspect, delete.
- Focus view (per container) with larger graphs + log tail.
- Logs view with full scrollback, follow toggle, search.
- Settings screen (groups order, card fields, general options), persisted to YAML.

Out of scope (v1, may come later):

- `shell` / `exec` into containers — **explicitly removed**.
- Podman / other runtimes (Docker only for now; client is behind an interface).
- Image / volume / network / compose-stack management.
- Per-group card field config (global only for v1).
- Multi-host.

## Architecture

Bubble Tea (Elm architecture: Model / Update / View). Go modules.

```
cmd/ekiben/main.go          entry point; flags (--config, --version), bootstrap
internal/docker/            Docker client wrapper behind an interface (mockable)
  client.go                   ContainerList, events stream, stats poll, actions
  types.go                    domain structs (Container, Stats, Group, Health…)
internal/config/            load/save ~/.config/ekiben/config.yml + defaults
internal/model/             app state (containers, groups, selection, viewMode)
  ringbuffer.go               per-container sparkline history (~30 samples)
internal/ui/
  app.go                      root Bubble Tea Model/Update/View, view routing
  layout.go                   responsive column engine (min card width = 20)
  card.go                     card renderer + states (normal/selected/problem)
  sparkline.go                braille/block sparkline
  grid.go                     grouped-card grid view
  focus.go                    single-container focus view
  logs.go                     scrollable logs view
  settings.go                 settings screen (Groups / Card fields / General)
  theme.go                    color themes (dark / light / mono)
```

### Key boundaries

- **`internal/docker` is the only package that talks to Docker.** It exposes a
  `DockerClient` interface so the UI/model layers are tested with a mock.
- **`internal/ui/layout.go` is pure** (width in → column count / card geometry
  out), trivially unit-testable.
- **`internal/model`** holds no rendering logic; **`internal/ui`** holds no
  Docker logic.

## Data flow

1. **Startup**: load config → connect to Docker → `ContainerList(all=true)` →
   group containers by the `com.docker.compose.project` label (containers
   without it go to a synthetic `standalone` group) → order groups per
   `group_order` (unknown groups appended alphabetically).
2. **Stats poller** (goroutine): every `refresh_interval` (1s / 2s / 5s) fetch
   stats for running containers concurrently, compute CPU %, append to each
   container's ring buffer (~30 samples → sparkline), emit a `statsMsg`
   (`tea.Msg`).
3. **Events stream** (goroutine): subscribe to Docker events; on
   start/stop/die/health_status changes emit a `containerEventMsg` so the grid
   updates without waiting for the next poll.
4. **Update**: handles
   - key messages: arrow navigation across cards, action keys, view switches,
     group collapse;
   - `statsMsg` / `containerEventMsg`: update model state;
   - `tea.WindowSizeMsg`: recompute grid columns and card geometry.
5. **View**: render based on `viewMode` (`grid` | `focus` | `logs` | `settings`).

## Views

### Grid (default)

- Top header bar: totals (count, healthy / no-check / down, Σ CPU, Σ mem) + hints.
- One block per compose group: header (name, counts, Σ CPU/mem) + fluid card grid.
- Groups are expandable/collapsible (`space`), **all open by default**.
- Navigation: arrow keys across cards (wrapping across rows/groups).
- Selection: **double cyan border `╔═╗` + `►` marker** in the title — no filled
  background. No blink.
- Problem containers: **red border only** (sober) + status word; metrics stay
  normal-colored; unhealthy dot is yellow. Problem cards **stay in place**
  (no reordering).

### Card

- Minimum width **20 chars**; fluid grid, **no maximum column count**
  (`columns = max(1, (width + gap) / (minCardWidth + gap))`).
- Rows, all left-labeled and column-aligned:
  - `status · health` (e.g. `up · healthy`, `up · no-check`, `exited (137)`,
    `restarting`)
  - `cpu  <sparkline>  <pct>`
  - `mem  <bar>  <value>`
  - `net  ↓<rx> ↑<tx>`
  - `port <host ports>`
- Within a group the **compose project prefix is stripped** from the displayed
  name (the group header already provides context).
- **Fields are configurable globally** (toggle which rows appear). Optional
  extra fields: uptime, image, restarts, pids, disk io.

### Focus (`enter` on a card)

- Larger CPU/mem graphs over time, net, mapped ports, image, uptime, restarts.
- Log tail at the bottom (live).
- Keys: `esc` back, `l` full logs, `s` stop, `r` restart, `f` follow.

### Logs (`l`)

- **Full scrollback**: `↑↓` line, `PgUp/PgDn` page, `g/G` top/bottom.
- `f` toggles follow (tail). `/` search. `esc` back.

### Settings (`c`)

- **Groups tab**: reorderable list of compose projects (`J`/`K` to move); order
  is used by the grid. Per-group "collapsed by default" toggle.
- **Card fields tab**: toggle the card rows (global).
- **General tab**: `refresh_interval` (1/2/5s), `confirm_destructive` (on/off),
  `sort_within_group` (name | cpu | mem | status), `show_stopped` (on/off),
  `theme` (dark | light | mono).
- All changes saved to `~/.config/ekiben/config.yml`; UI redraws immediately.

## Actions

`start` / `stop` / `restart` / `pause` (unpause) / `inspect` / `delete`.
Available as quick actions on the selected card and inside the focus view.

**Destructive actions** (stop / restart / pause / delete) show a `y/N` confirm
prompt when `confirm_destructive` is on (default on).

**No shell / exec** — deliberately excluded.

## Config file

`~/.config/ekiben/config.yml`:

```yaml
refresh_interval: 2s          # 1s | 2s | 5s
confirm_destructive: true
sort_within_group: name       # name | cpu | mem | status
show_stopped: true
theme: dark                   # dark | light | mono
card_fields:                  # order = display order
  - status
  - health
  - cpu
  - mem
  - net
  - port
group_order:
  - hydra-dev-full
  - arya_server
  - timbr_it_server
group_collapsed:              # default-collapsed per group
  arya_server: false
```

Missing file → written with defaults on first run. Unknown groups not present in
`group_order` are appended alphabetically.

## Error handling

- **Docker unreachable** (socket missing / permission denied): full-screen
  message explaining the cause (e.g. add user to `docker` group) + automatic
  retry with backoff; the app does not crash.
- **Action failure**: error shown in a status line / toast, container state
  unchanged.
- **Stats stream error** for a single container: that container's metrics render
  as `unknown` (dim `—`) without taking down the poller.
- **Config parse error**: fall back to defaults, show a non-fatal warning.

## Testing

- **Pure unit tests**: layout engine (column count for widths), CPU% computation,
  sparkline ring buffer, group ordering / sorting, config load/save round-trip,
  compose-project grouping (including `standalone`).
- **Model/Update tests** with a **mock `DockerClient`**: navigation, action
  dispatch, confirm flow, event-driven state updates.
- **View golden tests** with `teatest` for grid / focus / logs / settings.

## Open implementation notes (non-blocking)

- Go module path depends on the publish target (e.g.
  `github.com/<owner>/ekiben`) — set when the repo is created.
- Sparkline rendering: Unicode block elements `▁▂▃▄▅▆▇█` (broad terminal
  support); braille is an alternative if more resolution is wanted.
- CPU% uses the standard delta formula (cpu delta / system delta × online CPUs).
```
