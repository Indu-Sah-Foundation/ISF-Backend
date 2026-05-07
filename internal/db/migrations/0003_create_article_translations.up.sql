CREATE TABLE article_translations (
                                      article_id     UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
                                      lang           TEXT NOT NULL,
                                      title          TEXT NOT NULL,
                                      body_md        TEXT NOT NULL,
                                      translated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                      PRIMARY KEY (article_id, lang)
);