# API Reference

## Base URL Conventions

Each service exposes its own HTTP server. In local development, services are accessed directly by port. In production, all traffic goes through the nginx reverse proxy on port 8080.

| Environment | Pattern |
| --- | --- |
| Local (direct) | `http://localhost:<port>/api/v1` |
| Local (via nginx) | `http://localhost:9080/api/v1` |
| Production | `https://api.greenlab.io/v1` |

All request and response bodies are `application/json` unless noted.

---

## Authentication Methods

| Level | Who | How |
| --- | --- | --- |
| `PUBLIC` | Anyone | No header required |
| `DEVICE` | IoT device | `X-API-Key: ts_<64hex>` header |
| `USER` | Logged-in human | `Authorization: Bearer <jwt>` header |
| `USER_OPTIONAL` | Logged-in or anonymous | JWT as `?token=<jwt>` query param or Bearer header |

---

## Standard Error Response

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "email is required",
    "details": {}
  }
}
```

| HTTP Code | Meaning |
| --- | --- |
| 400 | Validation error / bad request |
| 401 | Missing or invalid credentials |
| 403 | Authenticated but not authorised |
| 404 | Resource not found |
| 409 | Conflict (duplicate resource) |
| 422 | Unprocessable entity |
| 500 | Internal server error |

---

## Pagination

All list endpoints support cursor-based pagination.

**Request query params:**

| Param | Type | Description |
| --- | --- | --- |
| `cursor` | string | Opaque cursor from previous response |
| `limit` | int | Items per page (default 20, max 100) |

**Response envelope:**

```json
{
  "data": [ ... ],
  "pagination": {
    "next_cursor": "eyJpZCI6Ii4uLiJ9",
    "has_more": true
  }
}
```

---

## iam service — port 8001

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | PUBLIC | Liveness check, returns `{"status":"ok"}` |

### Auth

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/signup` | PUBLIC | Self-register a new account (creates user + org + workspace, returns tokens) |
| `POST` | `/api/v1/auth/register` | PUBLIC | Register a new user within an existing tenant |
| `POST` | `/api/v1/auth/login` | PUBLIC | Authenticate and receive JWT + refresh token |
| `POST` | `/api/v1/auth/refresh` | PUBLIC | Exchange refresh token for new JWT |
| `POST` | `/api/v1/auth/forgot-password` | PUBLIC | Trigger password reset email |
| `POST` | `/api/v1/auth/reset-password` | PUBLIC | Reset password using token from email |
| `POST` | `/api/v1/auth/verify-email` | PUBLIC | Verify email address using token |
| `POST` | `/api/v1/auth/logout` | USER | Invalidate current session |
| `GET` | `/api/v1/auth/me` | USER | Get current user profile |
| `PUT` | `/api/v1/auth/me` | USER | Update current user profile |
| `PUT` | `/api/v1/auth/me/password` | USER | Change password |

#### POST /api/v1/auth/signup

Self-registration: creates the user, a new org, and a default workspace in one call. Returns tokens immediately.

```json
{
  "email": "alice@example.com",
  "password": "s3cur3!"
}
```

#### POST /api/v1/auth/register

Register a new user within an existing tenant (requires `tenant_id`):

```json
{
  "tenant_id": "uuid",
  "email": "alice@example.com",
  "password": "s3cur3!",
  "first_name": "Alice",
  "last_name": "Smith"
}
```

#### POST /api/v1/auth/login — Response

```json
{
  "access_token": "<jwt>",
  "refresh_token": "<opaque>",
  "expires_in": 3600
}
```

