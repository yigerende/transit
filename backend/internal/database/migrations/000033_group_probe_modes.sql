ALTER TABLE connection_health_group_probe_configs
 ADD COLUMN IF NOT EXISTS probe_mode text NOT NULL DEFAULT 'real_model';
