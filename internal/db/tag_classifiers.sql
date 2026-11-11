-- Each historical centroid belongs to the unstamped legacy cohort until its
-- embedding provenance is supplied. New writes always carry their own profile.
DO $$
BEGIN
 IF NOT EXISTS (SELECT FROM pg_attribute WHERE attrelid='tag_statistics'::regclass
                AND attname='profile' AND NOT attisdropped) THEN
  ALTER TABLE tag_statistics ADD COLUMN profile text NOT NULL DEFAULT '';
  ALTER TABLE tag_statistics DROP CONSTRAINT tag_statistics_pkey;
  ALTER TABLE tag_statistics ADD PRIMARY KEY(tag_id,space_id,profile);
 END IF;
END $$;

CREATE TABLE IF NOT EXISTS tag_classifier (
 tag_id bigint NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
 space_id text NOT NULL REFERENCES embedding_space(id),
 profile text NOT NULL CHECK(profile <> ''),
 revision bigint NOT NULL DEFAULT 1,
 processed bigint NOT NULL DEFAULT 0,
 rebuild_target bigint NOT NULL DEFAULT 0,
 due_at timestamptz NOT NULL DEFAULT now(),
 last_error text,
 weights vector(768),
 bias double precision,
 model_version int,
 trained_at timestamptz,
 pool_refreshed_at timestamptz,
 positives int NOT NULL DEFAULT 0,
 rejections int NOT NULL DEFAULT 0,
 weak int NOT NULL DEFAULT 0,
 PRIMARY KEY(tag_id,space_id,profile)
);
CREATE INDEX IF NOT EXISTS tag_classifier_due ON tag_classifier(due_at) WHERE processed < revision;
CREATE INDEX IF NOT EXISTS media_embedding_sample ON media_embedding(space_id,profile,media_id)
 WHERE status='ready' AND value IS NOT NULL;
ALTER TABLE tag_statistics_state ADD COLUMN IF NOT EXISTS classifier_rebuild_id bigint;
ALTER TABLE tag_statistics_state ADD COLUMN IF NOT EXISTS classifier_rebuilt_at timestamptz;
ALTER TABLE tag_statistics_state ADD COLUMN IF NOT EXISTS pool_refreshed_at timestamptz NOT NULL DEFAULT now();

-- Coalesce ready-vector changes even for media whose only tag is tagme.
CREATE TABLE IF NOT EXISTS classifier_pool_dirty (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton)
);
CREATE OR REPLACE FUNCTION mark_classifier_pool_dirty() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP <> 'INSERT' AND OLD.space_id='vision' AND OLD.status='ready' AND OLD.profile<>'' THEN
  INSERT INTO classifier_pool_dirty DEFAULT VALUES ON CONFLICT DO NOTHING;
 ELSIF TG_OP <> 'DELETE' AND NEW.space_id='vision' AND NEW.status='ready' AND NEW.profile<>'' THEN
  INSERT INTO classifier_pool_dirty DEFAULT VALUES ON CONFLICT DO NOTHING;
 END IF;
 RETURN NULL;
END $$;
CREATE OR REPLACE TRIGGER classifier_pool_changed AFTER INSERT OR DELETE OR UPDATE OF status,value,profile
 ON media_embedding FOR EACH ROW EXECUTE FUNCTION mark_classifier_pool_dirty();
