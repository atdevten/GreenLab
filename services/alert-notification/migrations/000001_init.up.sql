-- Alert & Notification service — initial schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS alert_rules (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  channel_id   UUID NOT NULL,
  workspace_id UUID NOT NULL,
  name         TEXT NOT NULL,
  field_name   TEXT NOT NULL,
  condition    TEXT NOT NULL,
  threshold    DOUBLE PRECISION NOT NULL,
  severity     TEXT NOT NULL DEFAULT 'warning',
  message      TEXT NOT NULL DEFAULT '',
  enabled      BOOLEAN NOT NULL DEFAULT TRUE,
  cooldown_sec INTEGER NOT NULL DEFAULT 300,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notifications (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  workspace_id UUID NOT NULL,
  channel_type TEXT NOT NULL,
  recipient    TEXT NOT NULL DEFAULT '',
  subject      TEXT NOT NULL DEFAULT '',
  body         TEXT NOT NULL DEFAULT '',
  status       TEXT NOT NULL DEFAULT 'pending',
  retries      INTEGER NOT NULL DEFAULT 0,
  sent_at      TIMESTAMPTZ,
  error_msg    TEXT NOT NULL DEFAULT '',
  read         BOOLEAN NOT NULL DEFAULT FALSE,
  read_at      TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes covering all query patterns in the repo layer
CREATE INDEX ON alert_rules(channel_id, created_at);
CREATE INDEX ON alert_rules(workspace_id, created_at DESC);
CREATE INDEX ON alert_rules(enabled) WHERE enabled = TRUE;
CREATE INDEX ON notifications(workspace_id, created_at DESC);
