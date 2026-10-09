-- Existing and new policies must explicitly opt in to automatic suspension.
ALTER TABLE IF EXISTS connection_health_policies
    ADD COLUMN IF NOT EXISTS auto_suspend_enabled boolean NOT NULL DEFAULT false;
