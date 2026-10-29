ALTER TABLE chat_request_idempotency DROP COLUMN IF EXISTS binding_id;
ALTER TABLE chat_request_idempotency DROP COLUMN IF EXISTS turn_id;

ALTER TABLE chat_attachments DROP CONSTRAINT IF EXISTS chat_attachments_completeness_status_chk;
ALTER TABLE chat_attachments DROP COLUMN IF EXISTS completeness_status;

DROP TABLE IF EXISTS chat_export_provenance;
DROP TABLE IF EXISTS chat_action_binding_members;
DROP TABLE IF EXISTS chat_action_bindings;
DROP TABLE IF EXISTS chat_turn_grounding_refs;
DROP TABLE IF EXISTS chat_turn_tool_calls;
DROP TABLE IF EXISTS chat_turn_contexts;
DROP TABLE IF EXISTS chat_turns;
