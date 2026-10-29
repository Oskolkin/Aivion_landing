-- Prompt №9.1: Materiality registry immutability, real hashes, accepted-result guards.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Replace zero-hash budget seed when no SUCCEEDED materiality run references it.
UPDATE materiality_budget_profiles
SET budget_hash = '928a047fa6f29cb2266725e92414c374cb4619e66e543da3ca3cba507d5b1064',
    budget_json = '{"version":"DISCOVERY_STANDARD_V1","max_categories":15,"max_skus":40,"max_campaigns":15,"anchor_max_categories":8,"anchor_max_skus":20,"anchor_max_campaigns":8}'::jsonb
WHERE version = 'DISCOVERY_STANDARD_V1'
  AND budget_hash = repeat('0', 64)
  AND NOT EXISTS (
      SELECT 1 FROM materiality_runs r
      WHERE r.budget_profile_version = 'DISCOVERY_STANDARD_V1' AND r.status = 'SUCCEEDED'
  );

-- Replace zero-hash MATERIALITY_POLICY_V1 only if unused; keep as historical immutable row.
UPDATE materiality_policy_versions
SET policy_hash = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    policy_json = '{"version":"MATERIALITY_POLICY_V1","status":"SUPERSEDED_BY_V1_1","note":"absolute soft-diversity pilot; do not mutate"}'::jsonb
WHERE version = 'MATERIALITY_POLICY_V1'
  AND policy_hash = repeat('0', 64)
  AND NOT EXISTS (
      SELECT 1 FROM materiality_runs r
      WHERE r.materiality_policy_version = 'MATERIALITY_POLICY_V1' AND r.status = 'SUCCEEDED'
  );

-- Active Prompt 9.1 policy (MATERIALITY_POLICY_V1_1) with real canonical hash from Go CurrentPolicy().
INSERT INTO materiality_policy_versions (version, policy_hash, policy_json)
VALUES (
    'MATERIALITY_POLICY_V1_1',
    '76a74634d4851eac3f8d7788c928246e7788794190631237df43d4e74d153f6a',
    '{"version":"MATERIALITY_POLICY_V1_1","lane_registry_version":"MATERIALITY_LANE_REGISTRY_V1","rrf_k":60,"note":"relative soft-diversity; authoritative JSON is Go PolicySeed"}'::jsonb
)
ON CONFLICT (version) DO NOTHING;

-- Real lane config hashes from semantic columns (deterministic SHA-256 of config identity).
UPDATE materiality_lane_versions lv
SET config_hash = encode(
    digest(
        lv.lane_key || '|' || lv.entity_kind || '|' || lv.lane_version || '|' ||
        lv.ranking_semantic_key || '|' || lv.direction || '|' || lv.required_capability || '|' ||
        lv.allowed_readiness::text || '|' || lv.exposure_policy_key || '|' ||
        lv.candidate_limit::text || '|' || lv.weight::text || '|' || lv.typed_impl_key || '|' || lv.policy_version,
        'sha256'
    ),
    'hex'
)
WHERE config_hash = repeat('0', 64);

-- Point lane versions at V1_1 policy seal when still on V1 and unused by succeeded runs.
UPDATE materiality_lane_versions
SET policy_version = 'MATERIALITY_POLICY_V1_1'
WHERE policy_version = 'MATERIALITY_POLICY_V1'
  AND NOT EXISTS (SELECT 1 FROM materiality_runs r WHERE r.status = 'SUCCEEDED');

-- Reject all-zero placeholder hashes going forward.
ALTER TABLE materiality_policy_versions
    DROP CONSTRAINT IF EXISTS materiality_policy_versions_policy_hash_check;
ALTER TABLE materiality_policy_versions
    ADD CONSTRAINT materiality_policy_versions_policy_hash_check
    CHECK (policy_hash ~ '^[0-9a-f]{64}$' AND policy_hash <> repeat('0', 64));

ALTER TABLE materiality_budget_profiles
    DROP CONSTRAINT IF EXISTS materiality_budget_profiles_budget_hash_check;
ALTER TABLE materiality_budget_profiles
    ADD CONSTRAINT materiality_budget_profiles_budget_hash_check
    CHECK (budget_hash ~ '^[0-9a-f]{64}$' AND budget_hash <> repeat('0', 64));

ALTER TABLE materiality_lane_versions
    DROP CONSTRAINT IF EXISTS materiality_lane_versions_config_hash_check;
ALTER TABLE materiality_lane_versions
    ADD CONSTRAINT materiality_lane_versions_config_hash_check
    CHECK (config_hash ~ '^[0-9a-f]{64}$' AND config_hash <> repeat('0', 64));

