ALTER TABLE IF EXISTS connection_health_target_action_states ADD COLUMN IF NOT EXISTS quality_suspended boolean NOT NULL DEFAULT false;
