-- Legacy policy-wide reservations remain for audit. Channel counters are seeded
-- from that channel's actual probe events on its first reservation after upgrade.
CREATE TABLE IF NOT EXISTS connection_health_channel_probe_budget_usage (
    user_id text NOT NULL,
    admin_account_id text NOT NULL DEFAULT '',
    policy_id text NOT NULL,
    target_id text NOT NULL,
    day_start timestamptz NOT NULL,
    used integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, admin_account_id, policy_id, target_id, day_start)
);
-- Fresh installs create the event table later through runtime EnsureSchema.
DO $$ BEGIN
    IF to_regclass('connection_health_events') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_connection_health_events_channel_budget
        ON connection_health_events (user_id, admin_account_id, policy_id, connection_id, created_at) INCLUDE (result);
    END IF;
END $$;
