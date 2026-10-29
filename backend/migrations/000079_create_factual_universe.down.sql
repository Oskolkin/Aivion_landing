DROP TRIGGER IF EXISTS trg_factual_universes_forbid_referenced_delete ON factual_universes;
DROP FUNCTION IF EXISTS forbid_factual_universe_delete_when_referenced();
DROP TRIGGER IF EXISTS trg_factual_universes_immutable ON factual_universes;
DROP FUNCTION IF EXISTS forbid_factual_universe_update();
DROP TABLE IF EXISTS factual_universe_entity_sets;
DROP TABLE IF EXISTS factual_universe_dq_summary;
DROP TABLE IF EXISTS factual_universe_capabilities;
DROP TABLE IF EXISTS factual_universe_sources;
DROP TABLE IF EXISTS factual_universes;
