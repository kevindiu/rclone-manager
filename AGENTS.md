# AGENTS.md

Repository guide and engineering standards for AI coding agents working on `rclone-manager`.

## Project Overview

`rclone-manager` is a lightweight, self-hosted, provider-agnostic rclone backup and cloud storage manager.

- **Backend**: Go `1.27` (`CGO_ENABLED=0`), standard library `net/http` + `log/slog`, pure-Go SQLite (`modernc.org/sqlite`), cron scheduler (`github.com/robfig/cron/v3`), YAML/JSON config export (`gopkg.in/yaml.v3`).
- **Frontend**: React `19.3`, TypeScript `~7.0`, Vite `8.4`, Tailwind CSS `4.3`, Lucide icons, Biome `2.6`, embedded into the Go binary via `web/embed.go`.
- **Container**: Multi-stage, multi-arch (`linux/amd64` + `linux/arm64`) Alpine `3.24` image running as non-root `app` (`uid=1000:gid=1000`), copying a pinned `rclone` binary (`ARG RCLONE_VERSION=1.76.0`).

---

## Repository Layout

```text
cmd/rclone-manager/main.go      # Composition root: slog, DB, rclone, runner, scheduler, service, HTTP server
internal/
  auth/                         # Password hashing (PBKDF2-SHA256), session token generation, login rate limiting
  config/                       # Environment variable loading & directory initialization
  domain/                       # Core domain structs only (GlobalSettings, Job, JobRun, TransferProgress, ExportBundle)
  db/                           # SQLite store (WAL mode, PRAGMA user_version migrations, context-aware queries)
  events/                       # Thread-safe SSE pub/sub Hub
  rclone/                       # Rclone CLI wrapper (client, config, browse, ops, flagparser, ini, stats, types)
  runner/                       # Job execution engine, dynamic concurrency gate, arg builder, version retention
  scheduler/                    # Timezone-aware cron scheduler + daily log retention trigger
  service/                      # Application orchestration layer (validation, DB, scheduler reload, SSE broadcast, log tailing)
  api/                          # Thin HTTP handlers, DTOs, middleware (security headers, CORS, panic recovery, request logging)
web/
  embed.go                      # //go:embed all:dist -> DistFS
  public/                       # PWA manifest, sw.js, SVG/PNG icons
  src/
    api/client.ts               # Typed REST client & connectSSE helper
    components/ui/              # Shared UI primitives (ErrorBoundary, Modal, StatusBadge)
    features/                   # Feature modules (dashboard, jobs, remotes, browse, history, settings)
    hooks/                      # Custom hooks (useRoute, useSSE, useToast)
    types/api.ts                # Frontend TypeScript API types
    utils/format.ts             # Formatting & restore path helpers
```

---

## Architecture & Coding Standards

Follow these invariants whenever modifying or adding features:

### 1. Go Layering & Idioms
- **Keep `internal/domain` pure**: Never place HTTP-only DTOs (`JobWithStatus`, `SystemInfo`) or rclone CLI schemas (`ProviderInfo`, `BrowseEntry`) in `internal/domain`. Put HTTP DTOs in `internal/api` / `internal/service` and rclone schemas in `internal/rclone`.
- **Thin HTTP handlers**: `internal/api` handlers should only decode requests, call `internal/service` (or `internal/rclone` for remote/browse ops), map errors to HTTP status codes, and encode JSON. Put cross-cutting orchestration (validation + DB + scheduler reload + SSE broadcast) in `internal/service`.
- **Consumer-defined interfaces**: Define narrow interfaces at the consumer (`runner`, `scheduler`, `service`) rather than coupling packages to concrete `*db.Store` or `*rclone.Client` structs.
- **Typed sentinel errors**: Use `errors.Is(...)` with sentinel errors (`db.ErrNotFound`, `rclone.ErrRemoteNotFound`, `rclone.ErrRemoteExists`, `runner.ErrJobAlreadyRunning`, `service.ErrValidation`, `service.ErrConflict`) — never match error strings with `strings.Contains`.
- **Structured logging**: Use `log/slog` everywhere; never use `fmt.Println` or `log.Printf`, and never silently discard non-trivial errors (`_ = ...`).

### 2. SQLite & Migrations (`internal/db`)
- Keep `SetMaxOpenConns(1)`, WAL mode, `busy_timeout(5000)`, and `foreign_keys(ON)`.
- Every DB method must accept `ctx context.Context` as its first argument and use `QueryContext` / `QueryRowContext` / `ExecContext` / `BeginTx`.
- Schema changes must be added as **append-only entries** to `migrations` in `internal/db/db.go` using `PRAGMA user_version` — never edit or delete earlier migrations.

