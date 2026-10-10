ALTER TABLE IF EXISTS connection_health_events
    ADD COLUMN IF NOT EXISTS probe_mode text NOT NULL DEFAULT 'real_model';
