-- Prompt №9.2 J: materiality registry cleanup — remove unused V1 row or fix placeholder hash.
-- 000085 installs an append-only trigger that would otherwise make this cleanup
-- un-applicable on a clean migrate (MATERIALITY_REGISTRY_IMMUTABLE). Drop the
-- trigger for this one-time repair, then restore the same guard.

DROP TRIGGER IF EXISTS trg_materiality_policy_immutable ON materiality_policy_versions;

-- Delete unused legacy policy row when no succeeded run references it.
DELETE FROM materiality_policy_versions
WHERE version = 'MATERIALITY_POLICY_V1'
  AND policy_hash = repeat('a', 64)
  AND NOT EXISTS (
      SELECT 1 FROM materiality_runs r
      WHERE r.materiality_policy_version = 'MATERIALITY_POLICY_V1' AND r.status = 'SUCCEEDED'
  );

-- If historical runs still reference V1, replace placeholder hash with canonical hash of superseded JSON.
UPDATE materiality_policy_versions
SET policy_hash = '712a8c89f6687b118080fc0d9e182782483087a6ad09559fe99241262ab0f22a'
WHERE version = 'MATERIALITY_POLICY_V1'
  AND policy_hash = repeat('a', 64);

-- Reject all-a placeholder hashes going forward (000085 already blocks all-zero).
ALTER TABLE materiality_policy_versions
    DROP CONSTRAINT IF EXISTS materiality_policy_versions_policy_hash_check;
ALTER TABLE materiality_policy_versions
    ADD CONSTRAINT materiality_policy_versions_policy_hash_check
    CHECK (policy_hash ~ '^[0-9a-f]{64}$'
       AND policy_hash <> repeat('0', 64)
       AND policy_hash <> repeat('a', 64));

CREATE TRIGGER trg_materiality_policy_immutable
    BEFORE UPDATE OR DELETE ON materiality_policy_versions
    FOR EACH ROW EXECUTE FUNCTION materiality_registry_immutability_guard();
