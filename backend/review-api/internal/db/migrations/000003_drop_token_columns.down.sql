ALTER TABLE installations
    ADD COLUMN refresh_ciphertext BYTEA,
    ADD COLUMN refresh_nonce      BYTEA;
