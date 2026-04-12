# GreenLab IoT Platform — TODOS

Tracked deferred work. Items added via `/plan-ceo-review` on 2026-03-16, 2026-03-17, and 2026-03-29.

---

## Execution Order (Start Here)

Unblocked items first, then dependency chain. Build in this sequence:

```
IMMEDIATE (no dependencies, security-critical):
  TODO-003  Fix SSRF in webhook dispatch
  TODO-020  Fix realtime auth (return true → real check)
  TODO-022  Remove fake data from DevicesPage/ChannelsPage

THEN (P1 foundation chain):
  TODO-015  Provision endpoint (atomic device+channel+fields)
  TODO-013  API key version counter (cache invalidation)
  TODO-014  Workspace scoping on device/channel endpoints
  TODO-016  Cascade soft-delete channels on DeleteDevice
  TODO-005  Separate Postgres DBs per service   ← needs TODO-012 (DONE)
  TODO-006  Kafka-first ingestion refactor       ← needs TODO-005

PARALLEL (no blocking deps, do alongside the chain):
  TODO-018  Hash API key in Redis cache
  TODO-019  Add logger to ChannelService + RateLimit
  TODO-023  Add channel_id + request_id to ingest response
  TODO-024  BulkIngest max batch size (max=1000)
  TODO-025  make deploy with health-check gating
  TODO-033  Verify InfluxDB dedup key            ← needed before TODO-028

INTEGRATION TESTS (after foundation chain):
  TODO-021  Integration tests (auth + provision + version counter + cross-tenant isolation)
```

---

## P1 — Must do before production

### ~~[TODO-001] Add .gitignore~~ ✅ DONE (2026-03-17)
`.gitignore` added covering `keys/`, `.env*`, `coverage.out`, build output, IDE files.

---

### ~~[TODO-002] Fix Kafka publish failure silently swallowed in IngestService~~ ✅ DONE (2026-03-17)
Handler now returns 503 on Kafka publish failure. Device receives `service_unavailable` and can retry.

---

### [TODO-003] Fix SSRF vulnerability in webhook dispatch
**What:** Alert destinations store webhook URLs in `config JSONB` with no validation. The dispatcher HTTP-POSTs to whatever URL is stored.
**Why:** An authenticated user can target cloud metadata endpoints (AWS IMDS: `169.254.254.169/latest/meta-data/`), internal Redis/Postgres, or any VPC-internal service. This leaks credentials and enables lateral movement.
**How to fix:** In `services/alert-notification/internal/infrastructure/webhook/client.go`, validate URLs before dispatch:
  - HTTPS only (reject HTTP)
  - Reject RFC1918 ranges: 10.x.x.x, 172.16-31.x.x, 192.168.x.x
  - Reject link-local: 169.254.x.x
  - Reject localhost: 127.x.x.x
**Effort:** S
**Depends on:** Nothing

---

### [TODO-004] ~~Add database migration tooling~~ ✅ DONE (2026-03-16)
Migration files rewritten to match Go code (fixed schema drift in device-registry, alert-notification, supporting).
Indexes added to all 4 services. `make migrate-all` added. Startup schema check added to all 4 Postgres services.
`docs/schema.sql` deleted — migrations are now the source of truth.

---

### [TODO-005] Separate PostgreSQL databases per service in docker-compose
**What:** All 6 services currently point to `POSTGRES_DB: greenlab`. Give each service its own DB: `iam_db`, `device_registry_db`, `alert_db`, `supporting_db`.
**Why:** Architecture principle P3 states each service owns its data. Sharing one DB means a migration from `iam` can accidentally drop a table used by `alert-notification`.
**How to apply:** Add `POSTGRES_MULTIPLE_DATABASES=iam_db,device_registry_db,...` init script to postgres container, or use `psql -c "CREATE DATABASE ..."` in an init SQL file.
**Effort:** S
**Depends on:** Ingestion → device-registry HTTP refactor (TODO-012) must be complete first so ingestion no longer queries device-registry's DB

