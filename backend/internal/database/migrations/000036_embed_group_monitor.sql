CREATE TABLE IF NOT EXISTS embed_group_monitor_boards (
 user_id text NOT NULL, admin_account_id text NOT NULL,
 title text NOT NULL DEFAULT '分组质量观测', enabled boolean NOT NULL DEFAULT false,
 token text NOT NULL UNIQUE, PRIMARY KEY(user_id,admin_account_id)
);
CREATE TABLE IF NOT EXISTS embed_group_monitors (
 id text PRIMARY KEY, user_id text NOT NULL, admin_account_id text NOT NULL,
 config jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_embed_group_monitor_workspace ON embed_group_monitors(user_id,admin_account_id);
CREATE TABLE IF NOT EXISTS embed_group_monitor_jobs (
 monitor_id text NOT NULL REFERENCES embed_group_monitors(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('health','candy','pelican')),
 enabled boolean NOT NULL DEFAULT false, interval_seconds integer NOT NULL CHECK(interval_seconds BETWEEN 10 AND 86400),
 requested boolean NOT NULL DEFAULT false, manual boolean NOT NULL DEFAULT false, next_run_at timestamptz NOT NULL DEFAULT now(),
 run_id text NOT NULL DEFAULT '', started_at timestamptz, running_until timestamptz,
 PRIMARY KEY(monitor_id,kind)
);
CREATE INDEX IF NOT EXISTS idx_embed_group_monitor_due ON embed_group_monitor_jobs(next_run_at);
CREATE TABLE IF NOT EXISTS embed_group_monitor_samples (
 id text PRIMARY KEY, monitor_id text NOT NULL REFERENCES embed_group_monitors(id) ON DELETE CASCADE,
 kind text NOT NULL, started_at timestamptz NOT NULL, sample jsonb NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_embed_group_monitor_history ON embed_group_monitor_samples(monitor_id,kind,started_at DESC,id DESC);
