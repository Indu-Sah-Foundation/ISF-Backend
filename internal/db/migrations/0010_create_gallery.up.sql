-- Gallery: photos shown on /gallery. Each row references an image
-- already uploaded to Azure Blob storage (via the existing /admin/images/sas
-- endpoint), with optional title/caption/tags + size hint for the layout.
CREATE TABLE gallery (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    src         TEXT NOT NULL,                    -- public blob URL
    title       TEXT NOT NULL DEFAULT '',
    caption     TEXT NOT NULL DEFAULT '',
    size        TEXT NOT NULL DEFAULT 'M' CHECK (size IN ('S','M','L','XL')),
    tags        TEXT[] NOT NULL DEFAULT '{}',
    position    INTEGER NOT NULL DEFAULT 0,
    published   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX gallery_position_idx ON gallery (position);