---

### [TODO-006] Refactor ingestion to Kafka-first (two-stage pipeline)
**What:**
1. Remove InfluxDB write from `ingest_service.go`. Change to publish-only to `raw.sensor.ingest` Kafka topic.
2. Create new `services/normalization` worker: consumes `raw.sensor.ingest`, validates schema, writes to InfluxDB and Kafka `normalized.sensor`.
3. Migrate API endpoint: `/api/v1/ingest` → `POST /v1/channels/{channel_id}/data` and bulk variant.

**Why:** Three key decisions made in plan-ceo-review:
  - Two-stage pipeline (Kafka-first) — enables replay, decouples storage from ingest
  - Kafka publish failure must be an error (TODO-002)
  - API endpoint alignment with solution doc

**Architecture after refactor:**
```
IoT Device
    │  POST /v1/channels/{id}/data
    ▼
ingestion :8003
    │  publish only → raw.sensor.ingest
    ▼
Kafka
    │  consume
    ▼
normalization worker (new service)
    │  write
    ├──▶ InfluxDB (hot tier)
    └──▶ Kafka: normalized.sensor (fan-out to alert, query, Flink)
```

**Effort:** M
**Depends on:** TODO-005

---

## P2 — Before launch

### [TODO-007] Add per-channel rate limiting
**What:** Redis sliding-window counter keyed per `channel_id` enforced in ingestion middleware. Return `429 Too Many Requests` with `Retry-After` header when exceeded.
**Why:** Without rate limiting any device can flood the system, starving other tenants and overwhelming Kafka.
**Config:** Default 1 msg/sec, burst 10, configurable per channel in device-registry.
**Note:** Global rate limiting (100 req/min per API key) is already in place as of 2026-03-17 via `RateLimit` middleware. This TODO is for the finer-grained per-channel limit.
**Effort:** S
**Depends on:** TODO-006, TODO-012 (device-registry returns channel config in validate-api-key response)

---

### [TODO-008] Add OpenTelemetry instrumentation
**What:** OTel SDK in all 6 services. Trace ID generation at ingestion entry point. Trace ID propagation through Kafka message headers. Jaeger in docker-compose for local tracing.
**Why:** Solution doc mandates OTel. Without it, a message that causes a downstream alert failure is completely untraceable across service boundaries.
**Effort:** M
**Depends on:** TODO-006 (Kafka headers for trace propagation)

---

## Vision / Delight Items

### [TODO-009] CSV export endpoint
**What:** `GET /v1/channels/{id}/data?format=csv` — same query params as JSON, returns RFC 4180 CSV with headers matching field names.
**Why:** Excel/Google Sheets users shouldn't need to write code to get their data. Expands the non-developer user base.
**Effort:** S
**Depends on:** Nothing

---

### [TODO-010] Webhook delivery logs
**What:** Store each webhook delivery attempt (URL, HTTP status, latency, response body snippet) in PostgreSQL. Expose via `GET /v1/channels/{id}/alerts/{alert_id}/deliveries`.
**Why:** "Why didn't I get the alert?" is the most common support question. Self-service answer in the UI removes friction.
**Effort:** S
**Depends on:** Nothing

---

### [TODO-011] ThingSpeak compatibility shim
**What:** `GET /update?api_key={key}&field1={v}&field2={v}` — maps `field1`–`field8` to the channel's custom field names, proxies to the standard write path.
**Why:** Existing ThingSpeak users can migrate without reflashing device firmware.
**Effort:** S
**Depends on:** TODO-006

---

## In Progress — 2026-03-17 (from plan-ceo-review)

All items below were decided during the 2026-03-17 plan-ceo-review. Build in one diff.

