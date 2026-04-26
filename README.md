# KlyraDB

> Native desktop app to manage isolated **PostgreSQL, MySQL, MariaDB and Redis** instances on Linux.
> Stack: **Go + Wails v2 + vanilla HTML / pure CSS / vanilla JS** (no frontend framework).

## Features

- Create, start, stop and delete isolated database instances
- PostgreSQL 14–17, MySQL 8.0/8.4, MariaDB 10.6/10.11/11.4, Redis 7.2/7.4
- Each instance runs on its own port with its own data directory
- Auto port allocation per DB type (PG: 5432+, MySQL: 3306+, MariaDB: 3316+, Redis: 6379+)
- Robust PID tracking — validates `/proc/<pid>` + TCP port to detect ghost instances
- Connection URI one-click copy (correct format per DB type)
- **Auto-localized** UI — detects `$LANG` / `$LC_ALL` on startup
  - 30 languages shipped (en, es, pt, fr, de, it, nl, pl, ru, ar, he, zh-cn, zh-tw, ja, ko, ...)
  - RTL support (Arabic, Hebrew)

## Supported versions (April 2026)

| Engine     | Versions              |
|------------|-----------------------|
| PostgreSQL | 14, 15, 16, 17        |
| MySQL      | 8.0, 8.4              |
| MariaDB    | 10.6, 10.11, 11.4     |
| Redis      | 7.2, 7.4              |

## Install (Ubuntu Software Center)

Install from the Snap Store — no dependencies needed:

```bash
sudo snap install klyradb
```

Or download the latest binary from [Releases](../../releases) and run it directly.

## Build from source

```bash
# Build deps (one-time)
sudo apt install -y pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev

# Wails CLI (one-time)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Dev mode (live reload)
wails dev

# Production binary
wails build -tags webkit2_41
./build/bin/klyradb
```

Or push a `v*` tag and let GitHub Actions build it for you — no local deps needed.

## Release

```bash
git tag v0.2.0
git push origin v0.2.0
```

GitHub Actions compiles the binary and snap, creates a release, and publishes to the Snap Store automatically.

## Architecture

```
┌──────────────── Wails webview ────────────────┐
│  frontend/  HTML + pure CSS + vanilla JS      │
│  DB type picker → version select → create     │
│  calls window.go.main.App.*                   │
└───────────────────────┬───────────────────────┘
                        │ Wails IPC
┌───────────────────────▼───────────────────────┐
│  app.go                 bindings exposed to JS │
│  internal/engine/       common interface       │
│  internal/manager/      orchestrates all DBs   │
│  internal/pg/           PostgreSQL engine      │
│  internal/mysql/        MySQL engine           │
│  internal/mariadb/      MariaDB engine         │
│  internal/redis/        Redis engine           │
│  internal/i18n/         locale from $LANG      │
│  internal/store/        JSON persistence       │
└───────────────────────────────────────────────┘
```

### Data paths

| What              | Where                                         |
|-------------------|-----------------------------------------------|
| Instance registry | `~/.local/share/klyradb/instances.json`       |
| Instance data     | `~/.local/share/klyradb/data/<id>/`           |
| Instance logs     | `~/.local/share/klyradb/logs/<id>.log`        |
| PID files         | `~/.local/share/klyradb/pids/<id>.pid`        |
| Config files      | `~/.local/share/klyradb/conf/<id>.{cnf,conf}` |

## License

MIT
