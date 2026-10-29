-- Prompt 13.1: machine-readable revalidation spec on persisted proposal supports
ALTER TABLE action_proposal_supports
    ADD COLUMN IF NOT EXISTS revalidation_json JSONB NOT NULL DEFAULT '{}'::jsonb;
