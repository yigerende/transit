CREATE TABLE IF NOT EXISTS connection_health_quality_channels (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    target_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (user_id, admin_account_id, target_id)
);
