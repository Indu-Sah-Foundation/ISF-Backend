-- Team: people listed on the About page. One unified table with a `kind`
-- enum so admins manage all four sections from one place.
--
-- kind:
--   founder         -> Lal Sah, Dr. Vijay Sah, Shubham Sah (PersonCard layout, full bio)
--   advisor_intl    -> International advisory board
--   advisor_nat     -> National advisory board
--   board           -> Board members (MediaCard layout, name + role only)
CREATE TABLE team (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        TEXT NOT NULL CHECK (kind IN ('founder','advisor_intl','advisor_nat','board')),
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT '',         -- "Co-Founder · Lead Software Engineer"
    extras      TEXT NOT NULL DEFAULT '',         -- credentials line under role
    bio         TEXT NOT NULL DEFAULT '',
    motto       TEXT NOT NULL DEFAULT '',         -- optional italic pull-quote
    image_url   TEXT NOT NULL DEFAULT '',         -- blob URL portrait
    position    INTEGER NOT NULL DEFAULT 0,
    published   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX team_kind_position_idx ON team (kind, position);
