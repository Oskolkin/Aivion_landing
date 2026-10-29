ALTER TABLE chat_attachments
    DROP CONSTRAINT IF EXISTS chat_attachments_extraction_status_chk;

ALTER TABLE chat_attachments
    DROP COLUMN IF EXISTS extraction_status,
    DROP COLUMN IF EXISTS extracted_text,
    DROP COLUMN IF EXISTS extracted_meta,
    DROP COLUMN IF EXISTS page_count,
    DROP COLUMN IF EXISTS sheet_count,
    DROP COLUMN IF EXISTS slide_count,
    DROP COLUMN IF EXISTS character_count,
    DROP COLUMN IF EXISTS extraction_error,
    DROP COLUMN IF EXISTS extracted_at;

DROP TABLE IF EXISTS chat_request_idempotency;
