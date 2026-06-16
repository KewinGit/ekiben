# ekiben

A terminal UI to monitor and manage Docker containers, shown as cards grouped by
Compose project in a fluid responsive grid.

## Install

```bash
make build          # produces ./ekiben (static binary)
make install        # copies to ~/.local/bin
```

Or copy the built `ekiben` binary to any Linux server — no runtime deps.

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
