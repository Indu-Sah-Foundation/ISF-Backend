-- Projects: current + upcoming work shown on /events.
-- Body is stored as a JSONB array of typed "blocks" so admins can build
-- rich pages (paragraphs, bullet lists, sub-sections) without us shipping
-- schema changes every time the layout changes.
--
-- Example blocks payload:
--   [
--     {"type":"paragraph","heading":"Concept","body":"…"},
--     {"type":"bullets","heading":"Services","items":["…","…"]},
--     {"type":"subsections","items":[
--        {"heading":"A · General Health Camp","items":["…","…"]}
--     ]}
--   ]
CREATE TABLE projects (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          TEXT NOT NULL UNIQUE,
    kind          TEXT NOT NULL CHECK (kind IN ('current','upcoming')),
    title         TEXT NOT NULL,
    label         TEXT NOT NULL DEFAULT '',         -- e.g. "Current Project" / numeral "01"
    lede          TEXT NOT NULL DEFAULT '',         -- left-column intro paragraph
    image_url     TEXT NOT NULL DEFAULT '',         -- blob URL (or empty)
    image_variant TEXT NOT NULL DEFAULT 'default',  -- 'default' | 'alt'
    blocks        JSONB NOT NULL DEFAULT '[]'::jsonb,
    position      INTEGER NOT NULL DEFAULT 0,        -- sort order within kind
    published     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX projects_kind_position_idx ON projects (kind, position);