### ~~[TODO-012] Refactor ingestion: DeviceStore → device-registry HTTP client~~ ✅ DONE (2026-03-29)
`services/ingestion/internal/infrastructure/deviceregistry/client.go` implements the HTTP client. Ingestion no longer queries device-registry's Postgres directly.

**What (was):** Replace `services/ingestion/internal/infrastructure/postgres/device_store.go` with an HTTP client calling `POST device-registry/internal/validate-api-key {api_key, channel_id}`. Add `POST /internal/validate-api-key` endpoint to device-registry (returns `{device_id, field_names[], version}`).
**Why:** Ingestion directly queries device-registry's Postgres tables — violates P3 and blocks TODO-005.
**Effort:** M | **Priority:** P1 | **Depends on:** Nothing

---

### [TODO-013] API key version counter for instant cache invalidation
**What:** On `RotateAPIKey` and `DeleteDevice`, device-registry increments `device_version:{device_id}` in Redis. Ingestion cache stores `{device_id, channel_id, field_names, version}`. On cache hit, ingestion reads `device_version:{device_id}` and rejects stale entries.
**Why:** Without this, old API keys remain valid for up to 10 minutes after rotation — a security gap.
**Effort:** S | **Priority:** P1 | **Depends on:** TODO-012

---

### [TODO-014] Workspace scoping on all device/channel endpoints
**What:** All device and channel GET/PUT/DELETE handlers must verify the resource's `workspace_id` matches the caller's workspace (extracted from JWT claims). Touches ~8 handler methods.
**Why:** Currently any authenticated user can read/modify/delete any device if they know its UUID.
**Effort:** M | **Priority:** P1 | **Depends on:** IAM service emitting workspace_id in JWT claims

---

### [TODO-015] POST /api/v1/devices/provision — atomic device + channel + fields
**What:** Single transactional endpoint creating device + channel + fields in one Postgres transaction. `RegisterDeviceDrawer` uses this instead of three separate calls. Request: `{device: {...}, channel: {...}, fields: [...]}`. Response: fully created device with channel and fields.
**Why:** Current three-call sequence leaves orphaned devices/channels on partial failure (no rollback).
**Effort:** M | **Priority:** P1 | **Depends on:** Nothing

---

### [TODO-016] Cascade soft-delete channels on DeleteDevice
**What:** When a device is soft-deleted, soft-delete all of its owned channels in the same transaction. Currently `ON DELETE SET NULL` leaves orphaned channels visible in workspace listings.
**Why:** "Delete device" should mean "delete everything owned by that device."
**Effort:** S | **Priority:** P1 | **Depends on:** Channel soft-delete support (add `deleted_at` to channels table)

---

### ~~[TODO-017] Field schema validation at ingestion boundary~~ ✅ DONE (2026-03-29)
`ErrUnknownFieldIndex` validation is live in the deserializer pipeline. Unknown field names return 400 with context.

**What (was):** Field schema validation at ingestion boundary.
**What:** The `/internal/validate-api-key` response includes `field_names[]`. On each write, ingestion validates that all keys in `fields` map exist in `field_names[]`. Unknown field names return 400 with the error: `"unknown field X, channel has: [temperature, humidity]"`.
**Why:** Currently unknown field names are silently dropped by the normalization worker — device developers get no feedback about typos in field names.
**Effort:** S | **Priority:** P2 | **Depends on:** TODO-012, TODO-013

---

### [TODO-018] Fix device-registry Redis cache key: hash API key
**What:** `DeviceCache` uses `device:apikey:{raw_api_key}` as the Redis key — the plaintext key is visible in Redis key space. Change to `device:apikey:{sha256(apiKey)}` matching the pattern ingestion already uses.
**Why:** Anyone with Redis `KEYS *` access can enumerate all device API keys.
**Effort:** S | **Priority:** P2 | **Depends on:** Nothing

---

