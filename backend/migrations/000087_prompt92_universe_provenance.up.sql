-- Prompt №9.2: persist full source provenance in PG (not only MinIO).

ALTER TABLE factual_universe_sources
    ADD COLUMN source_endpoint_family TEXT NULL,
    ADD COLUMN semantic_mapping_version TEXT NULL,
    ADD COLUMN source_revision_hash TEXT NULL
        CHECK (source_revision_hash IS NULL OR source_revision_hash ~ '^[0-9a-f]{64}$'),
    ADD COLUMN watermark_hash TEXT NULL
        CHECK (watermark_hash IS NULL OR watermark_hash ~ '^[0-9a-f]{64}$'),
    ADD COLUMN source_finalized_at TIMESTAMPTZ NULL;

ALTER TABLE domain_watermarks
    ADD COLUMN semantic_mapping_hash TEXT NULL
        CHECK (semantic_mapping_hash IS NULL OR semantic_mapping_hash ~ '^[0-9a-f]{64}$'),
    ADD COLUMN source_revision_hash TEXT NULL
        CHECK (source_revision_hash IS NULL OR source_revision_hash ~ '^[0-9a-f]{64}$');

ALTER TABLE domain_watermark_revisions
    ADD COLUMN semantic_mapping_hash TEXT NULL
        CHECK (semantic_mapping_hash IS NULL OR semantic_mapping_hash ~ '^[0-9a-f]{64}$'),
    ADD COLUMN source_revision_hash TEXT NULL
        CHECK (source_revision_hash IS NULL OR source_revision_hash ~ '^[0-9a-f]{64}$');
