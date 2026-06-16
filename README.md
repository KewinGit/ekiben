# ekiben

A terminal UI to monitor and manage Docker containers, shown as cards grouped by
Compose project in a fluid responsive grid.

## Install

### Debian / Ubuntu (.deb)

```bash
# grab the latest .deb from Releases, then:
sudo apt install ./ekiben_<version>_amd64.deb
```

### RHEL / Fedora (.rpm)

```bash
sudo rpm -i ekiben_<version>_amd64.rpm
```

### Go

```bash
go install github.com/KewinGit/ekiben/cmd/ekiben@latest
```

### From source

```bash
make build      # -> ./ekiben (static binary, copy anywhere)
```

> A full APT repository (`apt install ekiben` after `apt update`) is planned.

## Usage

```bash
ekiben                 # uses ~/.config/ekiben/config.yml (created on first run)
ekiben --config ./my.yml
ekiben --version
```

### Keys

| Key | Action |
|-----|--------|
| ↑ ↓ ← → | navigate cards |
| enter | focus view |
| l | logs (scrollable) |
| s / r / p | stop / restart / pause (confirm) |
| i | inspect |
| d | delete (confirm) |
| space | collapse/expand group |
| c | settings |
| q | quit |

## Config

`~/.config/ekiben/config.yml` — refresh interval, confirm prompts, card fields,
compose-group order, per-group collapse, theme. See defaults written on first run.
