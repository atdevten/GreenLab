# GreenLab IoT Platform

An open-source IoT data platform built with Go and React. Collect sensor readings from devices, store time-series data, query historical and live data, trigger alerts, and monitor everything from a web dashboard — all through a clean REST + WebSocket API.

> **License:** Apache 2.0 · **Stack:** Go 1.22 + React 19 · **Status:** Active Development

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| Backend language | Go 1.22 | All backend services |
| Frontend | React 19 + TypeScript + Tailwind CSS 4 + Vite | Web dashboard |
| Relational DB | PostgreSQL 16 | Users, devices, channels, rules, audit |
| Time-series DB | InfluxDB 2.7 | Sensor readings / telemetry |
| Cache | Redis 7 | API key lookup, session tokens, query cache |
| Message bus | Kafka (Confluent 7.6) | Async event fan-out between services |
| Reverse proxy | nginx 1.27 | Single entry point (:8080) for all backend services |
| Container | Docker / Docker Compose | Local dev and deployment |
| HTTP framework | Gin | REST handlers in every service |
| Auth | RSA-signed JWT (RS256) + API Key | Users and devices respectively |

---

## Architecture

Seven independent backend services sit behind an nginx reverse proxy. The React frontend communicates with all services through the single gateway on port 8080.

```
                        ┌─────────────────────────────────────────────────┐
                        │              Client Layer                        │
                        │   React Dashboard (:5173 dev / :8080 prod)       │
                        └────────────────────┬────────────────────────────┘
                                             │ HTTP :8080
                                    ┌────────▼────────┐
                                    │     nginx       │
                                    │    :8080        │
                                    └──┬──────────┬───┘
                           JWT         │          │ API Key
                 ┌─────────────────────▼──┐   ┌───▼─────────────┐
                 │         iam            │   │    ingestion    │
                 │        :8001           │   │    :8003        │
                 │  auth + tenants        │   │  write-only     │
                 └───────────────┬────────┘   └────────┬────────┘
                                 │                     │ Kafka: raw.sensor.ingest
                 ┌───────────────▼────────┐   ┌────────▼────────┐
                 │    device-registry     │   │ normalization   │
                 │       :8002            │   │    :8006        │
                 │  devices/channels      │   │ validate+write  │
                 └────────────────────────┘   └────────┬────────┘
                                                       │ Kafka: normalized.sensor
                 ┌──────────────────┐   ┌─────────────┴────────────────┐
                 │   supporting     │   │                              │
                 │    :8007         │  ┌▼──────────────┐  ┌───────────▼──────┐
                 │  video + audit   │  │ query-realtime│  │alert-notification│
                 └──────────────────┘  │    :8004      │  │     :8005        │
                                       │ query+WS/SSE  │  │  rules + notify  │
                                       └───────────────┘  └──────────────────┘

  Infrastructure: PostgreSQL · InfluxDB · Redis · Kafka + ZooKeeper
```

---

## Documentation

| Doc | Description |
|---|---|
| [architecture.md](architecture.md) | Data flow, service responsibilities, communication patterns, security |
| [services.md](services.md) | Per-service reference: domains, entities, env vars, dependencies |
| [api.md](api.md) | All HTTP routes, auth levels, request/response formats |
| [development.md](development.md) | Setup, project structure, make targets, code conventions |
| [deployment.md](deployment.md) | Docker Compose, env vars, health checks, production guidance |

---

## Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/your-org/iot-platform.git && cd iot-platform

# 2. Generate RSA keys and start the full stack
make generate-keys && make up

# 3. Open the dashboard
open http://localhost:5173   # frontend dev server
# All backend APIs are available via nginx at http://localhost:8080
```

Run a single backend service locally (after infrastructure is up):

```bash
make run-iam
# or: make run-device-registry, run-ingestion, run-normalization,
#     run-query-realtime, run-alert-notification, run-supporting
```

Run the frontend dev server:

```bash
cd frontend && npm install && npm run dev
```

---

## License

Apache License 2.0. See [LICENSE](../LICENSE).