### Organisations & Workspaces

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/orgs` | USER | Create a new organisation |
| `GET` | `/api/v1/orgs` | USER | List organisations for current user |
| `GET` | `/api/v1/orgs/:id` | USER | Get organisation by ID |
| `PUT` | `/api/v1/orgs/:id` | USER | Update organisation |
| `DELETE` | `/api/v1/orgs/:id` | USER | Delete organisation |
| `GET` | `/api/v1/orgs/:orgID/workspaces` | USER | List workspaces in organisation |
| `POST` | `/api/v1/workspaces` | USER | Create a workspace within an org |
| `PUT` | `/api/v1/workspaces/:id` | USER | Update workspace name/slug/description |
| `DELETE` | `/api/v1/workspaces/:id` | USER | Delete a workspace |

#### PUT /api/v1/workspaces/:id

```json
{
  "name": "Production Farm",
  "slug": "production-farm",
  "description": "Main production environment"
}
```

### Workspace Members

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/workspaces/:id/members` | USER | List members of a workspace |
| `POST` | `/api/v1/workspaces/:id/members` | USER | Invite a user to a workspace by email |
| `PUT` | `/api/v1/workspaces/:id/members/:userId` | USER | Update a member's role |
| `DELETE` | `/api/v1/workspaces/:id/members/:userId` | USER | Remove a member from a workspace |

Valid roles: `owner` · `admin` · `member` · `viewer`

#### POST /api/v1/workspaces/:id/members

```json
{ "email": "bob@example.com", "role": "member" }
```

#### PUT /api/v1/workspaces/:id/members/:userId

```json
{ "role": "admin" }
```

### API Keys

Org-level personal access tokens for scripting and integrations. Distinct from device API keys.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/api-keys` | USER | List API keys for the current tenant |
| `POST` | `/api/v1/api-keys` | USER | Create a new API key (plain key returned once) |
| `DELETE` | `/api/v1/api-keys/:id` | USER | Revoke an API key |

#### POST /api/v1/api-keys

```json
{ "name": "CI Deploy Key", "scopes": ["read:devices", "write:ingest"] }
```

Response includes a `key` field — only returned on creation:

```json
{
  "id": "uuid",
  "name": "CI Deploy Key",
  "key": "ts_<64 hex chars>",
  "key_prefix": "ts_abc123",
  "scopes": ["read:devices", "write:ingest"],
  "created_at": "2026-03-15T10:00:00Z"
}
```

---

## device-registry service — port 8002

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | PUBLIC | Liveness check |

### Devices

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/devices` | USER | Create a device (generates API key automatically) |
| `GET` | `/api/v1/devices` | USER | List devices (`?workspace_id=`) |
| `GET` | `/api/v1/devices/:id` | USER | Get device by ID |
| `PUT` | `/api/v1/devices/:id` | USER | Update device name/description/status |
| `DELETE` | `/api/v1/devices/:id` | USER | Delete device |
| `POST` | `/api/v1/devices/:id/rotate-key` | USER | Rotate API key (old key immediately invalidated) |
| `GET` | `/api/v1/workspaces/:id/devices` | USER | List all devices scoped to a workspace |

#### POST /api/v1/devices

Creates a device and atomically provisions a default channel. Returns both.

```json
{
  "workspace_id": "uuid",
  "name": "Weather Station A",
  "description": "Rooftop sensor",
  "lat": 10.7769,
  "lng": 106.7009,
  "location_address": "Greenhouse A, Building 3",
  "channel_name": "Temperature Readings",
  "channel_visibility": "private"
}
```

`lat`/`lng` must both be provided or both omitted. Valid ranges: lat `[-90, 90]`, lng `[-180, 180]`. `channel_visibility` must be `public` or `private` (defaults to `private`).

Response:

```json
{
  "device": { "id": "uuid", "api_key": "ts_...", "lat": 10.7769, "lng": 106.7009, "location_address": "Greenhouse A", ... },
  "channel": { "id": "uuid", "short_id": 1, "name": "Temperature Readings", ... }
}
```

#### POST /api/v1/devices/:id/rotate-key — Response

```json
{
  "api_key": "ts_<new 64 hex chars>"
}
```

