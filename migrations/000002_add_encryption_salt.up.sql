-- Add encryption_salt column to users table
-- This salt is used for client-side key derivation from master password

-- First, add the column with default value for existing users
ALTER TABLE users
ADD COLUMN IF NOT EXISTS encryption_salt BYTEA DEFAULT '';

-- Generate unique encryption salts for existing users that don't have one
UPDATE users
SET encryption_salt = encode(gen_random_bytes(16), 'hex')
WHERE encryption_salt = '' OR encryption_salt IS NULL;

-- Now make the column NOT NULL (after populating existing users)
ALTER TABLE users
ALTER COLUMN encryption_salt SET NOT NULL,
ALTER COLUMN encryption_salt SET DEFAULT '';

-- Add comment to document the purpose of this column
COMMENT ON COLUMN users.encryption_salt IS 'Hex-encoded salt for client-side encryption key derivation (Argon2id). Never expose to clients in API responses.';
