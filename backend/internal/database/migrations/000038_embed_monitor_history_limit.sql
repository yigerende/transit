ALTER TABLE embed_group_monitor_boards
 ADD COLUMN IF NOT EXISTS history_limit integer NOT NULL DEFAULT 60
 CHECK (history_limit BETWEEN 1 AND 1000);
