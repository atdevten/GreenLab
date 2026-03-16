-- Supporting service — initial schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS audit_events (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id     TEXT NOT NULL,
  user_id       TEXT NOT NULL DEFAULT '',
  event_type    TEXT NOT NULL,
  resource_id   TEXT NOT NULL DEFAULT '',
  resource_type TEXT NOT NULL DEFAULT '',
  ip_address    TEXT NOT NULL DEFAULT '',
  user_agent    TEXT NOT NULL DEFAULT '',
  payload       JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_streams (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  device_id     UUID NOT NULL,
  workspace_id  UUID NOT NULL,
  name          TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  protocol      TEXT NOT NULL,
  source_url    TEXT NOT NULL DEFAULT '',
  storage_key   TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'pending',
  thumbnail_url TEXT NOT NULL DEFAULT '',
  duration_sec  INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes covering all query patterns in the repo layer
CREATE INDEX ON audit_events(tenant_id, created_at DESC);
CREATE INDEX ON audit_events(resource_type, resource_id);
CREATE INDEX ON video_streams(device_id, created_at DESC);
CREATE INDEX ON video_streams(workspace_id, created_at DESC);
