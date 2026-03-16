# System Architecture

## Architecture Principles

| Principle | Application |
|---|---|
| **Boring is good** | PostgreSQL, Redis, Kafka — proven tools over trendy alternatives |
| **CQRS** | Write path (ingestion) is fully separated from read path (query-realtime) |
| **Event-driven** | Services communicate via Kafka topics; no direct service-to-service REST calls for data flow |
| **Clean Architecture** | Every service follows domain → application → infrastructure → transport layers |
| **Single responsibility** | Each service owns one bounded context; no shared databases between services |
| **Fail-safe auth** | Public key distributed to all services; only identity holds the private key |

---

## Data Flow

### Telemetry Ingestion Path

```
IoT Device
    │
    │  POST /v1/channels/{channel_id}/data
    │  X-API-Key: ts_<device_key>
    ▼
┌─────────────────────────────────────────────────────────┐
│                    ingestion :8003                       │
│                                                          │
│  1. Validate API key against Redis cache                 │
│  2. Publish reading to Kafka topic: raw.sensor.ingest    │
└──────────────────────┬──────────────────────────────────┘
                       │ Kafka: raw.sensor.ingest
                       ▼
┌─────────────────────────────────────────────────────────┐
│                  normalization :8006                     │
│                                                          │
│  1. Consume raw.sensor.ingest                           │
│  2. Validate and normalise reading                       │
│  3. Write to InfluxDB (telemetry bucket)                 │
│  4. Publish to Kafka topic: normalized.sensor            │
└──────────────────────┬──────────────────────────────────┘
                       │ Kafka: normalized.sensor
         ┌─────────────┴───────────────┐
         ▼                             ▼
┌────────────────┐           ┌──────────────────────────┐
│ query-realtime │           │   alert-notification     │
│    :8004       │           │       :8005              │
│                │           │                          │
│ Broadcasts to  │           │ Evaluates rules against  │
│ WebSocket and  │           │ each reading; if rule    │
│ SSE clients    │           │ triggers, publishes to   │
│ subscribed to  │           │ Kafka: alert.events      │
│ that channel   │           └──────────┬───────────────┘
└────────────────┘                      │ Kafka: alert.events
                                        ▼
                              ┌──────────────────────────┐
                              │   alert-notification     │
                              │   (alert consumer)       │
                              │                          │
                              │ Dispatches email/webhook │
                              │ notification             │
                              └──────────────────────────┘
```

### User Authentication Path

```
HTTP Client
    │
    │  POST /api/v1/auth/login
    ▼
┌─────────────────────────────────────────────────────────┐
│                       iam :8001                          │
│                                                          │
│  1. Verify credentials against PostgreSQL                │
│  2. Sign JWT with RSA-4096 private key                   │
│  3. Store refresh token in PostgreSQL                    │
│  4. Store JWT in Redis session cache                     │
│  5. Publish user.events to Kafka (audit trail)           │
└─────────────────────────────────────────────────────────┘
    │
    │  JWT in Authorization: Bearer <token>
    │
    ▼
Any other service (device-registry, query-realtime, etc.)
    │
    │  Validates JWT signature using shared RSA public key
    │  (no round-trip to identity service)
    ▼
  Request proceeds
```

---

## Service Responsibilities

| Service | Port | Bounded Context | Databases | Auth Method |
|---|---|---|---|---|
| `iam` | 8001 | Users, orgs, workspaces, JWT issuance | PostgreSQL, Redis | Issues JWT |
| `device-registry` | 8002 | Devices, channels, fields, API keys | PostgreSQL, Redis | JWT |
| `ingestion` | 8003 | Telemetry write path (Kafka-only) | Redis | API Key |
| `query-realtime` | 8004 | Historical queries, WebSocket/SSE push | InfluxDB, Redis | JWT |
| `alert-notification` | 8005 | Alert rules, event dispatch, notifications | PostgreSQL | JWT |
| `normalization` | 8006 | Raw → normalised pipeline; InfluxDB write | InfluxDB | none (Kafka worker) |
| `supporting` | 8007 | Video streams (S3), audit log | PostgreSQL, S3 | JWT |

---

## Shared Package (`shared/`)

All services import the shared Go module at `github.com/greenlab/shared`. It contains:

| Package | Contents |
|---|---|
| `pkg/logger` | Structured JSON logger (Uber Zap) with global singleton |
| `pkg/middleware` | `JWTAuth`, `OptionalJWTAuth`, `APIKeyAuth`, `RequestID`, `RateLimit` Gin middlewares |
| `pkg/apierr` | Typed API error types with HTTP status mapping |
| `pkg/response` | Standardised JSON response envelope helpers |
| `pkg/pagination` | Cursor-based pagination utilities |
| `pkg/kafka` | Generic producer and consumer wrappers |
| `pkg/validator` | Input validation helpers |

---

## Inter-Service Communication

### Synchronous (REST)

Services do not call each other's REST APIs during normal data flow. The only implicit coupling is:

- **ingestion** reads API keys from the Redis cache populated by **device-registry** when a device is created or a key is rotated.

### Asynchronous (Kafka Topics)

| Topic | Producer | Consumers | Payload |
|---|---|---|---|
| `raw.sensor.ingest` | ingestion | normalization | `ReadingEvent` JSON (id, type, published_at, reading{channel_id, device_id, fields, tags, timestamp}) |
| `normalized.sensor` | normalization | query-realtime, alert-notification | `ReadingEvent` JSON (normalised, after InfluxDB write) |
| `alert.events` | alert-notification (rule engine) | alert-notification (dispatcher) | `AlertEvent` JSON |
| `user.events` | iam | supporting (audit consumer) | User lifecycle events |

---

## Authentication & Security

### JWT (Human Users)

1. `identity` signs tokens with an RSA-4096 private key.
2. All other services validate the signature using the shared public key (mounted as a file).
3. No service-to-service call is needed for token validation — it is stateless.
4. Claims include: `sub` (user ID), `tenant_id`, `roles`, `exp`.

### API Key (Devices)

1. `device-registry` generates API keys in the format `ts_<64 hex chars>`.
2. Keys are stored in PostgreSQL and cached in Redis.
3. `ingestion` validates keys by querying Redis. Cache miss falls back to a direct lookup.
4. Keys are never reused after rotation — the old key is invalidated atomically.

### Security Layers

| Layer | Mechanism |
|---|---|
| Transport | HTTPS (TLS termination at load balancer in production) |
| Authentication | RSA-256 JWT / API Key per request |
| Authorization | Role-based (`admin`, `operator`, `viewer`) enforced in application layer |
| Tenant isolation | Every entity carries `workspace_id` / `tenant_id`; queries are scoped |
| Secrets | RSA keys mounted as read-only volumes; never in environment variables |
| Rate limiting | `RateLimit` middleware available per-route (Redis-backed) |
| Audit | All user actions published to `user.events` → stored by supporting service |
