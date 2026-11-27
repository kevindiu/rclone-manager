# rclone-manager

A lightweight, self-hosted web UI for managing `rclone` remotes, scheduled backups, version history, and file restores.

## Features

- **Cloud & Encrypted Remotes**: Set up, edit, test, deduplicate, clean up, and delete S3, R2, B2, Google Drive, OneDrive, Dropbox, SFTP, WebDAV, and `crypt` encrypted remotes (including an Encrypted Remote Wizard), or import an existing `rclone.conf`.
- **Scheduled Backup & Sync Jobs**: Create `copy`, `sync`, `move`, `check`, and `bisync` jobs with cron schedules, bandwidth limits, include/exclude/age/size filters, command preview, dry-run testing, and safety guards (abort on empty source, required marker file, max delete limit).
- **Incremental Versioning**: Archive overwritten or deleted files into dated version folders (`_versions/YYYY-MM-DD`) with configurable retention cleanup.
- **File Browser & Quick Restore**: Browse remote files (including decrypted `crypt` remotes), preview images/PDFs/text inline, inspect per-file version history, upload/rename/bulk-delete, download files, or restore files/folders with overwrite, skip, or newer-only conflict handling.
- **Live Progress, Stats & Notifications**: Watch real-time transfer speed, ETA, per-file progress, and 14-day activity charts, review searchable run logs, and receive completion alerts via Webhook (Discord/Slack/JSON), Telegram Bot, or Ntfy.
- **Backup & Migration**: Export or import settings and jobs in JSON/YAML, or create passphrase-encrypted full `.rcmbak` backups including `rclone.conf` (PBKDF2-SHA256 + AES-256-GCM).
- **Mandatory Login & Session Control**: Password-protected access out of the box with active session tracking and remote session revocation.
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
- **Forgot password**: Reset it from the container CLI with `docker exec -i rclone-manager /usr/local/bin/rclone-manager reset-password` (or pass `-password <new-password>`).

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
| `RCLONE_MANAGER_TRUSTED_PROXIES` | `""` | Comma-separated reverse proxy IPs/CIDRs trusted for `X-Forwarded-For` / `X-Forwarded-Proto` |
| `RCLONE_MANAGER_LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |

Transfer defaults (`transfers`, `checkers`, bandwidth limit, retries, timeouts, max concurrent jobs, and log retention) are configured directly in **Settings** inside the web UI.

## License

This project is licensed under the [MIT License](LICENSE).
