# buckets

`buckets` is a fast, keyboard-only TUI task manager for engineers who need to capture and track dozens of small requests during the day.

## Features

- SQLite persistence (local)
- Two-pane TUI (list + details/edit)
- Fast task creation (title-only)
- Fuzzy filtering
- URL open + notes editor with autosave
- Clean UI (no box borders), dark/light theme support

## Install

### macOS / Linux

```sh
./scripts/install.sh
```

### Windows (PowerShell)

```powershell
.\scripts\install.ps1
```

## Uninstall

### macOS / Linux

```sh
./scripts/uninstall.sh
```

### Windows (PowerShell)

```powershell
.\scripts\uninstall.ps1
```

## Data & Config

`buckets` stores everything in:

- Config: `~/.config/bucket/config.yml`
- Logs: `~/.config/bucket/log.txt` (capped to 10MB)
- Database: `~/.config/bucket/bucket.db` (default; configurable)

## Keybindings (Main)

- `j` / `k` or `↓` / `↑`: move
- `Enter` / `→` / `l`: edit selected task
- `Esc` / `←` / `h`: exit edit mode
- `a`: add task (title-only)
- `Space`: cycle status
- `o`: open URL
- `/`: filter
- `q` / `ctrl+q` / `ctrl+c`: quit

## Keybindings (Edit)

- `ctrl+t/u/s/d/p/e/r/b/n`: jump to a field / action
- `Tab` / `Shift+Tab`: next/previous field
- `ctrl+space` (or `ctrl+@`): cycle status
- `ctrl+o`: open URL
- Notes editor: `ctrl+e` opens `$EDITOR`

## Build from source

```sh
go test ./...
go build ./cmd/bucket
```
