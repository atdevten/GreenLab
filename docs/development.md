# Developer Guide

## Prerequisites

| Tool | Minimum Version | Notes |
| --- | --- | --- |
| Go | 1.22 | Required for workspace support and language features |
| Node.js | 20.19+ or 22.12+ | Required for the frontend (Vite requirement) |
| Docker | 24+ | Required for local infrastructure |
| Docker Compose | v2 (plugin) | Bundled with Docker Desktop / OrbStack |
| `make` | Any | GNU make or compatible |
| `openssl` | Any | For generating RSA keys |
| `golangci-lint` | v1.57+ | Optional, for `make lint` |

---

## Project Structure

```text
IoT/
├── docker-compose.yml          # Full stack: infrastructure + all services
├── Makefile                    # Top-level build targets
├── go.work                     # Go workspace (links all modules)
├── go.work.sum
├── .markdownlint.json          # Markdownlint configuration
├── keys/                       # RSA key pair (generated, git-ignored)
│   ├── private.pem
│   └── public.pem
├── frontend/                   # React 19 + TypeScript + Tailwind CSS 4 + Vite
│   ├── src/
│   │   ├── api/                # Axios client + per-service API modules
│   │   ├── contexts/           # AuthContext, ThemeContext, ToastContext
│   │   ├── hooks/              # useDebounce, useEscapeKey, useNav, useTheme
│   │   ├── pages/              # One component per route
│   │   ├── components/         # layout/ (Sidebar, Topbar) + ui/ (Button, Card, Badge…)
│   │   └── types/              # Shared TypeScript interfaces
│   ├── .env                    # VITE_API_URL (defaults to http://localhost:9080)
│   └── package.json
├── shared/                     # Shared Go module (github.com/greenlab/shared)
│   └── pkg/
│       ├── apierr/
│       ├── kafka/
│       ├── logger/
│       ├── middleware/
│       ├── pagination/
│       ├── response/
│       └── validator/
└── services/
    ├── iam/                    # :8001 — users, JWT, orgs, workspace members, API keys
    ├── device-registry/        # :8002 — devices, channels, fields
    ├── ingestion/              # :8003 — telemetry write path
    ├── query-realtime/         # :8004 — historical query + WS/SSE + CSV export
    ├── alert-notification/     # :8005 — alert rules + notifications (read state)
    ├── normalization/          # :8006 — raw → normalised pipeline worker (Kafka + InfluxDB)
    └── supporting/             # :8007 — video + audit (filtering + CSV export)
```

Each backend service follows the same internal layout:

```text
services/<name>/
├── cmd/server/main.go          # Entrypoint: wires dependencies, starts HTTP server
├── go.mod                      # Module: github.com/greenlab/<name>
└── internal/
    ├── application/            # Use cases / service layer
    ├── domain/                 # Entities, value objects, repository interfaces
    │   └── <subdomain>/
    ├── infrastructure/         # Adapter implementations (DB, cache, Kafka, HTTP clients)
    │   ├── influxdb/           # (if needed)
    │   ├── kafka/
    │   ├── postgres/
    │   ├── redis/
    │   └── s3/                 # (if needed)
    └── transport/
        └── http/               # Gin handlers, router
```

---

## Go Workspace

`go.work` declares a multi-module workspace containing all services and the shared module:

```text
go 1.22

use (
    ./shared
    ./services/iam
    ./services/device-registry
    ./services/ingestion
    ./services/normalization
    ./services/query-realtime
    ./services/alert-notification
    ./services/supporting
)
```

Changes to `shared/` are immediately reflected in all services during development without publishing a module version.

---

## Make Targets

| Target | Description |
| --- | --- |
| `make all` | Tidy all modules and build all services |
| `make tidy` | Run `go mod tidy` in shared and every service |
| `make build` | Compile all services; fails fast on first error |
| `make test` | Run `go test ./...` in every service |
| `make lint` | Run `golangci-lint run` in every service |
| `make up` | `docker compose up -d` — start full stack in background |
| `make down` | Stop and remove all containers |
| `make logs` | Follow logs from all containers |
| `make generate-keys` | Generate RSA-4096 key pair to `keys/` |
| `make run-<name>` | Run a single service from source (e.g. `make run-iam`) |
| `make mock` | Regenerate mocks for services that have a `.mockery.yaml` |

