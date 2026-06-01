-- Distinguish live-mode donations from test-mode ones. Existing rows were all
-- created in Stripe test mode, so they default to false; new rows created while
-- the backend runs with an sk_live_ key are stored as true.
ALTER TABLE donations ADD COLUMN livemode BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX donations_livemode_created_idx ON donations (livemode, created_at DESC);
