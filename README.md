# Aurora Admin Service

Admin service for Aurora platform.

This repository contains:
- Go backend (`cmd/server`) with HTTP + gRPC (h2c) transport.
- React/Vite admin UI (`src`) embedded into Go binary via `go:embed`.
- Runtime config bootstrap to etcd (`/runtime`, `/shared/cors`, `/endpoint/admin`).

## Main Features

- API key login and rotation.
- Token secret management and rotation.
- Certificate store transport (gRPC).
- Module status endpoint from etcd endpoint registry.
- SPA admin UI bundled into the backend binary runtime (`internal/app/dist`).

## Tech Stack

- Go 1.26
- Gin
- etcd v3 client
- React 19 + Vite + TypeScript

## Required Dependencies

- etcd (required at startup)
- Node.js + npm (to build UI)
- Go toolchain

PostgreSQL/Redis values are still part of runtime config and are seeded/loaded via etcd keys, but service startup itself depends on etcd connectivity first.

## Configuration Model

Startup flow:

1. Load env from `.env` (or process env).
2. Connect etcd with env etcd settings.
3. Seed missing keys to etcd using CAS transaction (only if key does not exist).
4. Reload runtime config strictly from etcd.
5. Start HTTP + gRPC server.

Seeded key spaces:

- `/runtime/*`
- `/shared/cors/*`
- `/endpoint/admin` (value format: `running:<host:port>`)

Optional runtime fields (can be empty/missing):

- `redis/username`, `redis/password`, `redis/ca`, `redis/client_key`, `redis/client_cert`
- `etcd/username`, `etcd/password`, `etcd/ca`, `etcd/client_key`, `etcd/client_cert`, `etcd/server_name`
- `telegram/bot_token`, `telegram/chat_id`

## Local Development

### 1) Prepare env

```bash
cd Admin
# edit .env values
```

### 2) Install UI dependencies

```bash
npm ci
```

### 3) Build UI bundle for backend serving

```bash
npm run build
```

### 4) Run backend

```bash
go mod download
go run ./cmd/server
```

Default listen port is from `APP_PORT` (current default: `3009`).

Note: `npm run build` outputs assets to `internal/app/dist`, and backend embeds this folder at build time.

## Useful Commands

```bash
# lint frontend
npm run lint

# build frontend
npm run build

# run all Go tests
go test ./...
```

## API Endpoints

Health:

- `GET /health/liveness`
- `GET /health/readiness`
- `GET /health/startup`

Admin API (`/api/v1`):

- `POST /apikey/login`
- `POST /apikey/rotate` (requires admin api key auth)
- `GET /modules/enabled` (requires admin api key auth)
- `GET /modules/status` (requires admin api key auth)

## Docker

Production image:

```bash
docker build -t aurora-admin-service:local -f Dockerfile .
```

Dev image (air hot reload):

```bash
docker build -t aurora-admin-service:dev -f Dockerfile.dev .
```

## Linux Install (systemd)

Install assets:

- `install/install.sh`
- `install/aurora-admin.service`

Usage:

```bash
# generate env template
./install/install.sh --config

# install from env file
./install/install.sh -f ./aurora-admin.env.sample

# install specific release tag
./install/install.sh -f ./aurora-admin.env.sample -v v20260305120000-abc12345-alpha
```

Notes:

- Installer creates and runs service as user `aurora` (non-root service user).
- Installer can download release binaries from:
  - `https://github.com/phucle996/aurora-admin/releases`
- Installed systemd unit: `aurora-admin.service`.
- UI is already embedded in backend binary, no separate `dist` deployment step.

## Release Pipeline

GitHub Actions workflow:

- `.github/workflows/release.yml`

Current behavior:

- Build linux binaries (`amd64`, `arm64`)
- Package tarballs + checksums
- Publish GitHub release tag