### Channels

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/channels` | USER | Create a channel |
| `GET` | `/api/v1/channels` | USER | List channels (`?workspace_id=` or `?device_id=`) |
| `GET` | `/api/v1/channels/:id` | USER | Get channel by ID |
| `PUT` | `/api/v1/channels/:id` | USER | Update channel |
| `DELETE` | `/api/v1/channels/:id` | USER | Delete channel |

#### POST /api/v1/channels

```json
{
  "workspace_id": "uuid",
  "device_id": "uuid",
  "name": "Temperature Readings",
  "description": "Celsius values from rooftop sensors",
  "visibility": "private",
  "tags": ["outdoor", "temperature"]
}
```

### Fields

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/fields` | USER | Create a field within a channel |
| `GET` | `/api/v1/fields` | USER | List fields (filter by `?channel_id=`) |
| `GET` | `/api/v1/fields/:id` | USER | Get field by ID |
| `PUT` | `/api/v1/fields/:id` | USER | Update field |
| `DELETE` | `/api/v1/fields/:id` | USER | Delete field |

#### POST /api/v1/fields

```json
{
  "channel_id": "uuid",
  "name": "temperature",
  "label": "Temperature",
  "unit": "°C",
  "field_type": "float",
  "position": 1
}
```

---

## ingestion service — port 8003

> **Note:** ingestion uses the path prefix `/v1` (not `/api/v1`) to align with the channel-centric resource URL design.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | PUBLIC | Liveness check |

