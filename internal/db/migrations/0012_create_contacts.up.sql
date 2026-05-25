-- Contact form submissions from the public /contact page.
-- Read-only for the public POST; admin can list + delete.
CREATE TABLE contacts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    message    TEXT NOT NULL,
    -- Crude spam signal: visitor IP. Not used for blocking yet; helpful
    -- if a wave of garbage submissions ever needs triage.
    ip         TEXT NOT NULL DEFAULT '',
    -- Set to true when the admin has "read" the message via the admin
    -- panel. Doesn't restrict anything — just a UI hint.
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX contacts_created_at_idx ON contacts (created_at DESC);
