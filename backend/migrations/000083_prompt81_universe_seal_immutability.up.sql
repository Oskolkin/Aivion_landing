-- Prompt 8.1: Universe seal lifecycle, child immutability, extended source states, manifest FK.

-- 1) Extended factual source states (readiness-aligned; keep legacy values).
ALTER TABLE factual_universe_sources DROP CONSTRAINT IF EXISTS factual_universe_sources_source_state_check;
ALTER TABLE factual_universe_sources ADD CONSTRAINT factual_universe_sources_source_state_check
  CHECK (source_state IN (
    'SUCCESS','EMPTY_VALID','PARTIAL','FAILED','NOT_ATTEMPTED',
    'UNAVAILABLE','NOT_APPLICABLE','PENDING_ACCEPTANCE','STALE'
  ));

-- 2) PREPARING → SEALED lifecycle. Existing rows are treated as already sealed.
ALTER TABLE factual_universes
  ADD COLUMN IF NOT EXISTS seal_status TEXT NOT NULL DEFAULT 'SEALED'
    CHECK (seal_status IN ('PREPARING','SEALED'));

-- New seals insert as PREPARING then flip to SEALED in the same transaction.
ALTER TABLE factual_universes ALTER COLUMN seal_status SET DEFAULT 'PREPARING';

-- 3) Child mutation guards: only PREPARING universes accept inserts; never UPDATE/DELETE children.
CREATE OR REPLACE FUNCTION forbid_factual_universe_child_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE st TEXT;
BEGIN
  IF TG_OP = 'INSERT' THEN
    SELECT seal_status INTO st FROM factual_universes WHERE id = NEW.universe_id;
    IF st IS NULL THEN
      RAISE EXCEPTION 'factual universe missing for child insert' USING ERRCODE = '23503';
    END IF;
    IF st = 'SEALED' THEN
      RAISE EXCEPTION 'factual universe children are immutable after seal' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'factual universe children cannot be updated or deleted' USING ERRCODE = '55000';
END;
$$;

DROP TRIGGER IF EXISTS trg_fu_sources_immutable ON factual_universe_sources;
CREATE TRIGGER trg_fu_sources_immutable
  BEFORE INSERT OR UPDATE OR DELETE ON factual_universe_sources
  FOR EACH ROW EXECUTE FUNCTION forbid_factual_universe_child_mutation();

DROP TRIGGER IF EXISTS trg_fu_capabilities_immutable ON factual_universe_capabilities;
CREATE TRIGGER trg_fu_capabilities_immutable
  BEFORE INSERT OR UPDATE OR DELETE ON factual_universe_capabilities
  FOR EACH ROW EXECUTE FUNCTION forbid_factual_universe_child_mutation();

DROP TRIGGER IF EXISTS trg_fu_dq_immutable ON factual_universe_dq_summary;
CREATE TRIGGER trg_fu_dq_immutable
  BEFORE INSERT OR UPDATE OR DELETE ON factual_universe_dq_summary
  FOR EACH ROW EXECUTE FUNCTION forbid_factual_universe_child_mutation();

DROP TRIGGER IF EXISTS trg_fu_entity_sets_immutable ON factual_universe_entity_sets;
CREATE TRIGGER trg_fu_entity_sets_immutable
  BEFORE INSERT OR UPDATE OR DELETE ON factual_universe_entity_sets
  FOR EACH ROW EXECUTE FUNCTION forbid_factual_universe_child_mutation();

-- Seal flip is a one-way root transition (still forbidden generic UPDATE except seal_status PREPARING→SEALED).
CREATE OR REPLACE FUNCTION forbid_factual_universe_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.seal_status = 'PREPARING' AND NEW.seal_status = 'SEALED'
     AND OLD.seller_account_id = NEW.seller_account_id
     AND OLD.analysis_date = NEW.analysis_date
     AND OLD.analysis_cutoff_at = NEW.analysis_cutoff_at
     AND OLD.manifest_hash = NEW.manifest_hash
     AND OLD.manifest_schema_version = NEW.manifest_schema_version
     AND OLD.temporal_policy_version = NEW.temporal_policy_version
     AND OLD.capability_policy_version = NEW.capability_policy_version
     AND OLD.dq_policy_version = NEW.dq_policy_version
     AND OLD.factual_api_version = NEW.factual_api_version
     AND OLD.universe_policy_version = NEW.universe_policy_version
     AND COALESCE(OLD.manifest_artifact_id,0) = COALESCE(NEW.manifest_artifact_id,0)
  THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'factual universe is immutable' USING ERRCODE = '55000';
END;
$$;

-- 4) Manifest artifact FK (deferrable: artifact may be inserted in same tx before seal flip).
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'factual_universes_manifest_artifact_fk'
  ) THEN
    ALTER TABLE factual_universes
      ADD CONSTRAINT factual_universes_manifest_artifact_fk
      FOREIGN KEY (manifest_artifact_id) REFERENCES analysis_artifacts(id)
      ON DELETE RESTRICT
      DEFERRABLE INITIALLY DEFERRED;
  END IF;
END $$;

-- 5) Reject sentinel zero input hashes at claim time (application enforces; DB soft-checks non-zero when set).
ALTER TABLE analysis_run_stages DROP CONSTRAINT IF EXISTS analysis_run_stages_input_hash_nonzero;
ALTER TABLE analysis_run_stages
  ADD CONSTRAINT analysis_run_stages_input_hash_nonzero
  CHECK (input_hash IS NULL OR (input_hash ~ '^[0-9a-f]{64}$' AND input_hash <> repeat('0',64)));
