ALTER TABLE IF EXISTS connection_health_events ADD COLUMN IF NOT EXISTS manual boolean NOT NULL DEFAULT false;

DO $$
BEGIN
 IF to_regclass('connection_health_states') IS NOT NULL AND to_regclass('connection_health_events') IS NOT NULL THEN
WITH sources AS (
 SELECT s.connection_id, s.model_name, e.admin_group_id, e.own_group_name, e.upstream_group_name
 FROM connection_health_states s
 CROSS JOIN LATERAL (
  SELECT admin_group_id, own_group_name, upstream_group_name, to_state, created_at
  FROM connection_health_events e
  WHERE e.connection_id=s.connection_id AND e.model_name=s.model_name
   AND e.user_id=s.user_id AND e.admin_account_id=s.admin_account_id
   AND e.policy_id<>'' AND e.result IN ('ok','network_fluctuation','rate_limited','server_error','auth','model_not_found','invalid_response','unsupported')
  ORDER BY e.created_at DESC, e.id DESC LIMIT 1
 ) e
 WHERE (s.connection_id LIKE 'sub2api:%' OR s.connection_id LIKE 'newapi:%')
  AND e.created_at>=s.last_probe_at AND e.to_state=s.state
)
UPDATE connection_health_states s SET own_group_id=e.admin_group_id, upstream_group_id=e.admin_group_id,
 own_group_name=e.own_group_name, upstream_group_name=e.upstream_group_name
FROM sources e WHERE s.connection_id=e.connection_id AND s.model_name=e.model_name
 AND (s.own_group_id,s.upstream_group_id,s.own_group_name,s.upstream_group_name)
 IS DISTINCT FROM (e.admin_group_id,e.admin_group_id,e.own_group_name,e.upstream_group_name);
 END IF;
END $$;
