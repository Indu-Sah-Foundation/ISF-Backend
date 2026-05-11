CREATE TABLE donations (
                           id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                           amount_cents             INTEGER NOT NULL,
                           currency                 TEXT NOT NULL DEFAULT 'usd',
                           email                    TEXT,
                           name                     TEXT,
                           status                   TEXT NOT NULL DEFAULT 'pending',
                           stripe_session_id        TEXT NOT NULL UNIQUE,
                           stripe_payment_intent_id TEXT,
                           created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                           updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX donations_status_created_idx ON donations (status, created_at DESC);

CREATE TABLE stripe_events (
                               id           TEXT PRIMARY KEY,
                               received_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);