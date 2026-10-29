DROP INDEX IF EXISTS idx_sessions_review_invite_id;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_review_invite_id_fkey;
ALTER TABLE sessions DROP COLUMN IF EXISTS review_invite_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS session_kind;
ALTER TABLE sessions DROP COLUMN IF EXISTS read_only;
DROP TABLE IF EXISTS review_invites;
