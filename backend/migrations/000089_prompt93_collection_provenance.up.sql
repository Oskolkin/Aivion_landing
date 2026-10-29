-- Prompt №9.3 A: honest collection source provenance — separate semantic mapping from provider revision.

ALTER TABLE collection_domain_runs
    ADD COLUMN semantic_mapping_hash TEXT NULL
        CHECK (semantic_mapping_hash IS NULL OR semantic_mapping_hash ~ '^[0-9a-f]{64}$');

-- Backfill: legacy source_revision_hash stored SemanticMappingHash semantics.
UPDATE collection_domain_runs
SET semantic_mapping_hash = source_revision_hash
WHERE semantic_mapping_hash IS NULL
  AND source_revision_hash IS NOT NULL
  AND source_revision_hash ~ '^[0-9a-f]{64}$'
  AND semantic_mapping_version IS NOT NULL;

COMMENT ON COLUMN collection_domain_runs.source_revision_hash IS
    'Legacy: historically aliased semantic_mapping_hash. New runs: provider/source revision only when exposed; NULL otherwise.';
COMMENT ON COLUMN collection_domain_runs.semantic_mapping_hash IS
    'SHA-256 of checked-in source→canonical mapping semantics. Never duplicates provider revision.';
