CREATE TABLE bot_principals (
    user_id            UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    gitflame_username  VARCHAR(255) UNIQUE NOT NULL,
    consented_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
