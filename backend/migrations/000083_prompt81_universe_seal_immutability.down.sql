ALTER TABLE analysis_run_stages DROP CONSTRAINT IF EXISTS analysis_run_stages_input_hash_nonzero;

ALTER TABLE factual_universes DROP CONSTRAINT IF EXISTS factual_universes_manifest_artifact_fk;

DROP TRIGGER IF EXISTS trg_fu_entity_sets_immutable ON factual_universe_entity_sets;
DROP TRIGGER IF EXISTS trg_fu_dq_immutable ON factual_universe_dq_summary;
DROP TRIGGER IF EXISTS trg_fu_capabilities_immutable ON factual_universe_capabilities;
DROP TRIGGER IF EXISTS trg_fu_sources_immutable ON factual_universe_sources;
DROP FUNCTION IF EXISTS forbid_factual_universe_child_mutation();

-- Restore strict root immutability (no seal flip exception).
CREATE OR REPLACE FUNCTION forbid_factual_universe_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'factual universe is immutable' USING ERRCODE = '55000';
END;
$$;

ALTER TABLE factual_universes DROP COLUMN IF EXISTS seal_status;

ALTER TABLE factual_universe_sources DROP CONSTRAINT IF EXISTS factual_universe_sources_source_state_check;
ALTER TABLE factual_universe_sources ADD CONSTRAINT factual_universe_sources_source_state_check
  CHECK (source_state IN ('SUCCESS','EMPTY_VALID','PARTIAL','FAILED','NOT_ATTEMPTED'));
