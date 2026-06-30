CREATE TABLE installations (
    gitflame_username  TEXT PRIMARY KEY,
    user_id            UUID NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
