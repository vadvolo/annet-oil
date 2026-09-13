-- Audit event store. Applied idempotently on recorder startup.
CREATE TABLE IF NOT EXISTS audit_events (
    id            BIGSERIAL PRIMARY KEY,
    timestamp     TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor         TEXT        NOT NULL,
    actor_role    TEXT,
    source        TEXT        NOT NULL,
    action        TEXT        NOT NULL,
    devices       TEXT[],
    command       TEXT,
    params        JSONB,
    success       BOOLEAN     NOT NULL,
    duration_ms   BIGINT,
    error_type    TEXT,
    error_message TEXT,
    request_id    TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events (timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor     ON audit_events (actor);
CREATE INDEX IF NOT EXISTS idx_audit_events_action    ON audit_events (action);
CREATE INDEX IF NOT EXISTS idx_audit_events_source    ON audit_events (source);