### [TODO-019] Add logger to ChannelService + RateLimit middleware
**What:** (a) Inject `*slog.Logger` into `ChannelService` — DB errors are currently invisible server-side. (b) Add `*slog.Logger` to `RateLimit` middleware — Redis failures fail open silently with no log.
**Why:** Observability gap: DB errors in channel operations and Redis outages during rate limiting produce no server-side trace.
**Effort:** S | **Priority:** P2 | **Depends on:** Nothing

---

### [TODO-020] Fix query-realtime authorization (return true → real check)
**What:** `realtime_handler.go:21` returns `true` unconditionally. Replace with: public channels open to all, private channels require valid JWT (`OptionalJWTAuth` + channel visibility check via device-registry).
**Why:** Private channel real-time streams are fully exposed — any WebSocket client can subscribe.
**Effort:** S | **Priority:** P1 | **Depends on:** Nothing

---

### [TODO-021] Integration tests: auth + provision + version counter
**What:** Add integration tests using testcontainers (Redis + Postgres). Cover: cache hit/miss/version-mismatch paths, provision rollback on channel/field failure, key rotation invalidates cache immediately.
Key tests: `TestAPIKeyAuth_VersionMismatch_Revalidates`, `TestProvisionDevice_ChannelCreateFails_RollsBack`, `TestRotateAPIKey_IncrementsVersionCounter`, `TestBulkIngest_ExceedsMaxReturns400`, `TestCrossTenant_DeviceA_CannotReadDeviceB`, `TestCrossTenant_ChannelA_CannotWriteChannelB`.
**Why:** Mock-based tests won't catch Redis key format or version comparison bugs.
**Effort:** M | **Priority:** P1 | **Depends on:** TODO-012, TODO-013, TODO-015

---

### [TODO-022] Remove seed/fake data from DevicesPage and ChannelsPage
**What:** Strip hardcoded `initial[]` devices, `fakeKey()`, `genKey()`, and `CHANNELS[]` arrays from `DevicesPage.tsx` and `ChannelsPage.tsx`. Handle empty state properly (empty state UI, not fake data).
**Why:** Fake API keys and device names ship to staging environments and confuse contributors.
**Effort:** S | **Priority:** P1 | **Depends on:** API endpoints working correctly

---

### [TODO-023] Add channel_id + request_id to Ingest 201 response
**What:** Extend `IngestResponse` to include `{accepted, written_at, channel_id, request_id}`. The `request_id` comes from the `X-Request-ID` header set by `RequestID` middleware.
**Why:** Lets device firmware log the exact channel and request so server-side issues can be correlated from device logs.
**Effort:** S | **Priority:** P2 | **Depends on:** Nothing

---

### [TODO-024] Add BulkIngest max batch size (max=1000)
**What:** Add `validate:"max=1000"` to `BulkIngestRequest.Readings`. Return 400 with `"batch size exceeds maximum of 1000"`.
**Why:** No limit allows OOM allocation before Kafka rejects oversized messages with an opaque error.
**Effort:** S | **Priority:** P2 | **Depends on:** Nothing

---

### [TODO-025] Add make deploy target with ordered health-check gating
**What:** `make deploy` deploys device-registry first, polls `/health` until 200, then deploys ingestion.
**Why:** Ingestion must not start before device-registry's `/internal/validate-api-key` endpoint is live — otherwise all writes fail with 503.
**Effort:** S | **Priority:** P1 | **Depends on:** TODO-012

---

## Data Format Optimization — from /plan-ceo-review on 2026-03-21

### ~~[TODO-026] Compact Format Ingestion (5-format deserializer + format dispatcher)~~ ✅ DONE (2026-03-29)
All 4 active deserializers shipped: OJson, MsgPack, Binary, JSON. Protobuf stub returns 501. Format dispatcher in `handler.go`. Field index validation active. Binary DEVID validation active.