---

## Local Development Workflow

### First-time setup

```bash
# 1. Generate RSA key pair
make generate-keys

# 2. Start infrastructure only (Postgres, Redis, InfluxDB, Kafka, ZooKeeper)
docker compose up -d postgres redis influxdb zookeeper kafka

# 3. Run backend services from source (each in a separate terminal)
make run-iam
make run-device-registry
make run-ingestion
make run-normalization
make run-query-realtime
make run-alert-notification
make run-supporting

# 4. Run the frontend dev server
cd frontend && npm install && npm run dev
# Dashboard available at http://localhost:5173
# Backend API available at http://localhost:9080 (via nginx) or direct on service ports
```

### Running the full stack via Docker Compose

```bash
make generate-keys
make up          # builds images and starts all containers
make logs        # follow all logs
```

### Stopping

```bash
make down
# or to also remove volumes (wipes all data):
docker compose down -v
```

---

## Running a Single Service

```bash
# Using the Makefile shortcut
make run-iam

# Or directly
cd services/iam
go run ./cmd/server/main.go

# Override defaults
PORT=9001 LOG_LEVEL=debug make run-iam
```

---

## Running Tests

```bash
# All services
make test

# A single service
cd services/iam && go test ./...

# With verbose output
cd services/device-registry && go test -v ./...

# A specific package
cd services/iam && go test ./internal/domain/auth/...
```

---

## Frontend Development

```bash
cd frontend

# Install dependencies
npm install

# Start dev server (hot reload)
npm run dev

# Type-check
npx tsc --noEmit

# Build for production
npm run build
```

The frontend reads `VITE_API_URL` from `frontend/.env` (defaults to `http://localhost:9080`). All API calls go through the axios client at `src/api/client.ts`, which automatically attaches the JWT and handles token refresh on 401 responses.

---

## Code Conventions

### Clean Architecture Layers

The dependency rule is strict: inner layers must never import outer layers.

```text
domain ← application ← infrastructure
                      ← transport/http
```

| Layer | Allowed imports |
| --- | --- |
| `domain` | Standard library, `github.com/google/uuid`, `golang.org/x/crypto` |
| `application` | `domain`, shared interfaces only |
| `infrastructure` | `domain`, `application` interfaces, external SDKs |
| `transport/http` | `application`, `shared/pkg/middleware`, `shared/pkg/response` |

### Naming

- **Entities** — struct names match the domain noun (`User`, `Device`, `Channel`).
- **Repositories** — interface in `domain`, implementation in `infrastructure/postgres`.
- **Services** — `application.NewXxxService(...)` returns a concrete struct.
- **Handlers** — `transport/http.NewXxxHandler(svc)` takes the application service interface.
- **Constructors** — always `NewXxx(...)`, never bare struct literals in application code.

### Error Handling

- Domain functions return `(T, error)`.
- Application services wrap errors with `fmt.Errorf("action: %w", err)`.
- Handlers map errors to HTTP responses using `shared/pkg/apierr` types.

### Logging

Use `shared/pkg/logger`:

```go
log := logger.L()
log.Info("doing something", zap.String("id", id))
log.Error("failed", zap.Error(err))
```

Never use `fmt.Println` or the standard `log` package in service code.

---

## Adding a New Feature

1. **Domain** — Add/update entities, value objects, and repository interfaces in `internal/domain/<subdomain>/`.
2. **Application** — Add/update use case methods in `internal/application/`. Keep business logic here, not in handlers.
3. **Infrastructure** — Implement repository interfaces in `internal/infrastructure/postgres/` (or InfluxDB/Redis/Kafka). Add SQL migrations if needed.
4. **Transport** — Add route(s) to the relevant handler. Register in `router.go`.
5. **Mocks** — Run `make mock-<service>` to regenerate mocks after changing repository interfaces.
6. **Tests** — Write unit tests for domain logic and application layer. Mock repository interfaces.
7. **Tidy** — Run `make tidy && make build` to verify everything compiles.
8. **Docs** — Update `docs/api.md` with new routes and `docs/services.md` if entities changed.
