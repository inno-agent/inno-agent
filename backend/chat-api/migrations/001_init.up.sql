CREATE TABLE chats (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(255) NOT NULL,
    title       VARCHAR(512),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_chats_user_id ON chats(user_id, updated_at DESC);
CREATE INDEX idx_chats_user_id_deleted_at ON chats (user_id, deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(255) NOT NULL,
    chat_id     UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role        VARCHAR(16) NOT NULL CHECK (role IN ('user', 'assistant')),
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_chat_id ON messages(chat_id, created_at);
CREATE INDEX idx_messages_user_id ON messages(user_id);
