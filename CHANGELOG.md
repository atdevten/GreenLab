# Changelog

All notable changes to this project will be documented in this file.

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
