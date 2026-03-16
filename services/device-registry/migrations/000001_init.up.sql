-- Device Registry service — initial schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS devices (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  workspace_id UUID NOT NULL,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  api_key      TEXT NOT NULL UNIQUE DEFAULT '',
  status       TEXT NOT NULL DEFAULT 'active',
  last_seen_at TIMESTAMPTZ,
  metadata     JSONB,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS channels (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  workspace_id UUID NOT NULL,
  device_id    UUID REFERENCES devices(id) ON DELETE SET NULL,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  visibility   TEXT NOT NULL DEFAULT 'private',
  tags         JSONB NOT NULL DEFAULT '[]',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fields (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  channel_id  UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  label       TEXT NOT NULL DEFAULT '',
  unit        TEXT NOT NULL DEFAULT '',
  field_type  TEXT NOT NULL DEFAULT 'float',
  position    INTEGER NOT NULL DEFAULT 1,
  description TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id, position)
);

-- Indexes covering all query patterns in the repo layer
CREATE INDEX ON devices(workspace_id, created_at DESC);
CREATE INDEX ON channels(workspace_id, created_at DESC);
CREATE INDEX ON channels(device_id);
CREATE INDEX ON fields(channel_id, position);
