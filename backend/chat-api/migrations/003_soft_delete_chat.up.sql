ALTER TABLE chats ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_chats_user_id_deleted_at ON chats (user_id, deleted_at) WHERE deleted_at IS NULL;