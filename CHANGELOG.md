# Changelog

All notable changes to this project will be documented in this file.

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