-- Immutable accepted Materiality runs: block UPDATE of semantic columns and DELETE when SUCCEEDED.
CREATE OR REPLACE FUNCTION materiality_runs_immutability_guard() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status = 'SUCCEEDED' THEN
            RAISE EXCEPTION 'MATERIALITY_RUN_IMMUTABLE: cannot delete SUCCEEDED materiality_runs id=%', OLD.id;
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.status = 'SUCCEEDED' THEN
        IF NEW.ranking_hash IS DISTINCT FROM OLD.ranking_hash
            OR NEW.selection_hash IS DISTINCT FROM OLD.selection_hash
            OR NEW.materiality_policy_hash IS DISTINCT FROM OLD.materiality_policy_hash
            OR NEW.budget_profile_hash IS DISTINCT FROM OLD.budget_profile_hash
            OR NEW.input_landscape_core_hash IS DISTINCT FROM OLD.input_landscape_core_hash
            OR NEW.materiality_policy_version IS DISTINCT FROM OLD.materiality_policy_version
            OR NEW.budget_profile_version IS DISTINCT FROM OLD.budget_profile_version
            OR NEW.seller_account_id IS DISTINCT FROM OLD.seller_account_id
            OR NEW.factual_universe_id IS DISTINCT FROM OLD.factual_universe_id
        THEN
            RAISE EXCEPTION 'MATERIALITY_RESULT_CONFLICT: cannot mutate SUCCEEDED materiality_runs id=%', OLD.id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_materiality_runs_immutability ON materiality_runs;
CREATE TRIGGER trg_materiality_runs_immutability
    BEFORE UPDATE OR DELETE ON materiality_runs
    FOR EACH ROW EXECUTE FUNCTION materiality_runs_immutability_guard();

-- Selections append-once: no UPDATE/DELETE after parent run SUCCEEDED.
CREATE OR REPLACE FUNCTION materiality_selections_immutability_guard() RETURNS trigger AS $$
DECLARE
    run_status TEXT;
    rid BIGINT;
BEGIN
    rid := COALESCE(NEW.materiality_run_id, OLD.materiality_run_id);
    SELECT status INTO run_status FROM materiality_runs WHERE id = rid;
    IF run_status = 'SUCCEEDED' AND TG_OP IN ('UPDATE', 'DELETE') THEN
        RAISE EXCEPTION 'MATERIALITY_SELECTION_IMMUTABLE: cannot % selections for SUCCEEDED run', TG_OP;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_materiality_selections_immutability ON materiality_selections;
CREATE TRIGGER trg_materiality_selections_immutability
    BEFORE UPDATE OR DELETE ON materiality_selections
    FOR EACH ROW EXECUTE FUNCTION materiality_selections_immutability_guard();

-- Policy / budget / lane version append-only (no in-place mutation).
CREATE OR REPLACE FUNCTION materiality_registry_immutability_guard() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF TG_TABLE_NAME = 'materiality_policy_versions' AND NEW.policy_hash IS DISTINCT FROM OLD.policy_hash THEN
            RAISE EXCEPTION 'CONTRACT_VERSION_COLLISION: materiality_policy_versions.version=%', OLD.version;
        END IF;
        IF TG_TABLE_NAME = 'materiality_budget_profiles' AND NEW.budget_hash IS DISTINCT FROM OLD.budget_hash THEN
            RAISE EXCEPTION 'CONTRACT_VERSION_COLLISION: materiality_budget_profiles.version=%', OLD.version;
        END IF;
        IF TG_TABLE_NAME = 'materiality_lane_versions' AND NEW.config_hash IS DISTINCT FROM OLD.config_hash THEN
            RAISE EXCEPTION 'CONTRACT_VERSION_COLLISION: materiality_lane_versions %/%', OLD.lane_key, OLD.entity_kind;
        END IF;
        RAISE EXCEPTION 'MATERIALITY_REGISTRY_IMMUTABLE: %', TG_TABLE_NAME;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'MATERIALITY_REGISTRY_IMMUTABLE: cannot delete from %', TG_TABLE_NAME;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_materiality_policy_immutable ON materiality_policy_versions;
CREATE TRIGGER trg_materiality_policy_immutable
    BEFORE UPDATE OR DELETE ON materiality_policy_versions
    FOR EACH ROW EXECUTE FUNCTION materiality_registry_immutability_guard();

DROP TRIGGER IF EXISTS trg_materiality_budget_immutable ON materiality_budget_profiles;
CREATE TRIGGER trg_materiality_budget_immutable
    BEFORE UPDATE OR DELETE ON materiality_budget_profiles
    FOR EACH ROW EXECUTE FUNCTION materiality_registry_immutability_guard();

DROP TRIGGER IF EXISTS trg_materiality_lane_ver_immutable ON materiality_lane_versions;
CREATE TRIGGER trg_materiality_lane_ver_immutable
    BEFORE UPDATE OR DELETE ON materiality_lane_versions
    FOR EACH ROW EXECUTE FUNCTION materiality_registry_immutability_guard();
