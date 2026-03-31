# Service Reference

## 1. iam

**Purpose:** Issues and manages user identities, JWT tokens, organisations, workspaces, workspace members, and org-level API keys. The only service that holds the RSA private key.

**Port:** `8001` · **Sub-domains:** `auth` · `tenant`

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| `User` | `id`, `tenant_id`, `email`, `password_hash`, `roles` (`admin`/`operator`/`viewer`), `status` (`pending`/`active`/`disabled`), `email_verified` |
| `Org` | `id`, `name`, `slug`, `plan` (`free`/`starter`/`pro`/`enterprise`), `owner_user_id` |
| `Workspace` | `id`, `org_id`, `name`, `slug`, `description` |
| `WorkspaceMember` | `id`, `workspace_id`, `user_id`, `name`, `email`, `role` (`owner`/`admin`/`member`/`viewer`), `joined_at` |
| `APIKey` | `id`, `tenant_id`, `user_id`, `name`, `key_prefix`, `key_hash`, `scopes`, `created_at`, `last_used` |
| `Token` | `id`, `user_id`, `refresh_token` (hashed), `expires_at` |

**Package Structure**

```text
services/iam/
├── cmd/server/main.go
└── internal/
    ├── application/        # AuthService, TenantService
    ├── domain/
    │   ├── auth/           # User entity, roles, statuses
    │   └── tenant/         # Org, Workspace, WorkspaceMember, APIKey entities
    ├── infrastructure/
    │   ├── kafka/          # EventProducer (user.events)
    │   ├── postgres/       # UserRepo, TokenRepo, OrgRepo, WorkspaceRepo, APIKeyRepo
    │   └── redis/          # Session token cache
    └── transport/http/     # AuthHandler, TenantHandler, Router
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| DB | PostgreSQL | Users, tokens, orgs, workspaces, members, API keys |
| Cache | Redis | Session token cache |
| Kafka | `user.events` (produce) | Publishes login/register/update events for audit |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8001` | HTTP listen port |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6380` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password (optional) |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `JWT_PRIVATE_KEY_PATH` | `keys/private.pem` | Path to RSA private key (PEM) |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | Path to RSA public key (PEM) |
| `JWT_ISSUER` | `greenlab-identity` | JWT `iss` claim value |
| `LOG_LEVEL` | `info` | Zap log level (`debug`/`info`/`warn`/`error`) |

---

## 2. device-registry

**Purpose:** CRUD management for devices, channels, and fields. Generates and rotates device API keys. Caches API key → device/channel mappings in Redis for fast lookup by ingestion.

**Port:** `8002` · **Sub-domains:** `device` · `channel` · `field`

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| `Device` | `id`, `workspace_id`, `name`, `description`, `api_key` (`ts_<64hex>`), `status` (`active`/`inactive`/`blocked`), `last_seen_at`, `metadata` (JSONB) |
| `Channel` | `id`, `workspace_id`, `device_id` (optional), `name`, `description`, `visibility` (`public`/`private`), `tags` (JSONB array) |
| `Field` | `id`, `channel_id`, `name`, `label`, `unit`, `field_type` (`float`/`integer`/`string`/`boolean`), `position` (1–8) |

**Package Structure**

```text
services/device-registry/
├── cmd/server/main.go
└── internal/
    ├── application/        # DeviceService, ChannelService, FieldService
    ├── domain/
    │   ├── device/         # Device entity, status, API key generation
    │   ├── channel/        # Channel entity, visibility; ListByDevice support
    │   └── field/          # Field entity, field types
    ├── infrastructure/
    │   ├── postgres/       # DeviceRepo, ChannelRepo (device_id filter), FieldRepo
    │   └── redis/          # DeviceCache (api_key → device/channel mapping)
    └── transport/http/     # DeviceHandler, ChannelHandler, FieldHandler, Router
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| DB | PostgreSQL | All entity persistence |
| Cache | Redis | API key → `(device_id, channel_id)` mapping for ingestion fast-path |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8002` | HTTP listen port |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6380` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password (optional) |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | RSA public key for JWT validation |
| `LOG_LEVEL` | `info` | Zap log level |

---

## 3. ingestion

**Purpose:** High-throughput write endpoint for sensor data. Validates device API keys and publishes raw readings to Kafka. Does **not** write to InfluxDB directly — that is handled by the normalization worker.

**Port:** `8003` · **Sub-domains:** `reading` (write path only)

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| `Reading` | `channel_id`, `device_id`, `fields` (map[string]float64), `tags` (map[string]string), `timestamp` |

**Package Structure**

