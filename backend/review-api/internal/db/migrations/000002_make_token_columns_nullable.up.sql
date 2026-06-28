ALTER TABLE installations
    ALTER COLUMN refresh_ciphertext DROP NOT NULL,
    ALTER COLUMN refresh_nonce      DROP NOT NULL;