### Ingest

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/v1/channels/:channel_id/data` | DEVICE | Ingest a single reading |
| `POST` | `/v1/channels/:channel_id/data/bulk` | DEVICE | Ingest a batch of readings (JSON only) |

**Authentication:** pass the API key in the `X-API-Key` header or as `?api_key=<key>` query param. The `channel_id` in the path must match the channel bound to the key — mismatches return `403 device_id_mismatch`.

**Rate limit:** 100 requests per minute per API key. Exceeding the limit returns `429`.

**Publish failures** (Kafka unavailable) return `503 service_unavailable`.

### Content-Type formats

The single endpoint (`/data`) supports multiple wire formats selected by `Content-Type`. Choose based on your device's constraints:

| `Content-Type` | Format | Max body | Best for |
| --- | --- | --- | --- |
| `application/json` *(default)* | Standard JSON — named fields | 1 MB | Development, servers |
| `application/x-greenlab-ojson` | Optimised JSON — positional field array | 1 MB | Moderate bandwidth savings |
| `application/msgpack` | MessagePack binary — same schema as OJson | 1 MB | Low-power devices with msgpack support |
| `application/x-thingspeak-binary` | Fixed binary frame with CRC16 integrity check | 32 B | Ultra-constrained microcontrollers |
| `application/x-protobuf` | Protocol Buffers | — | *Coming soon* |

The bulk endpoint (`/data/bulk`) accepts `application/json` only.

---

#### POST /v1/channels/:channel_id/data — JSON

Standard format. Field values must be `float64`. `channel_id` is taken from the URL — **do not include it in the body**.

```json
{
  "fields": {
    "temperature": 23.5,
    "humidity": 61.2
  },
  "field_timestamps": {
    "temperature": "2026-03-10T14:00:00.000Z",
    "humidity":    "2026-03-10T14:00:00.180Z"
  },
  "tags": {
    "location": "rooftop"
  },
  "timestamp": "2026-03-10T14:00:00Z",
  "data": { "raw_adc": 1023 }
}
```

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `fields` | `object<string, float64>` | yes | Named field values |
| `field_timestamps` | `object<string, RFC3339>` | no | Per-field timestamps; overrides `timestamp` for that field |
| `tags` | `object<string, string>` | no | Metadata labels (not stored in time-series) |
| `timestamp` | RFC3339 string | no | Reading time; defaults to server receive time |
| `data` | any JSON | no | Opaque pass-through blob; stored but not processed |

Response (`201 Created`):

```json
{
  "success": true,
  "data": {
    "accepted": 1,
    "written_at": "2026-03-10T14:00:01Z"
  }
}
```

---

#### POST /v1/channels/:channel_id/data — OJson (`application/x-greenlab-ojson`)

Compact JSON using positional field indexing. Fields map to the channel's field schema by 1-based index (field 1, 2, 3…).

```json
{ "f": [23.5, 61.2], "t": 1741426620000, "td": [0, 180], "sv": 1 }
```

| Key | Type | Required | Description |
| --- | --- | --- | --- |
| `f` | `float64[]` | yes | Field values in schema index order (index 1, 2, …) |
| `t` | `int64` | no | Unix timestamp in **milliseconds**; defaults to server receive time |
| `td` | `uint16[]` | no | Per-field timestamp deltas in **milliseconds** from `t`; must have same length as `f` |
| `sv` | `uint32` | no | Schema version; validated against the channel's current schema version |

---

#### POST /v1/channels/:channel_id/data — MsgPack (`application/msgpack`)

Identical schema to OJson, encoded as MessagePack. Keys are the same compact short names: `f`, `t`, `td`, `sv`.

---

#### POST /v1/channels/:channel_id/data — Binary (`application/x-thingspeak-binary`)

Fixed-length binary frame for ultra-constrained devices. Max 32 bytes.

```
Byte offset  Length  Field
──────────────────────────────────────────────────────────
0            1       VER — schema version (uint8)
1–4          4       DEVID — first 4 bytes of device UUID (big-endian)
5–8          4       TS — unix timestamp in seconds (uint32 big-endian)
9            1       FIELDMSK — bitmask; bit i set → field index i+1 present
10…10+N×2    N×2     VALUES — N × uint16 big-endian (N = popcount(FIELDMSK))
10+N×2…end   2       CRC16/CCITT-FALSE over all preceding bytes
```

- Frame length: `12 + N×2` bytes where N = number of set bits in FIELDMSK (max 8 fields → 28 bytes)
- DEVID must match the first 4 bytes of the authenticated device UUID; mismatch returns `403`
- CRC mismatch returns `400 crc_mismatch`
- Field values are raw `uint16`; no scale/offset applied (scale support planned)

---

#### POST /v1/channels/:channel_id/data/bulk

Ingest multiple readings in a single request. JSON only.

```json
{
  "readings": [
    {
      "fields": { "temperature": 23.1 },
      "timestamp": "2026-03-10T13:59:00Z"
    },
    {
      "fields": { "temperature": 23.5, "humidity": 61.2 },
      "field_timestamps": { "humidity": "2026-03-10T14:00:00.180Z" },
      "timestamp": "2026-03-10T14:00:00Z"
    }
  ]
}
```

Each entry in `readings` accepts the same fields as the single-reading JSON body. Response includes total accepted count:

```json
{
  "success": true,
  "data": {
    "accepted": 2,
    "written_at": "2026-03-10T14:00:01Z"
  }
}
```

### Ingestion Error Codes

| HTTP | Code | Cause |
| --- | --- | --- |
| 400 | `validation_error` | Missing `fields`, invalid JSON |
| 400 | `crc_mismatch` | Binary frame CRC check failed |
| 400 | `invalid_frame_length` | Binary frame wrong length for FIELDMSK |
| 400 | `ts_delta_invalid` | `td` length doesn't match `f` length |
| 400 | `timestamp_too_old` / `timestamp_future` | Timestamp outside acceptable window |
| 403 | `device_id_mismatch` | Binary DEVID doesn't match authenticated device |
| 409 | `schema_version_mismatch` | `sv` doesn't match current channel schema version |
| 410 | `schema_force_deprecated` | Channel schema has been force-deprecated; device must fetch the latest schema and update firmware |
| 413 | `payload_too_large` | Body exceeds format limit |
| 415 | `unsupported_media_type` | Unknown `Content-Type` |
| 503 | `service_unavailable` | Kafka publish failed |

---

## query-realtime service — port 8004

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | PUBLIC | Liveness check |

### Historical Query

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/query` | USER | Query time-series data with aggregation |
| `GET` | `/api/v1/query/latest` | USER | Get the most recent reading for a channel/field |

#### GET /api/v1/query — Query Parameters

| Param | Required | Description |
| --- | --- | --- |
| `channel_id` | yes | Channel UUID |
| `field` | yes | Field name (e.g. `temperature`) |
| `start` | yes | RFC3339 start time |
| `end` | no | RFC3339 end time (default: now) |
| `aggregation` | no | `mean`, `max`, `min`, `sum`, `count` (default: `mean`) |
| `window` | no | Duration string e.g. `1m`, `5m`, `1h` (default: `5m`) |
| `format` | no | Set to `csv` to download results as a CSV file |

