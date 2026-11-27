# rclone-manager

A lightweight, self-hosted, provider-agnostic backup and cloud storage manager built with a Go backend (REST API + SSE + pure-Go SQLite) and an embedded React + Vite + Tailwind PWA frontend.

## Key Features

- **Universal Remotes Setup & Quota Dashboard**: Create, edit, test, and delete any rclone backend (`s3`, Cloudflare R2, Backblaze B2, Google Drive, OneDrive, Dropbox, SFTP, WebDAV, `crypt` overlays, etc.) via `rclone config providers` + `rclone config create/update/delete` with automatic secret obfuscation and UI masking. Includes a Headless OAuth assistant (`rclone authorize` + token paste + optional custom `client_id`/`client_secret`) and `rclone.conf` import.
- **Highly Configurable Jobs**: Create, edit, clone, enable/disable, and schedule `copy` or `sync` jobs (local-to-remote, remote-to-local, or remote-to-remote) with per-job overrides, custom flag tokenization (no shell execution), `--max-transfer` + `--cutoff-mode=soft`, cron preview, and pre-flight safety guards (`Abort if source empty`, required marker file, `--max-delete`).
- **Incremental Versioning & Delete Protection**: Dated `--backup-dir` support moves deleted/overwritten files into per-day version folders (with automatic destination self-exclusion and configurable retention pruning).
- **Cloud File Browser & Quick Restore**: Browse any remote (including decrypted `crypt` remotes and `_versions/` folders), download files directly, or restore files/folders back to their original path.
- **Real-time Progress & History**: Live SSE transfer dashboard (speed, ETA, progress bar, per-file progress) plus SQLite-backed run history and searchable per-run logs.
- **Backup & Migration**: Export/import global settings and job definitions in JSON or YAML.
- **PWA Support**: Mobile-first responsive UI with Web App Manifest and service worker for iOS/Android Home Screen installation.

## Quick Start (Docker)

```bash
# Ensure ./config is writable by container user (uid=1000, gid=1000)
mkdir -p ./config && chown 1000:1000 ./config
docker compose up -d --build
```

Open `http://localhost:8080`.

### Upgrading Bundled `rclone`

The `Dockerfile` pins `ARG RCLONE_VERSION=1.76.0` (never `:latest`) and copies `/usr/local/bin/rclone` from `rclone/rclone:${RCLONE_VERSION}`. To upgrade manually:

```bash
docker build --build-arg RCLONE_VERSION=1.77.0 -t rclone-manager:latest .
```

Multi-architecture build (`linux/amd64` + `linux/arm64`):

```bash
make docker-buildx RCLONE_VERSION=1.76.0
```

## Local Development

```bash
# Terminal 1: Go backend (requires rclone on PATH)
go run ./cmd/rclone-manager

# Terminal 2: Vite frontend dev server (proxies /api -> localhost:8080)
cd web && npm install && npm run dev
```

Or build a single binary with the frontend embedded:

```bash
make build
./bin/rclone-manager
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` / `RCLONE_MANAGER_PORT` | `8080` | HTTP server port |
| `RCLONE_MANAGER_HOST` | `0.0.0.0` | HTTP bind address |
| `RCLONE_MANAGER_DATA_DIR` | `./data` (`/config` in Docker) | Persistent data directory |
| `RCLONE_MANAGER_DB_PATH` | `<DATA_DIR>/rclone-manager.db` | SQLite database path |
| `RCLONE_CONFIG` | `<DATA_DIR>/rclone.conf` | Managed rclone configuration file |
| `RCLONE_MANAGER_LOG_DIR` | `<DATA_DIR>/logs` | Per-run JSON log directory |
| `RCLONE_BIN` | `rclone` | Path to rclone binary |
| `RCLONE_DEFAULT_TRANSFERS` | `4` | Default `--transfers` |
| `RCLONE_DEFAULT_CHECKERS` | `8` | Default `--checkers` |
| `RCLONE_DEFAULT_BWLIMIT` | `""` | Default `--bwlimit` schedule |
| `RCLONE_DEFAULT_RETRIES` | `3` | Default `--retries` |
| `RCLONE_DEFAULT_LOW_LEVEL_RETRIES` | `10` | Default `--low-level-retries` |
| `RCLONE_DEFAULT_TIMEOUT` | `5m` | Default `--timeout` |
| `RCLONE_DEFAULT_CONTIMEOUT` | `1m` | Default `--contimeout` |
| `RCLONE_LOG_RETENTION_DAYS` | `30` | Days to keep run history & logs (`0` = forever) |
| `RCLONE_TIMEZONE` / `TZ` | `UTC` | Scheduler and versioning timezone |
| `RCLONE_MAX_CONCURRENT_JOBS` | `2` | Maximum parallel job executions |
| `RCLONE_MANAGER_LOG_LEVEL` | `info` | Structured log level (`debug`, `info`, `warn`, `error`) |
| `RCLONE_MANAGER_LOG_FORMAT` | `text` | Structured log format (`text` or `json`) |
| `RCLONE_MANAGER_CORS_ORIGINS` | `""` | Comma-separated allowed CORS origins (empty = same-origin only) |
