```
        _    _ _
   ___ | | _(_) |__   ___ _ __
  / _ \| |/ / | '_ \ / _ \ '_ \
 |  __/|   <| | |_) |  __/ | | |
  \___||_|\_\_|_.__/ \___|_| |_|
```

# 駅弁 ekiben

**A railway-station bento for your Docker** — every Compose project is a compartment, each container a tasty morsel.

`ekiben` is a fast, mouse-friendly **terminal UI** to monitor and manage Docker on Linux servers (Debian / Ubuntu first). Instead of a flat table, your containers are served as **cards grouped by Compose project** in a fluid, responsive grid — with dedicated tabs for Images, Volumes, Networks and Info.

Ships as a **single static binary**: drop it on a server and run. No runtime dependencies.

---

## ✨ Features

- 🍱 **Bento-grid dashboard** — containers as cards, grouped per Compose project. Each group header shows aggregate CPU / MEM / NET / PIDs; card width adapts per group so names stay readable; collapse/expand groups (remembered across runs).
- 📈 **Live metrics** — CPU & RAM **sparklines**, memory %, network I/O, host ports, exposed ports, health and uptime. Pick exactly which fields appear on a card.
- 🗂 **Top-level tabs** — `Containers · Images · Volumes · Networks · Info`, switchable by keyboard (`tab` / `shift+tab` / `1`–`5`) or mouse click.
- 🔎 **Detail view** — full status + CPU/RAM history, image, networks and mounted volumes, plus the container's **logs in a scrollable pane** with follow, search and word-wrap.
- 🔗 **Dependency insight** — select any image / volume / network to see exactly **which containers use it**, and a clear **STATUS** column: `safe delete`, `in use (N)` or `locked`.
- ⚠️ **Tiered, safe deletes** — a single confirm for safe removals, a **block** with the blocking containers when something is in use, and a **double confirm** with a red centered popup for dangerous actions (e.g. deleting a running container).
- 🖱 **Full mouse support** — click to select a container/row, click tabs and group headers, wheel to scroll.
- ⚡ **Actions** — start, stop, restart, pause/unpause, inspect, delete — with destructive ones gated by the confirmation rules above. (No `exec`/shell by design.)
- 🎨 **Themes & config** — dark / light / mono, refresh interval, sort order, all saved to a simple YAML file.

---

## 🚀 Install

**Debian / Ubuntu (.deb)** — grab the latest from [Releases](https://github.com/KewinGit/ekiben/releases):
```bash
sudo apt install ./ekiben_<version>_amd64.deb
```

**RHEL / Fedora (.rpm)**
```bash
sudo rpm -i ekiben_<version>_amd64.rpm
```

**Go**
```bash
go install github.com/KewinGit/ekiben/cmd/ekiben@latest
```

**From source**
```bash
make build      # -> ./ekiben  (static binary, copy anywhere)
```

> A full APT repository (`apt update && apt install ekiben`) is planned.

---

## ⌨️ Usage

```bash
ekiben                      # uses ~/.config/ekiben/config.yml (created on first run)
ekiben --config ./my.yml
ekiben --version
```

### Keys

| Context | Key | Action |
|---|---|---|
| Global | `tab` / `shift+tab` / `1`–`5` | switch tab |
| Global | `c` · `q` | settings · quit |
| Containers | `↑ ↓ ← →` / click | navigate / select |
| Containers | `enter` or `l` | open detail (with logs) |
| Containers | `s` `r` `p` `a` `u` `d` | stop · restart · pause · start · unpause · delete |
| Containers | `i` · `space` | inspect · collapse/expand group |
| Img/Vol/Net | `↑ ↓` / click | select · `d` delete |
| Detail | `↑↓ PgUp/PgDn g/G` / wheel | scroll logs |
| Detail | `f` · `/` · `esc` | follow · search · back |

---

## ⚙️ Configuration

`~/.config/ekiben/config.yml` (created with sensible defaults on first run):

| Setting | Meaning |
|---|---|
| `refresh_interval` | stats poll rate (`1s` / `2s` / `5s`) |
| `confirm_destructive` | ask before stop/restart/delete |
| `sort_within_group` | `name` / `cpu` / `mem` / `status` |
| `show_stopped` | include stopped containers |
| `theme` | `dark` / `light` / `mono` |
| `card_fields` | which fields show on each card |
| `group_order` | compose-project order |
| `group_collapsed` | per-group collapse state |

Everything is editable live from the **Settings** screen (`c`).

---

## Built with

[Go](https://go.dev) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) · [Lipgloss](https://github.com/charmbracelet/lipgloss) · the official [Docker SDK](https://github.com/docker/docker). Docker only (Podman support may come later).

## License

[MIT](LICENSE) © Kevin Corso