```text
services/ingestion/
├── cmd/server/main.go
└── internal/
    ├── application/        # IngestService, EventPublisher interface
    ├── domain/             # Reading, validation errors
    ├── infrastructure/
    │   ├── apikey/         # APIKeyValidator (wraps Redis cache + Postgres fallback)
    │   ├── kafka/          # ReadingProducer (raw.sensor.ingest)
    │   └── redis/          # APIKeyCache (api_key validation)
    └── transport/http/     # Handler (Ingest, BulkIngest), Router
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| Cache | Redis | Validates API keys without hitting device-registry |
| Kafka | `raw.sensor.ingest` (produce) | Raw reading events for normalization worker |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8003` | HTTP listen port |
| `REDIS_ADDR` | `localhost:6380` | Redis address |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `LOG_LEVEL` | `info` | Zap log level |

---

## 4. normalization

**Purpose:** Two-stage pipeline worker. Consumes raw reading events from `raw.sensor.ingest`, validates and normalises each reading, writes it to InfluxDB, then publishes a `normalized.sensor` event for downstream consumers (query-realtime, alert-notification). Has no HTTP API of its own beyond a `/health` liveness endpoint.

**Port:** `8006` · **Sub-domains:** none (background worker)

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| `ReadingPayload` | `channel_id`, `device_id`, `fields` (map[string]float64), `tags` (map[string]string), `timestamp` |
| `ReadingEvent` | `id`, `type`, `published_at`, `reading` (`ReadingPayload`) |

**Package Structure**

```text
services/normalization/
├── cmd/server/main.go
└── internal/
    ├── application/        # NormalizationService (Process)
    ├── domain/             # ReadingPayload, ReadingEvent
    ├── infrastructure/
    │   ├── influxdb/       # Writer (writes to telemetry bucket)
    │   └── kafka/          # ReadingConsumer (raw.sensor.ingest), NormalizedProducer (normalized.sensor)
    └── (no transport layer — HTTP is health-only via main.go)
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| DB | InfluxDB | Persists normalised readings to `telemetry` bucket |
| Kafka | `raw.sensor.ingest` (consume, group: `normalization-service`) | Ingest pipeline input |
| Kafka | `normalized.sensor` (produce) | Fan-out to query-realtime and alert-notification |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8006` | HTTP listen port (health check only) |
| `INFLUXDB_URL` | `http://localhost:8086` | InfluxDB base URL |
| `INFLUXDB_TOKEN` | *(required)* | InfluxDB auth token |
| `INFLUXDB_ORG` | `greenlab` | InfluxDB organisation |
| `INFLUXDB_BUCKET` | `telemetry` | InfluxDB bucket name |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `LOG_LEVEL` | `info` | Zap log level |

---

## 5. query-realtime

**Purpose:** Serves historical time-series queries from InfluxDB and pushes live telemetry to connected clients via WebSocket and SSE. Supports CSV export. Consumes `normalized.sensor` from Kafka to drive the realtime hub.

**Port:** `8004` · **Sub-domains:** `query` (historical) · `realtime` (WebSocket/SSE hub)

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| Query params | `channel_id`, `field`, `start`, `end`, `aggregation`, `window`, `format` |
| Hub | In-memory fan-out from Kafka to registered WS/SSE clients |

**Package Structure**

```text
services/query-realtime/
├── cmd/server/main.go
└── internal/
    ├── application/        # QueryService (InfluxDB queries + CSV export), Hub
    ├── domain/
    │   ├── query/          # Query model
    │   └── realtime/       # Hub subscription model
    ├── infrastructure/
    │   ├── influxdb/       # Reader
    │   ├── kafka/          # ReadingConsumer (normalized.sensor)
    │   └── redis/          # Query result cache
    └── transport/http/     # QueryHandler (CSV branch on ?format=csv), RealtimeHandler, Router
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| DB | InfluxDB | Historical time-series reads |
| Cache | Redis | Query result caching |
| Kafka | `normalized.sensor` (consume, group: `query-realtime-group`) | Feed realtime hub |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8004` | HTTP listen port |
| `INFLUXDB_URL` | `http://localhost:8086` | InfluxDB base URL |
| `INFLUXDB_TOKEN` | `my-super-secret-token` | InfluxDB auth token |
| `INFLUXDB_ORG` | `greenlab` | InfluxDB organisation |
| `INFLUXDB_BUCKET` | `telemetry` | InfluxDB bucket |
| `REDIS_ADDR` | `localhost:6380` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password (optional) |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | RSA public key for JWT validation |
| `LOG_LEVEL` | `info` | Zap log level |