### 3. Rclone & Execution Safety (`internal/rclone`, `internal/runner`)
- **Never invoke a shell (`sh -c`)**. Pass argument slices directly to `exec.CommandContext`.
- Tokenize user-supplied flags exclusively with `rclone.ParseAndValidateFlags`, which rejects managed flags (`--config`, `--use-json-log`, `--rc`, `--rc-addr`, `--log-file`, `--stats-log-level`).
- Validate remote option keys with `rclone.ValidateParamKey` (`^[a-zA-Z][a-zA-Z0-9_]*$`) before passing `k=v` to `rclone config create/update`.
- Only pass `config_refresh_token=false` for OAuth backends (`prov.IsOAuth`); never pass it to non-OAuth backends like `s3` or `crypt`. Always pass `--obscure --non-interactive` and `chmod 0600` on `rclone.conf`.
- When `VersioningEnabled` is true and `backupDirBase` resides inside `job.Destination` (including default `<dest>/_versions`), keep the auto-generated `--exclude /<rel>/**` rule so `sync` never deletes archived versions.
- For subprocess cancellation, use `exec.CommandContext` with `cmd.Cancel` (`SIGINT`) and `cmd.WaitDelay` — never spawn untracked `time.Sleep` PID-kill goroutines.

### 4. HTTP Security & Streaming (`internal/api`, `internal/service`)
- **Mandatory authentication**: All `/api/*` endpoints (except `GET /api/health` and `/api/auth/{status,setup,login,logout}`) require a valid `HttpOnly` SQLite-backed session cookie (`rclone_manager_session`) and must return `401 Unauthorized` otherwise. Never add a flag or env var to disable login.
- **CORS**: Never emit `Access-Control-Allow-Origin: *` by default; CORS headers are emitted only when `RCLONE_MANAGER_CORS_ORIGINS` matches the request `Origin`.
- Preserve baseline security headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`).
- Never return raw passwords, secret keys, tokens, or crypt passphrases to the frontend — always mask with `rclone.MaskedSecret`.
- Format `Content-Disposition` headers with `mime.FormatMediaType`.
- Clear write deadlines via `http.NewResponseController(w).SetWriteDeadline(time.Time{})` for SSE (`/api/events`) and file/log streaming downloads.
- Keep `ReadRunLogs` bounded to the log tail for JSON previews and stream full log downloads directly from disk via `http.ServeContent`.

### 5. Frontend & PWA (`web/`)
- Organize new UI features under `web/src/features/<feature>/`. Keep shared UI primitives in `web/src/components/ui/` and reusable state/routing/SSE logic in `web/src/hooks/`.
- Support both Desktop (`md+` sidebar) and Mobile (`< md` bottom nav) viewports.
- Preserve WCAG accessibility: never add `user-scalable=no` or `maximum-scale=1.0` in `web/index.html`, and keep `Modal` accessible (`role="dialog"`, `aria-modal="true"`, `Escape` & backdrop close).
- Keep `web/public/sw.js` caching only static app shell & `/assets/*` (never `/api/*` or parameterized routes).
- Keep `web/src/` passing `npm run lint` (Biome) and `npm run build` (`tsc -b && vite build`) with zero warnings/errors.

### 6. Docker & Supply Chain
- **Never use floating `:latest` tags** in `Dockerfile` or CI workflows; pin explicit versions (`ARG RCLONE_VERSION=1.76.0`, `golang:1.27-alpine`, `node:26-alpine`, `alpine:3.24`) and verify downloaded binaries against checksums in CI.
- Keep the runtime container running as non-root `USER app` (`1000:1000`).

---

## Development & Verification Commands

```bash
# Local dev (backend + Vite proxy on :5173 -> :8080)
make dev
cd web && npm run dev

# Frontend lint & build
cd web && npm run lint && npm run build

# Backend vet & full race test suite (requires rclone on PATH)
go vet ./...
go test -race -v ./...

# Full local check (lint + web build + go vet + go test -race)
make check

# Single embedded binary build
make build

# Multi-arch Docker build
make docker-build
make docker-buildx
```

### Testing Expectations
- Add fast mock-based unit tests for orchestration logic in `internal/service`, `internal/events`, and `internal/config`.
- Keep real `rclone` integration tests in `internal/rclone`, `internal/runner`, and `internal/api`.
- Always run `cd web && npm run lint && npm run build` and `go vet ./... && go test -race -v ./...` after making changes.
