CREATE TABLE IF NOT EXISTS connection_health_channel_quality_suspension (
 user_id text NOT NULL,
 admin_account_id text NOT NULL,
 target_id text NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY (user_id, admin_account_id, target_id)
);

INSERT INTO connection_health_channel_quality_suspension(user_id,admin_account_id,target_id,enabled)
 SELECT user_id,admin_account_id,target_id,true FROM connection_health_target_action_states WHERE quality_suspended
 ON CONFLICT(user_id,admin_account_id,target_id) DO NOTHING;
