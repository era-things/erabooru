-- Cache revisions are committed atomically with the data they describe. Bump
-- at transaction end, after source/model writes have acquired their locks.
CREATE TABLE IF NOT EXISTS tag_review_state (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 revision bigint NOT NULL DEFAULT 1
);
INSERT INTO tag_review_state(singleton) VALUES(true) ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION invalidate_tag_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF current_setting('erabooru.tag_review_invalidated', true) IS DISTINCT FROM 'on' THEN
  UPDATE tag_review_state SET revision=revision+1 WHERE singleton;
  PERFORM set_config('erabooru.tag_review_invalidated', 'on', true);
 END IF;
 RETURN NULL;
END $$;

DO $$
DECLARE source_table text;
BEGIN
 FOREACH source_table IN ARRAY ARRAY['media','media_embedding','media_tag','media_negative_tag','tag','tag_statistics','tag_classifier'] LOOP
  IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgrelid=source_table::regclass AND tgname='tag_review_changed') THEN
   EXECUTE format('CREATE CONSTRAINT TRIGGER tag_review_changed AFTER INSERT OR UPDATE OR DELETE ON %I
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION invalidate_tag_review()', source_table);
  END IF;
 END LOOP;
END $$;
