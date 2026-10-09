CREATE TABLE IF NOT EXISTS connection_health_group_probe_configs (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    group_id text NOT NULL,
    model text NOT NULL,
    custom_key_id text NOT NULL DEFAULT '',
    interval_seconds integer NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 10 AND 86400),
    enabled boolean NOT NULL DEFAULT true,
    next_probe_at timestamptz,
    last_probe_at timestamptz,
    last_error_key text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, admin_account_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_connection_health_group_probe_due
    ON connection_health_group_probe_configs (next_probe_at) WHERE enabled;
