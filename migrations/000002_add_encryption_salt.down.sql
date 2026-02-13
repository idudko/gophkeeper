-- Remove encryption_salt column from users table
ALTER TABLE users
DROP COLUMN IF EXISTS encryption_salt;

-- Remove comment (optional, will be removed with column)
COMMENT ON COLUMN users.encryption_salt IS NULL;