**What (was):** Format dispatcher middleware + 4 new deserializers (optimized JSON, MessagePack, Protobuf, custom binary).
**What:** Format dispatcher middleware + 4 new deserializers (optimized JSON, MessagePack, Protobuf, custom binary). Each produces `IngestInput` — application layer unchanged. Extends `validate-api-key` response to include `{field_index_map, schema_version}`. Adds `GET /v1/channels/{id}/schema` endpoint to device-registry (auth required).
**Security requirements:** Binary frame — validate `payload.DEVID == auth.device_id` (403 on mismatch). Protobuf — post-unmarshal index validation against field_index_map (400 on unknown index). Cap proto.Unmarshal input at 64KB. Schema endpoint requires API key or JWT.
**Code quality:** Extract `CompactFormatDeserializer` interface with shared `resolveFieldIndices` and `resolveTDOffsets` helpers. Split binary deserializer into `BinaryFrameParser` + `BinaryFrameValidator`.
**Why:** Enables 97% bandwidth reduction (350B → ~7B/reading). Correctness gap: field validation must be explicit — Protobuf silent-drops unknown fields.
**Priority:** P2 | **Effort:** L human / M CC+gstack | **Depends on:** TODO-012

---

### [TODO-027] Per-Field Timestamp Deltas (`td` field in compact formats)
**What:** Add `td []uint16` (milliseconds since batch timestamp) to all compact format specs. Server: `resolveTDOffsets(baseTS time.Time, td []uint16) ([]time.Time, error)`. Populates `IngestInput.FieldTimestamps`. Overflow validation: any td[i] > 65535 → 400 `timestamp_delta_overflow`. SDK must split batch on overflow; log `batch_split_reason: td_overflow` for metrics.
**Why:** Without `td`, compact formats lose per-field timestamp precision that the JSON API (`FieldTimestamps`) already provides. Correctness parity fix.
**Priority:** P2 | **Effort:** S human / XS CC+gstack | **Depends on:** TODO-026

---

### [TODO-028] OTA Schema Update Protocol
**What:** `schema_version uint32` is mandatory in every compact format batch. On mismatch: server returns 409 Conflict `{current_version, schema_url}`. SDK fetches `GET /v1/channels/{id}/schema` and retries within 5s. Redis tracks per-device ACK'd version (key: `schema_ack:{channel_id}:{device_id}`, TTL 30 days). Platform waits for ≥80% of active devices (active = request in last 7 days) to ACK before changing `X-Recommended-Format`. Stuck devices after 14 days: operator can force-deprecate, old schema_version returns 410; devices have 48h to fetch new version.
**Field index rules:** Indices 1-255, sequential, never recycled. New fields always append: `next_index = MAX(historical_indices)+1`. Deleted fields marked `deprecated: true`. Renaming does NOT bump schema_version. Add/delete/type-change bumps version.
**Why:** Adding a field to a channel silently corrupts binary-format devices or drops data via Protobuf's unknown-field-ignore behavior.
**Priority:** P2 | **Effort:** M human / S CC+gstack | **Depends on:** TODO-026

---

### [TODO-029] Offline Replay Endpoint
**What:** `POST /v1/channels/{id}/replay` — accepts batch with timestamps up to `replay_window_days` old (default 30 days, configurable per channel via `PATCH /v1/channels/{id}`, range 1-365). Timestamps validated against replay window (not maxReadingAge). All replayed readings tagged `replay: true` in Kafka headers (analytics must not show replay data in live dashboards). Returns 400 `timestamp_out_of_replay_window` if too old. InfluxDB deduplicates by (channel_id, timestamp) — live data wins on conflict.
**Why:** `maxReadingAge` on normal ingest rejects buffered offline readings. Tier 1 devices (LoRa, NB-IoT) routinely offline for hours/days.
**Priority:** P2 | **Effort:** M human / S CC+gstack | **Depends on:** TODO-026

---

