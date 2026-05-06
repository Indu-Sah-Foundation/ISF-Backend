CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE people (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX people_email_idx ON people (email);