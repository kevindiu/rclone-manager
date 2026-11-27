# rclone-manager

A lightweight, self-hosted web UI for managing `rclone` remotes, scheduled backups, version history, and file restores.

## Features

- **Cloud & Encrypted Remotes**: Set up, edit, test, and delete S3, R2, B2, Google Drive, OneDrive, Dropbox, SFTP, WebDAV, and `crypt` encrypted remotes, or import an existing `rclone.conf`.
- **Scheduled Backup & Sync Jobs**: Create `copy` and `sync` jobs with cron schedules, bandwidth limits, include/exclude filters, dry-run testing, and safety guards (abort on empty source, required marker file, max delete limit).
- **Incremental Versioning**: Archive overwritten or deleted files into dated version folders (`_versions/YYYY-MM-DD`) with configurable retention cleanup.
- **File Browser & Quick Restore**: Browse remote files (including decrypted `crypt` remotes), inspect per-file version history, download files, or restore files/folders with overwrite, skip, or newer-only conflict handling.
- **Live Progress & History**: Watch real-time transfer speed, ETA, and per-file progress, and review searchable run logs.
- **Backup & Migration**: Export or import settings and jobs in JSON or YAML format.
- **Mandatory Login**: Password-protected access out of the box.
- **Mobile-Friendly PWA**: Installable web interface for desktop and mobile browsers.

## Quick Start (Docker Compose)

Create a `docker-compose.yml` file:

```yaml
services:
  rclone-manager:
    image: ghcr.io/kevindiu/rclone-manager:latest
    container_name: rclone-manager
    restart: unless-stopped
    user: "1000:1000"
    security_opt:
      - no-new-privileges:true
    ports:
      - "8080:8080"
    environment:
      TZ: UTC
    volumes:
      - ./config:/config
      # Mount any local directories you want to back up:
      # - /path/to/local/data:/data:ro
```

Prepare the config directory and start the container:

```bash
mkdir -p ./config && chown 1000:1000 ./config
docker compose up -d
```

Open `http://localhost:8080` in your browser.

## First-Run Login

- **Interactive setup**: On your first visit, you will be prompted to create an administrator username and password (minimum 8 characters).
- **Automatic setup (optional)**: Set `RCLONE_MANAGER_ADMIN_USER` and `RCLONE_MANAGER_ADMIN_PASSWORD` on initial startup to create the account automatically when starting with an empty database.
- **Change password**: Once signed in, update your password anytime under **Settings -> Account Security**.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `TZ` | `UTC` | Timezone for schedules, logs, and dated versions |
| `PORT` | `8080` | Web UI / HTTP port |
| `RCLONE_MANAGER_DATA_DIR` | `/config` (Docker) / `./data` | Persistent storage for database, logs, and `rclone.conf` |
| `RCLONE_MANAGER_ADMIN_USER` | `""` | Optional initial admin username (applied only on empty DB) |
| `RCLONE_MANAGER_ADMIN_PASSWORD` | `""` | Optional initial admin password (min 8 chars) |
| `RCLONE_MANAGER_SESSION_TTL` | `168h` | Login session lifetime (minimum `5m`) |
| `RCLONE_MANAGER_CORS_ORIGINS` | `""` | Allowed CORS origins (comma-separated; empty = same-origin only) |
| `RCLONE_MANAGER_LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |

Transfer defaults (`transfers`, `checkers`, bandwidth limit, retries, timeouts, max concurrent jobs, and log retention) are configured directly in **Settings** inside the web UI.
