CREATE TABLE IF NOT EXISTS rate_limit_rules (
    id BIGSERIAL PRIMARY KEY,
    service TEXT NOT NULL,
    phase TEXT NOT NULL,
    method TEXT NOT NULL,
    match_kind TEXT NOT NULL,
    path TEXT NOT NULL,
    path_pattern TEXT NOT NULL,
    app_key TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    key_by TEXT[] NOT NULL DEFAULT ARRAY['ip']::text[],
    strategy TEXT NOT NULL,
    window_seconds INTEGER NOT NULL,
    max_requests INTEGER NOT NULL,
    requests_per_second DOUBLE PRECISION,
    burst INTEGER,
    client_ttl_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rate_limit_rules_lookup ON rate_limit_rules (service, phase, method, match_kind, path, app_key);
CREATE INDEX idx_rate_limit_rules_pattern ON rate_limit_rules (service, phase, method, app_key);

-- Prevent duplicate rules for the same identity.
CREATE UNIQUE INDEX idx_rate_limit_rules_unique
    ON rate_limit_rules (service, phase, method, match_kind, path, COALESCE(app_key, ''));