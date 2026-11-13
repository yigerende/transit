-- Preserve existing publication while requiring explicit selection for new monitors.
-- Keep reruns from publishing monitors that the administrator has since hidden.
DO $$
BEGIN
 IF NOT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_schema=current_schema() AND table_name='embed_group_monitors' AND column_name='published'
 ) THEN
  ALTER TABLE embed_group_monitors ADD COLUMN published boolean NOT NULL DEFAULT false;
  UPDATE embed_group_monitors SET published=true;
 END IF;
END $$;
