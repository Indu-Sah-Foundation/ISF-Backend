-- Volunteers: unified table for field-team members (people with bios + photos)
-- and the simpler text-only volunteer/research field categories shown on the
-- right side of the /volunteers page.
--
-- kind:
--   team             -> field-team members. name/bio/image_url are used.
--   volunteer_field  -> "Oral health check-ups", etc. Only `name` (label) used.
--   research_field   -> "Research on Oral Health Status", etc. Only `name` used.
CREATE TABLE volunteers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        TEXT NOT NULL CHECK (kind IN ('team','volunteer_field','research_field')),
    name        TEXT NOT NULL,
    bio         TEXT NOT NULL DEFAULT '',
    image_url   TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL DEFAULT 0,
    published   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX volunteers_kind_position_idx ON volunteers (kind, position);
