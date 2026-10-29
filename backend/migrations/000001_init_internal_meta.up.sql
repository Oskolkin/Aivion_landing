CREATE TABLE IF NOT EXISTS internal_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO internal_meta (key, value)
VALUES ('bootstrap', 'ok')
ON CONFLICT (key) DO NOTHING;