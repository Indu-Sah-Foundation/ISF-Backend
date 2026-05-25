
CREATE TABLE gallery_tags (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX gallery_tags_position_idx ON gallery_tags (position);


INSERT INTO gallery_tags (name, position) VALUES
    ('Dental',    0),
    ('STEM',      1),
    ('Education', 2),
    ('Relief',    3),
    ('Community', 4),
    ('Events',    5)
ON CONFLICT (name) DO NOTHING;
