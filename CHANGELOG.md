# Changelog

All notable changes to this project will be documented in this file.

## [0.1.1.0] - 2026-04-12

### Added

- **Schema force-deprecation enforcement**: compact-format ingestion requests from devices using an old schema version now receive `410 Gone` instead of `409 Conflict` when an operator has called `POST /api/v1/channels/{id}/schema/force-deprecate`. Devices already on the current schema version are unaffected. The 410 response tells device firmware to stop retrying and fetch the latest schema.
- **Stuck-device tracking**: when a device receives a 410, its device ID is recorded in `schema_stuck:{channel_id}:{device_id}` (48-hour TTL, write-once to avoid retry storms). This key can be used for ops dashboards and alerting to identify devices that have not migrated.

### Changed

- `SchemaACKStore` (ingestion) now exposes `IsForceDeprecated` and `SetStuck` methods, reading the `schema_force_deprecated:{channel_id}` key written by the device-registry service.
- `ForceDeprecateSchema` response note updated to present tense: compact-format requests *are rejected* with 410 Gone (not "will receive").

## [0.1.0.0] - 2026-04-11

### Changed

- **Channel IDs are now UUIDs**: the ingestion API requires channel IDs to be valid UUID strings (e.g. `550e8400-e29b-41d4-a716-446655440001`). Plain integers are no longer accepted. This aligns channel identification with the device-registry's UUID primary keys.
- **UUID channel lookup in device validation**: the internal API key validation query now uses a typed UUID comparison (`c.id = $2::uuid`) instead of a text cast, preserving index usage on large tables.
- **Sidebar badges removed**: the Workspaces, Devices, and Alert Rules sidebar items no longer show hardcoded placeholder counts. Dynamic badge counts will be added when the data source is wired up.

### Fixed

- Nil UUID (`00000000-0000-0000-0000-000000000000`) is now explicitly rejected by channel ID validation.

## [0.0.3.0] - 2026-04-08

### Added

- **Device Location**: devices can now be registered with GPS coordinates (lat/lng) and a location label (e.g. "Greenhouse A"). Location is stored in device metadata and surfaced in API responses as `lat`, `lng`, and `location_address`.
- **Atomic channel configuration**: channel name and visibility are now set in the same `POST /api/v1/devices` request that creates the device — eliminating a separate update call and saving a round-trip during registration.
- **Frontend test suite**: Vitest + @testing-library/react bootstrapped with initial tests for the RegisterDeviceDrawer component.

### Changed

- **Device registration flow**: backend now accepts `channel_name` and `channel_visibility` in `CreateDeviceRequest` and applies them to the auto-created channel atomically. Frontend no longer issues a separate `channelsApi.update()` after device creation.
- **API types**: `CreateDeviceResponse` now returns `{ device, channel }` instead of just the device, giving callers the channel ID without a follow-up fetch.

### Fixed

- **Code quality**: extracted shared `LocationMetadata` type (was duplicated as anonymous structs in the service and handler layers). Frontend `lat`/`lng` stored as `number` in local state instead of round-tripping through `string`.
- **Validation**: lat/lng values are now range-validated (`[-90,90]` / `[-180,180]`) and partial coordinate pairs (lat without lng, or vice versa) are rejected. `channel_visibility` must be `public` or `private`.

## [0.0.2.2] - 2026-04-02

### Changed

- **Live Data**: WebSocket connections are now multiplexed — one persistent connection per page instead of one per channel. Switching channels sends subscribe/unsubscribe messages over the existing socket, eliminating reconnect overhead and reducing server-side connection count.

## [0.0.2.1] - 2026-04-01

### Fixed

- **Dashboard**: device and channel lists are now scoped to the active workspace — previously fetched all records across all workspaces, causing data leakage between orgs

## [0.0.2.0] - 2026-04-01

### Changed

- **Ingestion API**: `POST /v1/channels/{id}/data` and bulk variant now return `channel_id` and `request_id` in the 201 response, enabling device firmware to correlate server-side logs without guesswork

## [0.0.1.0] - 2026-04-01

### Fixed

- **Dashboard**: stat cards (Active Devices, Readings 24h, Active Alerts, Channels) now show real data fetched from `/api/v1/devices`, `/api/v1/channels`, and `/api/v1/notifications` — previously they always displayed `—` due to a backend field mismatch with `/api/v1/stats`
- **Dashboard**: Recent Alert Events, Top Channels by Volume, and Device Map all wired to real API data instead of hardcoded demo values
- **Dashboard**: Recent Alert Events now filters to critical/warning notifications only (matches section title)
- **Dashboard**: `timeAgo()` helper guards against malformed dates to prevent `NaN min ago` display
- **Query Tab**: wired to real backend APIs (`/api/v1/query`, `/api/v1/query/latest`) with proper loading and error states
- **Query Tab**: fixed race condition where field select could be enabled before device/channel loaded
- **Query Tab**: fixed disabled guard for field select so it stays disabled until a channel is selected
- **DevicesPage**: removed `as any` cast on query response, using typed response correctly
- **API**: renamed `field_key` to `field` in `queryApi.latest()` params to match backend contract
