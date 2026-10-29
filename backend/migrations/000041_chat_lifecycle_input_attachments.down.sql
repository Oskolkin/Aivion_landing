DROP INDEX IF EXISTS chat_attachments_draft_expires_idx;

ALTER TABLE chat_attachments
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS sha256,
    DROP COLUMN IF EXISTS openai_uploaded_at,
    DROP COLUMN IF EXISTS openai_file_id,
    DROP COLUMN IF EXISTS attachment_kind;

ALTER TABLE chat_attachments
    ALTER COLUMN export_type SET NOT NULL;

-- Cannot safely restore NOT NULL on session/message if draft rows exist; best-effort for down migration
DELETE FROM chat_attachments WHERE chat_session_id IS NULL OR chat_message_id IS NULL;

ALTER TABLE chat_attachments
    ALTER COLUMN chat_session_id SET NOT NULL,
    ALTER COLUMN chat_message_id SET NOT NULL;

DROP INDEX IF EXISTS chat_messages_client_request_id_uniq;
ALTER TABLE chat_messages DROP COLUMN IF EXISTS client_request_id;

DROP INDEX IF EXISTS chat_sessions_seller_action_key_uniq;
ALTER TABLE chat_sessions DROP COLUMN IF EXISTS action_key;