---

## 6. alert-notification

**Purpose:** Manages alert rules; evaluates incoming telemetry against those rules; dispatches notifications via email or webhook when a rule fires.

**Port:** `8005` · **Sub-domains:** `alert` (rules, events) · `notification` (delivery, read state)

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| `Rule` | `id`, `channel_id`, `workspace_id`, `name`, `field_name`, `condition` (`gt`/`gte`/`lt`/`lte`/`eq`/`neq`), `threshold`, `severity` (`info`/`warning`/`critical`), `enabled`, `cooldown_sec` |
| `AlertEvent` | `id`, `rule_id`, `channel_id`, `field_name`, `actual_value`, `threshold`, `severity`, `triggered_at` |
| `Notification` | `id`, `workspace_id`, `channel_type` (`email`/`webhook`), `recipient`, `subject`, `body`, `status` (`pending`/`sent`/`failed`), `read`, `read_at`, `retries` |

**Package Structure**

```text
services/alert-notification/
├── cmd/server/main.go
└── internal/
    ├── application/        # RuleEngine, AlertService, Dispatcher, NotificationService
    ├── domain/
    │   ├── alert/          # Rule, AlertEvent, RuleRepository
    │   └── notification/   # Notification entity (read/read_at), MarkRead method
    ├── infrastructure/
    │   ├── email/          # SMTPSender
    │   ├── kafka/          # AlertProducer, TelemetryConsumer, AlertConsumer
    │   ├── postgres/       # RuleRepo, NotificationRepo (MarkRead, MarkAllRead)
    │   └── webhook/        # HTTP webhook client
    └── transport/http/     # AlertHandler, NotificationHandler (read routes), Router
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| DB | PostgreSQL | Alert rules, notifications |
| Kafka | `normalized.sensor` (consume, group: `alert-notification-telemetry-group`) | Evaluate rules |
| Kafka | `alert.events` (produce) | Publish triggered alerts |
| Kafka | `alert.events` (consume, group: `alert-notification-alert-group`) | Dispatch notifications |
| External | SMTP server | Email delivery |
| External | HTTP endpoints | Webhook delivery |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8005` | HTTP listen port |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | PostgreSQL connection string |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `SMTP_HOST` | `smtp.example.com` | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP server port |
| `SMTP_USERNAME` | `` | SMTP auth username |
| `SMTP_PASSWORD` | `` | SMTP auth password |
| `SMTP_FROM` | `noreply@greenlab.io` | From address for alert emails |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | RSA public key for JWT validation |
| `LOG_LEVEL` | `info` | Zap log level |

---

## 7. supporting

**Purpose:** Handles cross-cutting concerns: video stream metadata with S3 URL generation, and an append-only audit log populated via Kafka. Audit events support `?resource_type`, `?search` filtering, and CSV export.

**Port:** `8007` · **Sub-domains:** `video` · `audit`

**Key Entities**

| Entity | Key Fields |
| --- | --- |
| `Stream` | `id`, `workspace_id`, `name`, `status`, `s3_key`, `content_type`, `size_bytes`, `created_at` |
| `AuditEvent` | `id`, `tenant_id`, `user_id`, `user_name`, `action`, `resource_type`, `resource_id`, `target`, `ip`, `metadata` (JSONB), `occurred_at` |

**Package Structure**

```text
services/supporting/
├── cmd/server/main.go
└── internal/
    ├── application/        # VideoService, AuditService (ListTenantFilter)
    ├── domain/
    │   ├── video/          # Stream entity
    │   └── audit/          # AuditEvent entity
    ├── infrastructure/
    │   ├── kafka/          # AuditConsumer (user.events)
    │   ├── postgres/       # StreamRepo, EventRepo (dynamic WHERE + CSV export)
    │   └── s3/             # Storage (presigned URLs)
    └── transport/http/     # VideoHandler, AuditHandler (CSV branch), Router
```

**Dependencies**

| Type | Resource | Usage |
| --- | --- | --- |
| DB | PostgreSQL | Stream metadata, audit events |
| Kafka | `user.events` (consume, group: `supporting-audit-group`) | Persist audit trail |
| External | AWS S3 | Video file storage (presigned upload/download URLs) |

**Environment Variables**

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8007` | HTTP listen port |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | PostgreSQL connection string |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `AWS_REGION` | `us-east-1` | AWS region for S3 |
| `S3_BUCKET` | `greenlab-video` | S3 bucket name |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | RSA public key for JWT validation |
| `LOG_LEVEL` | `info` | Zap log level |
