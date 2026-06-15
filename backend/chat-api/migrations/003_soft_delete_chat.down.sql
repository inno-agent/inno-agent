DROP INDEX IF EXISTS idx_chats_user_id_deleted_at;
ALTER TABLE chats DROP COLUMN IF EXISTS deleted_at;