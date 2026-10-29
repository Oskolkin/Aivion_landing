ALTER TABLE domain_watermark_revisions
    DROP COLUMN IF EXISTS source_revision_hash,
    DROP COLUMN IF EXISTS semantic_mapping_hash;

ALTER TABLE domain_watermarks
    DROP COLUMN IF EXISTS source_revision_hash,
    DROP COLUMN IF EXISTS semantic_mapping_hash;

ALTER TABLE factual_universe_sources
    DROP COLUMN IF EXISTS source_finalized_at,
    DROP COLUMN IF EXISTS watermark_hash,
    DROP COLUMN IF EXISTS source_revision_hash,
    DROP COLUMN IF EXISTS semantic_mapping_version,
    DROP COLUMN IF EXISTS source_endpoint_family;
