-- Existing policies also receive the new 20-second default.
ALTER TABLE IF EXISTS connection_health_policies
    ADD COLUMN IF NOT EXISTS max_latency_ms integer NOT NULL DEFAULT 20000;