### [TODO-030] Official Python + Go SDKs (Phase 1)
**What:** Python package (`pip install greenlab-iot`) and Go module. SDK handles: MessagePack serialization, field name→index resolution at init (fetches schema + caches locally), batching with configurable size, per-field `td` calculation and batch-split on overflow, LZ4 compression for batches >100 bytes, retry on 409 (schema re-fetch + retry), `X-Recommended-Format` header processing + auto-switch. Format preference and schema_version persisted in local config file on reboot.
**Phase 2 (separate TODO):** ESP32/Arduino C++ SDK with binary frame + offline buffering.
**Why:** The 5-format spec is useless without a library. Device engineers must call `sdk.setField("temp", 28.1)` and `.send()` — not implement MessagePack serialization from scratch.
**Priority:** P2 | **Effort:** L human / M CC+gstack | **Depends on:** TODO-026, TODO-027, TODO-028

---

### [TODO-031] Kafka DLQ for failed publishes on /replay endpoint
**What:** When the Kafka broker is unavailable during replay ingestion, the reading is silently lost — the device already received 201. Add a retry mechanism (3× with exponential backoff) on Kafka publish for the replay path. If retries exhausted, write to a DLQ (Postgres `replay_dlq` table or Redis list) for operator reprocessing. Emit metric `replay_publish_failure_total`.
**Why:** Replayed readings represent data that was already lost once (device was offline). Silent server-side loss on broker downtime is unacceptable for Tier 1 offline-buffering use case.
**Context:** Pre-existing gap on `/data` too (Kafka unavailable → 503), but `/replay` returns 201 before confirming Kafka publish — higher severity. Start with retry; DLQ is the complete fix.
**Effort:** M human / S CC+gstack | **Priority:** P2 | **Depends on:** TODO-029

---

