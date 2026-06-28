CREATE TABLE service_clients (
    client_id   TEXT PRIMARY KEY,
    secret_hash BYTEA NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
