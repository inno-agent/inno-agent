ALTER TABLE installations
    ALTER COLUMN refresh_ciphertext SET NOT NULL,
    ALTER COLUMN refresh_nonce      SET NOT NULL;