### ~~[TODO-032] Redis ACK storage schema for schema_version OTA tracking~~ ✅ DONE (v0.1.1.0 — 2026-04-12)
**What:** Design and implement Redis key structure for per-device schema_version acknowledgement tracking. Key: `schema_ack:{channel_id}:{device_id}`, value: `uint32` (highest ACK'd version), TTL: 30 days. Implement 80% active-device threshold logic (active = request in last 7 days, count key: `schema_active:{channel_id}:{window}`). Implement 14-day force-deprecation path: operator endpoint `POST /v1/channels/{id}/schema/force-deprecate` sets old version to return 410. Devices have 48h to fetch new version (tracked in `schema_stuck:{channel_id}:{device_id}`). Add cleanup job for expired ACK keys.
**Why:** Without this, TODO-028's safe OTA rollout guarantee (≥80% ACK before format recommendation changes) cannot be enforced. The 14-day force-deprecation path prevents "stuck" devices from blocking schema evolution indefinitely.
**Context:** CEO plan flagged this as unresolved feasibility concern #1. The ACK mechanism is simple (one Redis SET per request with schema_version); the 80% threshold logic and force-deprecation operator path are the complex parts.
**Effort:** M human / S CC+gstack | **Priority:** P2 | **Depends on:** TODO-028
**Completed:** v0.1.1.0 (2026-04-12)

---

### [TODO-033] Verify InfluxDB dedup key for 409-retry idempotency
**What:** Confirm that the InfluxDB write path uses `(channel_id, timestamp)` as the dedup key. If InfluxDB uses a different dedup strategy (e.g., per-field, or includes tag set), 409-schema-mismatch retries in TODO-028 could create duplicate readings. Check `services/ingestion/internal/infrastructure/influxdb/writer.go` (or equivalent). If the dedup key is wrong: fix the write path to ensure exact-timestamp writes are idempotent. Add an integration test: write same (channel_id, timestamp) twice, verify InfluxDB contains exactly one record.
**Why:** The 409 → fetch schema → retry flow is only safe if the retry is idempotent. Silent duplicate readings in InfluxDB corrupt time-series analytics.
**Context:** CEO plan flagged this as unresolved feasibility concern #2. Must be verified (not assumed) before TODO-028 ships.
**Effort:** S human / XS CC+gstack | **Priority:** P1 | **Depends on:** TODO-028 (blocks it)

---

### [TODO-034] X-Recommended-Format adaptive format negotiation
**What:** Emit `X-Recommended-Format: {format}` header on every 2xx ingestion response. Initially static: always emit `msgpack` for HTTP devices, `binary` for MQTT Tier 1 (≤8 fields). Phase 2: adaptive logic — track per-device format metrics in Redis (success rate, bytes/reading, rolling 24h window). Switch recommendation when alternative format shows ≥15% size reduction AND ≥99.5% success rate over 24h. SDK reads header and applies on next connect without requiring developer action. Emit `format_recommendation_total{from,to}` metric on every switch.
**Why:** Closes the device feedback loop — devices can auto-upgrade to more efficient formats without developer intervention. Drives the ≥50% MessagePack adoption metric in the CEO plan success criteria.
**Context:** Static header emission is a 10-line change; adaptive logic requires Redis metric tracking. Ship static first (fold into TODO-026), then add adaptive logic as Phase 2 in this TODO.
**Effort:** M human / S CC+gstack | **Priority:** P2 | **Depends on:** TODO-026

---

## New Items — from /plan-ceo-review on 2026-03-29

### [TODO-035] InfluxDB data retention / TTL policy per channel
**What:** Add configurable data retention per channel. Default: 90 days. Range: 1–365 days. Store in `channels.retention_days` column. On channel create/update, call InfluxDB bucket API to set retention policy. Warn operators when a channel's bucket is >80% of allocated retention window. Add admin endpoint: `GET /api/v1/admin/storage/usage`.
**Why:** Without retention policy, InfluxDB fills up indefinitely. No graceful handling means a disk crisis in production with no warning.
**Effort:** S human / XS CC+gstack | **Priority:** P1 | **Depends on:** Nothing

---

### [TODO-036] Device heartbeat / offline alert
**What:** Background job (runs every 5 min) that checks `last_seen_at` for all active devices. If a device has not sent a reading in `heartbeat_timeout` (default 10 min, configurable per device), emit a `device.offline` alert via the existing alert-notification service. When the device sends again, emit `device.online` to close the alert. Add `heartbeat_timeout_minutes` field to device settings.
**Why:** "Is my sensor still on?" is the #1 question from operators. Without offline detection, silent device failures go unnoticed until crops die or data gaps appear.
**Effort:** S human / XS CC+gstack | **Priority:** P2 | **Depends on:** alert-notification service

---

### [TODO-037] Workspace-level read-only API key
**What:** New endpoint: `POST /api/v1/workspaces/{id}/api-keys` with `{"scope": "read", "name": "my-dashboard"}`. Returns a scoped key that can call `GET /v1/channels/{id}/data` but is rejected by the ingestion service. Store in `workspace_api_keys` table with `scope`, `name`, `created_at`, `last_used_at`. Add `DELETE /api/v1/workspaces/{id}/api-keys/{key_id}` for revocation.
**Why:** Device write keys cannot be shared with dashboard builders or integrations without granting write access. Read-only keys are table stakes for any third-party integration story.
**Effort:** S human / XS CC+gstack | **Priority:** P2 | **Depends on:** TODO-014 (workspace scoping)

---

### [TODO-038] Webhook payload signing (HMAC-SHA256)
**What:** On `POST /api/v1/channels/{id}/alerts`, accept optional `secret` field. Store hashed secret alongside webhook URL. On dispatch, compute `HMAC-SHA256(secret, body)` and emit as `X-GreenLab-Signature: sha256={hex}`. Document verification pattern in API docs. Add `POST /api/v1/channels/{id}/alerts/{id}/verify-signature` sandbox endpoint for testing.
**Why:** Without signature verification, webhook receivers have no way to confirm the request came from GreenLab. Any party that knows the endpoint URL can forge alerts.
**Effort:** S human / XS CC+gstack | **Priority:** P2 | **Depends on:** TODO-003 (SSRF fix first)

---

### [TODO-039] Promote device location to first-class DB columns
**What:** Add `lat NUMERIC`, `lng NUMERIC`, `location_address TEXT` as nullable columns to the `devices` table with a migration. Remove location data from the `metadata` JSONB blob. Update `DeviceRepository` and `toDeviceResponse` to map directly from struct fields — eliminating `json.Unmarshal` on every list response.
**Why:** Location is currently stored in `metadata` and unpacked via `json.Unmarshal` on every `toDeviceResponse` call, including list endpoints. With 100 devices per workspace that is 100 unmarshal operations per list request. First-class columns also enable spatial queries (nearest device, within-radius) without full-table JSON extraction.
**Effort:** S human / XS CC+gstack | **Priority:** P2 | **Depends on:** none

---

## Architecture Decisions Recorded (for context)

These decisions should not be revisited without explicit discussion:

| Decision | Choice | Rationale | Date |
|----------|--------|-----------|------|
| Ingest pipeline | Kafka-first (two-stage) | Enables replay, decouples InfluxDB downtime from ingest failure | 2026-03-16 |
| PostgreSQL isolation | Separate DB per service | Enforces P3 (each service owns its data), prevents schema collision | 2026-03-16 |
| API endpoint design | `/v1/channels/{id}/data` | RESTful, resource-centric, enables nginx per-channel routing | 2026-03-16 |
| Ingestion auth source | device-registry HTTP (not direct DB) | Enforces P3; enables separate DBs per service; ~2ms latency on cache miss | 2026-03-17 |
| Cache invalidation | Version counter in Redis (`device_version:{id}`) | Near-instant invalidation on rotation with one extra Redis GET; no cross-service HTTP on hot path | 2026-03-17 |
| Device creation API | `POST /provision` (atomic transaction) | Prevents orphaned device/channel on partial failure; mirrors ThingSpeak "create channel" UX | 2026-03-17 |
| Cascade on delete | Soft-delete channels with device | Avoids orphaned channels in workspace listings; consistent UX | 2026-03-17 |
| Compact format gating | Blocked on TODO-012 | field_index_map must come from validate-api-key response; no temporary workaround | 2026-03-21 |
| Field index never recycled | Permanent, sequential | Devices burn index map into firmware; recycling would corrupt data silently | 2026-03-21 |
| Binary frame DEVID | Validate against auth.device_id | Prevents direct object reference attack (authenticated device writing to different device's channel) | 2026-03-21 |
| Schema endpoint auth | Requires API key or JWT | Field names/types are sensitive; consistent with existing auth architecture | 2026-03-21 |
| td field for compact formats | uint16 ms offsets from batch ts | Preserves per-field timestamp parity with JSON API; batch must split on overflow >65535ms | 2026-03-21 |
| Body size cap | All formats capped (1MB JSON/msgpack, 64KB proto, 32B binary) | Prevents pre-parse memory exhaustion; Gin has no default body limit | 2026-03-21 |
| Replay rate limiting | 100 readings/min per device via Redis sliding window | Prevents burst from reconnecting Tier 1 devices saturating Kafka | 2026-03-21 |
| Deserializer interface | Shared steps 1/3/4/5; only raw byte parsing differs per format | DRY; forces all compact formats to produce canonical IngestInput | 2026-03-21 |
| Field Position vs Index | Keep Position (display order 1-8), add Index uint8 (compact key 1-255) | Plan assumed 1-255 range; domain enforced 1-8; resolved by adding separate field | 2026-03-21 |

---

## Frontend — from PR #38 review

### ~~[TODO-039] Fix QueryParams field_key vs field param name mismatch~~ ✅ DONE (2026-04-01)
Renamed `field_key` → `field` in `QueryParams` (types/index.ts) and updated `queryApi.latest()` signature. All call sites in `QueryPage.tsx` updated. `as any` casts removed.
