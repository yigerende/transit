ALTER TABLE IF EXISTS connection_health_quality_channels
    ADD COLUMN IF NOT EXISTS independent boolean NOT NULL DEFAULT false;
