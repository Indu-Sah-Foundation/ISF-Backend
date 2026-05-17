-- Achievements: letters of appreciation, recognitions, certificates.
-- Simple flat table — each row is one tile on the /achievements page.
CREATE TABLE achievements (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slot          TEXT NOT NULL UNIQUE,            -- stable slug for ordering / referencing
    title         TEXT NOT NULL,
    organization  TEXT NOT NULL DEFAULT '',        -- "Netrawati Dabajong Municipality"
    place         TEXT NOT NULL DEFAULT '',        -- "Dhading, Nepal"
    body          TEXT NOT NULL DEFAULT '',
    image_url     TEXT NOT NULL DEFAULT '',        -- blob URL for the certificate / letter
    awarded_at    DATE,                            -- nullable — show only year-month-day if set
    position      INTEGER NOT NULL DEFAULT 0,
    published     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX achievements_position_idx ON achievements (position);
