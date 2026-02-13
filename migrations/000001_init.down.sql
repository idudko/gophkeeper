-- Drop triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_items_updated_at ON items;
DROP TRIGGER IF EXISTS update_user_syncs_updated_at ON user_syncs;

-- Drop indexes
DROP INDEX IF EXISTS idx_items_data_gin;
DROP INDEX IF EXISTS idx_items_user_deleted;
DROP INDEX IF EXISTS idx_items_updated_at;
DROP INDEX IF EXISTS idx_items_deleted_at;
DROP INDEX IF EXISTS idx_items_type;
DROP INDEX IF EXISTS idx_items_user_id;
DROP INDEX IF EXISTS idx_users_email;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (in reverse order of creation)
DROP TABLE IF EXISTS user_syncs;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS users;
