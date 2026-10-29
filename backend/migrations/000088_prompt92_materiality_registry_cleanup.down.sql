-- Revert Prompt №9.2 materiality registry cleanup.

ALTER TABLE materiality_policy_versions
    DROP CONSTRAINT IF EXISTS materiality_policy_versions_policy_hash_check;
ALTER TABLE materiality_policy_versions
    ADD CONSTRAINT materiality_policy_versions_policy_hash_check
    CHECK (policy_hash ~ '^[0-9a-f]{64}$' AND policy_hash <> repeat('0', 64));
