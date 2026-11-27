# rclone-manager

A lightweight, self-hosted, provider-agnostic backup and cloud storage manager built with a Go backend (REST API + SSE + pure-Go SQLite) and an embedded React + Vite + Tailwind PWA frontend.

## Key Features

- **Universal Remotes Setup & Quota Dashboard**: Create, edit, test, and delete any rclone backend (`s3`, Cloudflare R2, Backblaze B2, Google Drive, OneDrive, Dropbox, SFTP, WebDAV, `crypt` overlays, etc.) via `rclone config providers` + `rclone config create/update/delete` with automatic secret obfuscation and UI masking. Includes a Headless OAuth assistant (`rclone authorize` + token paste + optional custom `client_id`/`client_secret`) and `rclone.conf` import.
- **Highly Configurable Jobs**: Create, edit, clone, enable/disable, and schedule `copy` or `sync` jobs (local-to-remote, remote-to-local, or remote-to-remote) with per-job overrides, custom flag tokenization (no shell execution), `--max-transfer` + `--cutoff-mode=soft`, cron preview, and pre-flight safety guards (`Abort if source empty`, required marker file, `--max-delete`).
- **Incremental Versioning & Delete Protection**: Dated `--backup-dir` support moves deleted/overwritten files into per-day version folders (with automatic destination self-exclusion and configurable retention pruning).
- **Cloud File Browser & Quick Restore**: Browse any remote (including decrypted `crypt` remotes and `_versions/` folders), download files directly, or restore files/folders back to their original path.
- **Real-time Progress & History**: Live SSE transfer dashboard (speed, ETA, progress bar, per-file progress) plus SQLite-backed run history and searchable per-run logs.
- **Backup & Migration**: Export/import global settings and job definitions in JSON or YAML.
- **Mandatory Authentication**: Built-in local login with PBKDF2-SHA256 password hashing, SQLite-backed `HttpOnly` session cookies, brute-force rate limiting, and interactive first-run setup (or optional headless env bootstrap).
- **PWA Support**: Mobile-first responsive UI with Web App Manifest and service worker for iOS/Android Home Screen installation.

## Quick Start (Docker)

```bash
# Ensure ./config is writable by container user (uid=1000, gid=1000)
mkdir -p ./config && chown 1000:1000 ./config
docker compose up -d --build
```

Open `http://localhost:8080`.

### Prebuilt Image (GHCR)

GitHub Actions automatically builds and pushes multi-architecture (`linux/amd64` + `linux/arm64`) images to `ghcr.io/kevindiu/rclone-manager` on pushes to `main`, `v*` tags, and manual `workflow_dispatch` runs (after `frontend` and `backend` CI checks pass):

```bash
docker pull ghcr.io/kevindiu/rclone-manager:main
# Or pin a release tag / commit SHA:
# docker pull ghcr.io/kevindiu/rclone-manager:v1.0.0
# docker pull ghcr.io/kevindiu/rclone-manager:sha-<short_sha>
```

> **Note**: For the very first push to a new GHCR package, set package visibility to **Public** under GitHub **Package Settings** if you want unauthenticated `docker pull` access.

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

### Core Runtime Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `TZ` | `UTC` | Container clock, logs, and default scheduler/versioning timezone |
| `RCLONE_MANAGER_HOST` | `0.0.0.0` | HTTP bind address |
| `RCLONE_MANAGER_DATA_DIR` | `./data` (`/config` in Docker) | Persistent data directory (`DB_PATH`, `RCLONE_CONFIG`, and `LOG_DIR` derive from this automatically unless overridden) |
| `RCLONE_MANAGER_DB_PATH` | `<DATA_DIR>/rclone-manager.db` | Optional override for SQLite database path |
| `RCLONE_CONFIG` | `<DATA_DIR>/rclone.conf` | Optional override for managed rclone configuration file |
| `RCLONE_MANAGER_LOG_DIR` | `<DATA_DIR>/logs` | Optional override for per-run JSON log directory |
| `RCLONE_BIN` | `rclone` | Path to rclone binary |
| `RCLONE_MANAGER_LOG_LEVEL` | `info` | Structured log level (`debug`, `info`, `warn`, `error`) |
| `RCLONE_MANAGER_LOG_FORMAT` | `text` | Structured log format (`text` or `json`) |
| `RCLONE_MANAGER_ADMIN_USER` | `""` | Optional initial admin username (seeds only when DB has 0 users) |
| `RCLONE_MANAGER_ADMIN_PASSWORD` | `""` | Optional initial admin password (min 8 chars; requires `RCLONE_MANAGER_ADMIN_USER`) |
| `RCLONE_MANAGER_SESSION_TTL` | `168h` | Login session lifetime (minimum `5m`) |
| `RCLONE_MANAGER_CORS_ORIGINS` | `""` | Comma-separated allowed CORS origins (empty = same-origin only) |

### Optional First-Run Settings Seeds

These variables only seed initial defaults when initializing an empty database; subsequent edits are persisted and managed in **Settings** in the Web UI.

| Variable | Default | Description |
|---|---|---|
| `RCLONE_DEFAULT_TRANSFERS` | `4` | Initial default `--transfers` |
| `RCLONE_DEFAULT_CHECKERS` | `8` | Initial default `--checkers` |
| `RCLONE_DEFAULT_BWLIMIT` | `""` | Initial default `--bwlimit` schedule |
| `RCLONE_DEFAULT_RETRIES` | `3` | Initial default `--retries` |
| `RCLONE_DEFAULT_LOW_LEVEL_RETRIES` | `10` | Initial default `--low-level-retries` |
| `RCLONE_DEFAULT_TIMEOUT` | `5m` | Initial default `--timeout` |
| `RCLONE_DEFAULT_CONTIMEOUT` | `1m` | Initial default `--contimeout` |
| `RCLONE_LOG_RETENTION_DAYS` | `30` | Initial days to keep run history & logs (`0` = forever) |
| `RCLONE_MAX_CONCURRENT_JOBS` | `2` | Initial maximum parallel job executions |

## Authentication

Login is **mandatory** for all UI actions and `/api/*` endpoints (except `/api/health` and `/api/auth/{status,setup,login,logout}`).

- **First-run interactive setup**: On a fresh database, opening the web UI prompts you to create the administrator account.
- **Headless setup**: Set both `RCLONE_MANAGER_ADMIN_USER` and `RCLONE_MANAGER_ADMIN_PASSWORD` (at least 8 characters) on initial boot to create the admin account automatically when the database has no users yet.
- **Password management**: Signed-in users can change their password in **Settings -> Account Security**, which also revokes all other active sessions.
