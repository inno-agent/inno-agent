ALTER TABLE installations
    DROP COLUMN IF EXISTS refresh_ciphertext,
    DROP COLUMN IF EXISTS refresh_nonce;
