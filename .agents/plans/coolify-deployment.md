# Plan: Deploy aimocksvr Go Backend on Coolify

## Overview

Deploy the aimocksvr Go backend + embedded React frontend as a single container on Coolify, with managed PostgreSQL and Redis provisioned by Coolify.

## Architecture

```
Coolify (Traefik) --> Go App (Fiber, port 8080)
                         |
                    +----+----+
                    |         |
               PostgreSQL   Redis
```

The frontend React SPA is embedded in the Go binary via `//go:embed all:dist/*`, so a single container serves the API and dashboard.

---

## Step 1: Create `Dockerfile` (multi-stage)

### Stage 1 — Build frontend (`node:22-alpine`)
- `npm ci` + `npm run build` in `web/`
- Output: `web/dist/`

### Stage 2 — Build Go binary (`golang:1.26-alpine`)
- Copy `web/dist/` into `internal/web/dist/` (required for `//go:embed all:dist/*`)
- `CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/server`

### Stage 3 — Final runtime (`gcr.io/distroless/static-debian12`)
- Copy `/server` binary from build stage
- Expose port `8080`
- `ENTRYPOINT ["/server"]`

The final image contains only the Go binary (~15-20MB), no Node.js, no Go toolchain.

---

## Step 2: Create `.dockerignore`

Exclude to keep the build context small:
```
.git/
node_modules/
web/node_modules/
bin/
*.log
.DS_Store
.agents/
.vscode/
Tiltfile
docker-compose.yml
.env
configs/config.yaml
```

---

## Step 3: Configure Coolify services

### PostgreSQL
- Name: `mocksvr-postgres`
- Version: 16
- Set database name, user, password via Coolify UI

### Redis
- Name: `mocksvr-redis`
- Version: 7
- Set password via Coolify UI

### Go App
- Deploy from Git repo: `https://github.com/profause/aimocksvr.git`
- Build pack: `Dockerfile`
- Port: `8080`
- Health check path: `/health`
- Health check port: `8080`
- Internal links: `mocksvr-postgres` (port 5432), `mocksvr-redis` (port 6379)

---

## Step 4: Set environment variables

Set these in Coolify's environment variable editor for the Go app:

```
MOCKSVR_APP_ENV=production
MOCKSVR_SERVER_HOST=0.0.0.0
MOCKSVR_SERVER_PORT=8080
MOCKSVR_DATABASE_URL=postgres://<user>:<pass>@mocksvr-postgres:5432/<dbname>?sslmode=disable
MOCKSVR_CACHE_REDIS_ADDR=mocksvr-redis:6379
MOCKSVR_CACHE_REDIS_PASSWORD=<redis-password>
MOCKSVR_CACHE_REDIS_DB=0
MOCKSVR_CACHE_REDIS_TTL=60s
MOCKSVR_LOG_LEVEL=info
MOCKSVR_DASHBOARD_ENABLED=true
MOCKSVR_AI_PROVIDER=
MOCKSVR_AI_BASE_URL=
MOCKSVR_AI_API_KEY=
MOCKSVR_AI_MODEL=
MOCKSVR_AUTH_ENABLED=false
```

> The app runs auto-migrations at startup (`database.Migrate` in `cmd/server/main.go:119`), so no separate migration job is needed.

---

## Step 5: SSL / Domain

In Coolify's app settings, set a custom domain (e.g. `mocksvr.example.com`). Coolify auto-provisions Let's Encrypt SSL via Traefik. No code changes needed.

---

## Step 6: Verify

1. Check Coolify deployment logs for `applying database migrations` and `http server listening`
2. Hit `https://mocksvr.example.com/health` — should return `{"status":"ok"}`
3. Verify the dashboard at the root URL
4. Check PostgreSQL connectivity via a mock endpoint

---

## Files to create/modify

| File          | Action                     |
| ------------- | -------------------------- |
| `Dockerfile`    | **Create** — multi-stage build |
| `.dockerignore` | **Create** — exclude dev files |

No changes to existing Go or frontend code are required.

---

## Key code references

- Server entrypoint: `cmd/server/main.go`
- Config loading (env vars + YAML): `internal/config/config.go`
- Health endpoint: `internal/router/router.go:44-48`
- Auto-migrations: `internal/database/migrate.go`
- Frontend embed: `internal/web/embed.go`
- Go module: `go.mod` (Go 1.26)
