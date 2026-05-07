CREATE TABLE articles (
                          id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                          slug          TEXT NOT NULL UNIQUE,
                          title         TEXT NOT NULL,
                          body_md       TEXT NOT NULL,
                          source_lang   TEXT NOT NULL DEFAULT 'en',
                          published_at  TIMESTAMPTZ,
                          created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                          updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);