When `format=csv`, the response is `Content-Type: text/csv` with `Content-Disposition: attachment; filename="query-export.csv"`.

#### GET /api/v1/query/latest — Query Parameters

| Param | Required | Description |
| --- | --- | --- |
| `channel_id` | yes | Channel UUID |
| `field` | yes | Field name |

### Realtime Push

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/ws` | USER | WebSocket upgrade; receive readings in real-time |
| `GET` | `/api/v1/sse` | USER | Server-Sent Events stream of readings |
| `GET` | `/api/v1/stats` | USER | Hub statistics (connected clients, message rate) |

WebSocket and SSE connections accept the JWT as a `?token=<jwt>` query parameter (for clients that cannot set `Authorization` headers on upgrade requests). A valid JWT is required.

Subscribe to a channel by sending a JSON message after connecting:

```json
{ "action": "subscribe", "channel_id": "uuid" }
```

Incoming reading events:

```json
{
  "channel_id": "uuid",
  "device_id": "uuid",
  "fields": { "temperature": 23.5 },
  "tags": { "location": "rooftop" },
  "timestamp": "2026-03-10T14:00:00Z"
}
```

---

## alert-notification service — port 8005

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | PUBLIC | Liveness check |

### Alert Rules

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/alert-rules` | USER | Create an alert rule |
| `GET` | `/api/v1/alert-rules` | USER | List alert rules |
| `GET` | `/api/v1/alert-rules/:id` | USER | Get alert rule by ID |
| `PUT` | `/api/v1/alert-rules/:id` | USER | Update alert rule (set `enabled: false` to disable) |
| `DELETE` | `/api/v1/alert-rules/:id` | USER | Delete alert rule |

#### POST /api/v1/alert-rules

```json
{
  "channel_id": "uuid",
  "workspace_id": "uuid",
  "name": "High Temperature",
  "field_name": "temperature",
  "condition": "gt",
  "threshold": 40.0,
  "severity": "critical",
  "message": "Temperature exceeded 40°C",
  "cooldown_sec": 300
}
```

Conditions: `gt` · `gte` · `lt` · `lte` · `eq` · `neq`
Severities: `info` · `warning` · `critical`

### Notifications

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/notifications` | USER | Manually send a notification |
| `GET` | `/api/v1/notifications` | USER | List notifications for workspace |
| `GET` | `/api/v1/notifications/:id` | USER | Get notification by ID |
| `PATCH` | `/api/v1/notifications/:id/read` | USER | Mark a notification as read |
| `POST` | `/api/v1/notifications/read-all` | USER | Mark all notifications as read |

---

## supporting service — port 8007

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | PUBLIC | Liveness check |

### Video Streams

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/streams` | USER | Create a stream record |
| `GET` | `/api/v1/streams` | USER | List streams in workspace |
| `GET` | `/api/v1/streams/:id` | USER | Get stream by ID |
| `PATCH` | `/api/v1/streams/:id/status` | USER | Update stream status |
| `GET` | `/api/v1/streams/:id/upload-url` | USER | Get S3 presigned upload URL |
| `GET` | `/api/v1/streams/:id/download-url` | USER | Get S3 presigned download URL |
| `DELETE` | `/api/v1/streams/:id` | USER | Delete stream record |

### Audit Events

Audit events are written automatically via Kafka. The API is read-only.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/audit/events` | USER | List audit events for current tenant |
| `GET` | `/api/v1/audit/events/resource` | USER | List events filtered by resource type/ID |
| `GET` | `/api/v1/audit/events/:id` | USER | Get a specific audit event |

#### GET /api/v1/audit/events — Query Parameters

| Param | Required | Description |
| --- | --- | --- |
| `resource_type` | no | Filter by resource type (e.g. `device`, `channel`, `user`) |
| `search` | no | Case-insensitive search across user name, action, and target |
| `format` | no | Set to `csv` to download as `audit-log.csv` |
