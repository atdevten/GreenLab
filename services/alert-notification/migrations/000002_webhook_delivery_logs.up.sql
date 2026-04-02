CREATE TABLE IF NOT EXISTS webhook_delivery_logs (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  rule_id       UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
  url           TEXT NOT NULL,
  http_status   INTEGER NOT NULL DEFAULT 0,
  latency_ms    INTEGER NOT NULL DEFAULT 0,
  response_body TEXT NOT NULL DEFAULT '',
  error_msg     TEXT NOT NULL DEFAULT '',
  delivered_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON webhook_delivery_logs(rule_id, delivered_at DESC);
