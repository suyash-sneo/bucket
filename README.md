# bucket

`bucket` is a simple, fast, in-terminal task manager.

It is built to reduce cognitive load during busy work: capture ad-hoc requests quickly into a bucket, keep moving, and return to focused execution. The app is keyboard-first and TUI-native by design.

## Features

- Keyboard-first two-pane TUI (list + details/edit)
- Very fast task capture (`a` for title-only quick add)
- SQLite local persistence
- Notes editor with autosave
- URL open, filtering, and status cycling workflows
- Draft and conflict recovery protections
- Dark/light theme support

## Install (macOS)

### Latest release

```sh
curl -fsSL https://raw.githubusercontent.com/suyash-sneo/bucket/master/scripts/install.sh | sh
```

### Specific version

```sh
curl -fsSL https://raw.githubusercontent.com/suyash-sneo/bucket/master/scripts/install.sh | BUCKET_VERSION=v0.0.1 sh
```

## Uninstall (macOS)

### Remove binary + data

```sh
curl -fsSL https://raw.githubusercontent.com/suyash-sneo/bucket/master/scripts/uninstall.sh | sh
```

### Remove binary, keep data

```sh
curl -fsSL https://raw.githubusercontent.com/suyash-sneo/bucket/master/scripts/uninstall.sh | BUCKET_KEEP_DATA=1 sh
```

## Data & Config

Bucket stores local state in `~/.config/bucket/`:

- Config: `~/.config/bucket/config.yml`
- Logs: `~/.config/bucket/log.txt` (capped; default 10MB)
- Database: `~/.config/bucket/bucket.db` (default)
- Drafts: `~/.config/bucket/drafts/`
- Migration backups: `~/.config/bucket/backups/`

## How to Use

### Core flow

1. Run `bucket`
2. Press `a`, type title, press `Enter`
3. Press `Enter` (or `l`) to edit details
4. Use `ctrl+t/u/s/d/p/e/r/b/n` to jump fields
5. Press `Esc` or `ctrl+h` to go back to list

### Main list keys

- `j` / `k` or `↓` / `↑`: move
- `Space`: cycle status
- `/`: filter by title
- `o`: open URL
- `I / U / A / C / @`: switch list view
- `q` / `ctrl+q` / `ctrl+c`: quit

### Edit keys

- `Tab` / `Shift+Tab`: next/previous field
- `ctrl+space` (or `ctrl+@`): cycle status
- `ctrl+o`: open URL
- `ctrl+k`: clear URL (when URL field is focused)

## Documentation

- Config guide: `docs/config.txt`
- Conflict/draft design: `docs/conflict-management.txt`
- Build from source guide: `docs/build-source.txt`
- Full implementation spec: `Implementation.md`
- Contributing guide: `CONTRIBUTING.md`

## Contributing

See `CONTRIBUTING.md` for contribution workflow.

Help wanted: Linux and Windows build validation, runtime testing, and issue reports from real environments.

## License

This project is licensed under the MIT License. You are free to use, modify, and fork it.

See `LICENSE`.
