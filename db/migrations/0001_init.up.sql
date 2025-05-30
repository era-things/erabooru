-- 1. media files --------------------------------------------
CREATE TABLE media (
    id          BIGSERIAL PRIMARY KEY,
    hash        BYTEA            UNIQUE NOT NULL,
    mime_type   TEXT                      NOT NULL,
    width       INT                       NOT NULL,
    height      INT                       NOT NULL,
    duration    REAL,
    created_at  TIMESTAMPTZ DEFAULT now()
);

-- 2. tags ----------------------------------------------------
CREATE TABLE tag (
    id   SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE media_tag (
    media_id BIGINT REFERENCES media(id) ON DELETE CASCADE,
    tag_id   INT    REFERENCES tag(id)   ON DELETE CASCADE,
    PRIMARY KEY (media_id, tag_id)
);

-- 3. speed up sort:date
CREATE INDEX media_created_at_idx ON media (created_at DESC);