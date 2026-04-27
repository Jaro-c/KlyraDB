# KlyraDB

> Local database manager for Linux, macOS and Windows.  
> Spin up isolated PostgreSQL, MySQL, MariaDB, Redis and MongoDB instances — no Docker, no config files, just click and run.

[![CI](https://github.com/Jaro-c/KlyraDB/actions/workflows/ci.yml/badge.svg)](https://github.com/Jaro-c/KlyraDB/actions/workflows/ci.yml)
[![Release](https://github.com/Jaro-c/KlyraDB/actions/workflows/release.yml/badge.svg)](https://github.com/Jaro-c/KlyraDB/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

---

## Install

**Linux — Snap Store**
```bash
sudo snap install klyradb
```

**Linux / macOS / Windows — Direct download**

Download the latest binary from [Releases](https://github.com/Jaro-c/KlyraDB/releases/latest):

| Platform | File |
|----------|------|
| Linux    | `klyradb-linux-amd64` |
| Windows  | `klyradb-windows-amd64.exe` or `klyradb-windows-amd64-setup.exe` |
| macOS    | `klyradb-macos-arm64.zip` |

---

## What it does

KlyraDB lets you create and manage fully isolated database instances directly on your machine — each with its own port, data directory and config. No root access required, no system-wide packages touched.

- **Create** a new instance in seconds — pick a DB type, a version, and a name
- **Start / Stop** instances individually with one click
- **Copy connection URI** to clipboard instantly
- **Auto-detects installed versions** — queries the official release channels at startup and always shows the 3 most recent major versions for each engine
- **Upgrade prompt** — notified when a newer version is available for an existing instance
- **30+ languages** — UI adapts to your system locale automatically
- **Dark and light theme**

---

## Supported databases

| Engine     | Default port | Version source |
|------------|-------------|----------------|
| PostgreSQL | 5432        | Dynamic — latest 3 majors from postgresql.org |
| MySQL      | 3306        | Dynamic — latest 3 majors from mysql.com |
| MariaDB    | 3316        | Dynamic — latest 3 majors from mariadb.org |
| Redis      | 6379        | Dynamic — latest 3 majors from redis.io |
| MongoDB    | 27017       | Dynamic — latest 3 majors from mongodb.com |

Version lists are fetched from [endoflife.date](https://endoflife.date) at startup and cached for the session. If the network is unavailable the app falls back to a built-in list.

> **Snap install:** PostgreSQL, MySQL, MariaDB and Redis engines are bundled inside the snap — no host installation required. MongoDB and any extra versions are detected from the host.

---

## Build from source

**Requirements:** Go 1.26+, Node.js, and [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone and build
git clone https://github.com/Jaro-c/KlyraDB.git
cd KlyraDB

# Linux
wails build -tags webkit2_41

# macOS
wails build

# Windows
wails build -nsis
```

Binary lands in `build/bin/`.

**Run tests:**
```bash
go test ./internal/...
```

---

## Project structure

```
internal/
  engine/    — shared types, port utils, PID checks
  manager/   — instance lifecycle, port allocation, persistence
  store/     — JSON state file (SNAP_USER_COMMON or ~/.local/share/klyradb)
  versions/  — dynamic version detection via endoflife.date API
  pg/        — PostgreSQL engine
  mysql/     — MySQL engine
  mariadb/   — MariaDB engine
  redis/     — Redis engine
  mongodb/   — MongoDB engine
  i18n/      — locale loading and detection
frontend/    — vanilla JS + CSS (no framework)
snap/        — snapcraft.yaml and desktop entry
```

---

## License

[Apache 2.0](LICENSE)
