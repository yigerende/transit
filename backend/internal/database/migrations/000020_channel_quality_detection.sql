CREATE TABLE IF NOT EXISTS connection_health_quality_settings (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    revision text NOT NULL,
    config jsonb NOT NULL,
    PRIMARY KEY (user_id, admin_account_id)
);
CREATE TABLE IF NOT EXISTS connection_health_quality_groups (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    group_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, admin_account_id, group_id)
);
CREATE TABLE IF NOT EXISTS connection_health_quality_states (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    target_id text NOT NULL,
    revision text NOT NULL,
    state jsonb NOT NULL,
    PRIMARY KEY (user_id, admin_account_id, target_id)
);
CREATE TABLE IF NOT EXISTS connection_health_quality_history (
    id text PRIMARY KEY,
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    target_id text NOT NULL,
    revision text NOT NULL,
    sample jsonb NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_quality_history_target ON connection_health_quality_history (user_id, admin_account_id, target_id, created_at DESC, id DESC);
