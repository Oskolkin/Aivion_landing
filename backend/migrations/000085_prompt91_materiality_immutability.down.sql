-- Reverse Prompt №9.1 materiality immutability guards (keeps hash data).

DROP TRIGGER IF EXISTS trg_materiality_lane_ver_immutable ON materiality_lane_versions;
DROP TRIGGER IF EXISTS trg_materiality_budget_immutable ON materiality_budget_profiles;
DROP TRIGGER IF EXISTS trg_materiality_policy_immutable ON materiality_policy_versions;
DROP TRIGGER IF EXISTS trg_materiality_selections_immutability ON materiality_selections;
DROP TRIGGER IF EXISTS trg_materiality_runs_immutability ON materiality_runs;

DROP FUNCTION IF EXISTS materiality_registry_immutability_guard();
DROP FUNCTION IF EXISTS materiality_selections_immutability_guard();
DROP FUNCTION IF EXISTS materiality_runs_immutability_guard();

DELETE FROM materiality_policy_versions WHERE version = 'MATERIALITY_POLICY_V1_1';

ALTER TABLE materiality_lane_versions DROP CONSTRAINT IF EXISTS materiality_lane_versions_config_hash_check;
ALTER TABLE materiality_lane_versions
    ADD CONSTRAINT materiality_lane_versions_config_hash_check
    CHECK (config_hash ~ '^[0-9a-f]{64}$');

ALTER TABLE materiality_budget_profiles DROP CONSTRAINT IF EXISTS materiality_budget_profiles_budget_hash_check;
ALTER TABLE materiality_budget_profiles
    ADD CONSTRAINT materiality_budget_profiles_budget_hash_check
    CHECK (budget_hash ~ '^[0-9a-f]{64}$');

ALTER TABLE materiality_policy_versions DROP CONSTRAINT IF EXISTS materiality_policy_versions_policy_hash_check;
ALTER TABLE materiality_policy_versions
    ADD CONSTRAINT materiality_policy_versions_policy_hash_check
    CHECK (policy_hash ~ '^[0-9a-f]{64}$');
