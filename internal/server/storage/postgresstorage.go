// Package postgresstorage provides PostgreSQL implementation of storage interfaces using pgx/v5.
package postgresstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStorage implements the Storage interface using PostgreSQL with pgx/v5.
type PostgresStorage struct {
	pool *pgxpool.Pool
}

// Config holds PostgreSQL connection configuration.
type Config struct {
	DSN               string
	MaxOpenConns      int32
	MaxIdleConns      int32
	ConnMaxLifetime   time.Duration
	ConnMaxIdleTime   time.Duration
	MinConns          int32
	HealthCheckPeriod time.Duration
}

// DefaultConfig returns default PostgreSQL configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxOpenConns:      25,
		MaxIdleConns:      5,
		MinConns:          2,
		ConnMaxLifetime:   5 * time.Minute,
		ConnMaxIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}
}

// NewPostgresStorage creates a new PostgreSQL storage instance with pgx/v5.
func NewPostgresStorage(ctx context.Context, cfg *Config) (*PostgresStorage, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Parse configuration with retry logic
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = cfg.MaxOpenConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime

	// Set up before connect callback for initial connection checks
	poolConfig.BeforeConnect = func(ctx context.Context, cfg *pgx.ConnConfig) error {
		// Add connection timeout
		cfg.ConnectTimeout = 10 * time.Second
		return nil
	}

	// Create pool with retry context
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection with retry logic
	if err := retryOperation(ctx, 3, func() error {
		return pool.Ping(ctx)
	}); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database after retries: %w", err)
	}

	return &PostgresStorage{pool: pool}, nil
}

// retryOperation executes a function with retry logic.
func retryOperation(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	for i := range maxRetries {
		if err := fn(); err != nil {
			lastErr = err
			// Check if error is retryable
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				// Connection errors are retryable
				if isConnectionError(pgErr.Code) {
					time.Sleep(time.Duration(i+1) * time.Second)
					continue
				}
			}
			// Context cancellation is not retryable
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
		} else {
			return nil
		}
	}
	return lastErr
}

// isConnectionError checks if a PostgreSQL error code indicates a connection issue.
func isConnectionError(code string) bool {
	// Common connection-related error codes
	connectionErrors := map[string]bool{
		"08006": true, // connection failure
		"08001": true, // SQLCLIENT unable to establish SQLconnection
		"08004": true, // SQLserver rejected establishment of SQLconnection
		"08007": true, // transaction resolution unknown
		"57P01": true, // admin shutdown
		"57P02": true, // crash shutdown
		"57P03": true, // cannot connect now
	}
	return connectionErrors[code]
}

// Close closes the database connection pool.
func (s *PostgresStorage) Close() error {
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}

// Ping checks if the database connection is alive with retry logic.
func (s *PostgresStorage) Ping(ctx context.Context) error {
	return retryOperation(ctx, 3, func() error {
		return s.pool.Ping(ctx)
	})
}

// CreateUser creates a new user in the database.
func (s *PostgresStorage) CreateUser(ctx context.Context, user *models.User) error {
	const query = `
		INSERT INTO users (id, email, password, encryption_salt, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, encryption_salt, created_at, updated_at
	`

	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.UpdatedAt = time.Now()

	err := s.pool.QueryRow(ctx, query,
		user.ID, user.Email, user.Password, user.EncryptionSalt, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("%w: %v", models.ErrUserAlreadyExists, err)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByEmail retrieves a user by email address.
func (s *PostgresStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const query = `
		SELECT id, email, encryption_salt, created_at, updated_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`

	var user models.User
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID.
func (s *PostgresStorage) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	const query = `
		SELECT id, email, encryption_salt, created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`

	var user models.User
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user.
func (s *PostgresStorage) UpdateUser(ctx context.Context, user *models.User) error {
	const query = `
		UPDATE users
		SET email = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, email, encryption_salt, created_at, updated_at
	`

	err := s.pool.QueryRow(ctx, query,
		user.Email, user.UpdatedAt, user.ID,
	).Scan(&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrUserNotFound
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user by ID.
func (s *PostgresStorage) DeleteUser(ctx context.Context, id string) error {
	const query = `DELETE FROM users WHERE id = $1`

	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// CreateItem creates a new item in the database.
func (s *PostgresStorage) CreateItem(ctx context.Context, item *models.Item) error {
	const query = `
		INSERT INTO items (id, user_id, type, version, meta, data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
	`

	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.Version == 0 {
		item.Version = 1
	}
	item.UpdatedAt = time.Now()

	// Marshal item data to JSONB
	dataJSON, err := s.marshalItemData(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item data: %w", err)
	}

	err = s.pool.QueryRow(ctx, query,
		item.ID, item.UserID, item.Type, item.Version, item.Meta, dataJSON,
		item.CreatedAt, item.UpdatedAt,
	).Scan(
		&item.ID, &item.UserID, &item.Type, &item.Version,
		&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	// Unmarshal data back to struct
	if err := s.unmarshalItemData(item, dataJSON); err != nil {
		return fmt.Errorf("failed to unmarshal item data: %w", err)
	}

	return nil
}

// GetItem retrieves a specific item by user ID and item ID.
func (s *PostgresStorage) GetItem(ctx context.Context, userID, itemID string) (*models.Item, error) {
	const query = `
		SELECT id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
		FROM items
		WHERE user_id = $1 AND id = $2
	`

	var item models.Item
	var dataJSON []byte

	err := s.pool.QueryRow(ctx, query, userID, itemID).Scan(
		&item.ID, &item.UserID, &item.Type, &item.Version,
		&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrItemNotFound
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	// Unmarshal data to struct
	if err := s.unmarshalItemData(&item, dataJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal item data: %w", err)
	}

	return &item, nil
}

// GetItems retrieves items for a user with optional filtering and retry logic.
func (s *PostgresStorage) GetItems(ctx context.Context, userID string, dataType models.DataType, limit, offset int) ([]*models.Item, error) {
	query := `
		SELECT id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
		FROM items
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	args := []any{userID}
	argPos := 2

	// Add type filter if specified
	if dataType != "" {
		query += fmt.Sprintf(" AND type = $%d", argPos)
		args = append(args, dataType)
		argPos++
	}

	// Add ordering and pagination
	query += " ORDER BY updated_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	var items []*models.Item
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.Item
		var dataJSON []byte

		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Type, &item.Version,
			&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item row: %w", err)
		}

		// Unmarshal data to struct
		if err := s.unmarshalItemData(&item, dataJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal item data: %w", err)
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}

// UpdateItem updates an existing item with optimistic locking.
func (s *PostgresStorage) UpdateItem(ctx context.Context, item *models.Item) error {
	const query = `
		UPDATE items
		SET type = $3, version = $4, meta = $5, data = $6, updated_at = NOW(), deleted_at = $7
		WHERE user_id = $1 AND id = $2 AND version = $4 - 1
		RETURNING id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
	`

	item.UpdatedAt = time.Now()
	item.Version++

	// Marshal item data to JSONB
	dataJSON, err := s.marshalItemData(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item data: %w", err)
	}

	err = s.pool.QueryRow(ctx, query,
		item.UserID, item.ID, item.Type, item.Version,
		item.Meta, dataJSON, item.DeletedAt,
	).Scan(
		&item.ID, &item.UserID, &item.Type, &item.Version,
		&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: item version mismatch", models.ErrVersionConflict)
		}
		return fmt.Errorf("failed to update item: %w", err)
	}

	// Unmarshal data back to struct
	if err := s.unmarshalItemData(item, dataJSON); err != nil {
		return fmt.Errorf("failed to unmarshal item data: %w", err)
	}

	return nil
}

// DeleteItem soft deletes an item by user ID and item ID.
func (s *PostgresStorage) DeleteItem(ctx context.Context, userID, itemID string) error {
	const query = `
		UPDATE items
		SET deleted_at = NOW()
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
	`

	result, err := s.pool.Exec(ctx, query, userID, itemID)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrItemNotFound
	}

	return nil
}

// GetItemsSince retrieves items updated since a given timestamp with retry logic.
func (s *PostgresStorage) GetItemsSince(ctx context.Context, userID string, since time.Time, limit, offset int) ([]*models.Item, error) {
	query := `
		SELECT id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
		FROM items
		WHERE user_id = $1 AND updated_at > $2
		ORDER BY updated_at DESC
	`
	args := []any{userID, since}
	argPos := 3

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	var items []*models.Item
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get items since: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.Item
		var dataJSON []byte

		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Type, &item.Version,
			&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item row: %w", err)
		}

		// Unmarshal data to struct
		if err := s.unmarshalItemData(&item, dataJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal item data: %w", err)
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}

// GetLastSync retrieves the last sync timestamp for a user with retry logic.
func (s *PostgresStorage) GetLastSync(ctx context.Context, userID string) (time.Time, error) {
	const query = `
		SELECT last_sync_at
		FROM user_syncs
		WHERE user_id = $1
	`

	var lastSync time.Time
	err := s.pool.QueryRow(ctx, query, userID).Scan(&lastSync)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil // No sync yet
		}
		return time.Time{}, fmt.Errorf("failed to get last sync: %w", err)
	}

	return lastSync, nil
}

// UpdateLastSync updates the last sync timestamp for a user with retry logic.
func (s *PostgresStorage) UpdateLastSync(ctx context.Context, userID string, timestamp time.Time) error {
	const query = `
		INSERT INTO user_syncs (user_id, last_sync_at, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET last_sync_at = $2, updated_at = NOW()
	`

	_, err := s.pool.Exec(ctx, query, userID, timestamp)
	if err != nil {
		return fmt.Errorf("failed to update last sync: %w", err)
	}

	return nil
}

// BeginTx starts a new database transaction.
func (s *PostgresStorage) BeginTx(ctx context.Context) (storage.Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &postgresTx{tx: tx, storage: s}, nil
}

// Helper functions for marshaling/unmarshaling item data

type itemData struct {
	Type     string               `json:"type"`
	Meta     string               `json:"meta"`
	Password *models.PasswordData `json:"password,omitempty"`
	Text     *models.TextData     `json:"text,omitempty"`
	Binary   *models.BinaryData   `json:"binary,omitempty"`
	Card     *models.CardData     `json:"card,omitempty"`
}

func (s *PostgresStorage) marshalItemData(item *models.Item) ([]byte, error) {
	return json.Marshal(itemData{
		Type:     string(item.Type),
		Meta:     item.Meta,
		Password: item.Password,
		Text:     item.Text,
		Binary:   item.Binary,
		Card:     item.Card,
	})
}

func (s *PostgresStorage) unmarshalItemData(item *models.Item, dataJSON []byte) error {
	var data itemData
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		return err
	}

	item.Meta = data.Meta
	item.Password = data.Password
	item.Text = data.Text
	item.Binary = data.Binary
	item.Card = data.Card

	return nil
}

// isDuplicateKeyError checks if error indicates a duplicate key constraint violation.
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}

// postgresTx implements the Transaction interface using pgx.
type postgresTx struct {
	tx      pgx.Tx
	storage *PostgresStorage
}

// Commit commits the transaction.
func (t *postgresTx) Commit() error {
	if err := t.tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction.
func (t *postgresTx) Rollback() error {
	if err := t.tx.Rollback(context.Background()); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// Transaction methods delegate to storage implementation with transaction context

func (t *postgresTx) CreateUser(ctx context.Context, user *models.User) error {
	const query = `
		INSERT INTO users (id, email, password, encryption_salt, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, encryption_salt, created_at, updated_at
	`

	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.UpdatedAt = time.Now()

	err := t.tx.QueryRow(ctx, query,
		user.ID, user.Email, user.Password, user.EncryptionSalt, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (t *postgresTx) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const query = `
		SELECT id, email, encryption_salt, created_at, updated_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`

	var user models.User
	err := t.tx.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (t *postgresTx) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	const query = `
		SELECT id, email, encryption_salt, created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`

	var user models.User
	err := t.tx.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (t *postgresTx) UpdateUser(ctx context.Context, user *models.User) error {
	const query = `
		UPDATE users
		SET email = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, email, encryption_salt, created_at, updated_at
	`

	err := t.tx.QueryRow(ctx, query,
		user.Email, user.ID,
	).Scan(&user.ID, &user.Email, &user.EncryptionSalt, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrUserNotFound
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (t *postgresTx) DeleteUser(ctx context.Context, id string) error {
	const query = `DELETE FROM users WHERE id = $1`

	result, err := t.tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

func (t *postgresTx) CreateItem(ctx context.Context, item *models.Item) error {
	const query = `
		INSERT INTO items (id, user_id, type, version, meta, data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
	`

	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.Version == 0 {
		item.Version = 1
	}
	item.UpdatedAt = time.Now()

	// Marshal item data to JSONB
	dataJSON, err := t.storage.marshalItemData(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item data: %w", err)
	}

	err = t.tx.QueryRow(ctx, query,
		item.ID, item.UserID, item.Type, item.Version, item.Meta, dataJSON,
		item.CreatedAt, item.UpdatedAt,
	).Scan(
		&item.ID, &item.UserID, &item.Type, &item.Version,
		&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	// Unmarshal data back to struct
	if err := t.storage.unmarshalItemData(item, dataJSON); err != nil {
		return fmt.Errorf("failed to unmarshal item data: %w", err)
	}

	return nil
}

func (t *postgresTx) GetItem(ctx context.Context, userID, itemID string) (*models.Item, error) {
	const query = `
		SELECT id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
		FROM items
		WHERE user_id = $1 AND id = $2
	`

	var item models.Item
	var dataJSON []byte

	err := t.tx.QueryRow(ctx, query, userID, itemID).Scan(
		&item.ID, &item.UserID, &item.Type, &item.Version,
		&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrItemNotFound
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	// Unmarshal data to struct
	if err := t.storage.unmarshalItemData(&item, dataJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal item data: %w", err)
	}

	return &item, nil
}

func (t *postgresTx) GetItems(ctx context.Context, userID string, dataType models.DataType, limit, offset int) ([]*models.Item, error) {
	query := `
		SELECT id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
		FROM items
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	args := []any{userID}
	argPos := 2

	// Add type filter if specified
	if dataType != "" {
		query += fmt.Sprintf(" AND type = $%d", argPos)
		args = append(args, dataType)
		argPos++
	}

	// Add ordering and pagination
	query += " ORDER BY updated_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		var item models.Item
		var dataJSON []byte

		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Type, &item.Version,
			&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item row: %w", err)
		}

		// Unmarshal data to struct
		if err := t.storage.unmarshalItemData(&item, dataJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal item data: %w", err)
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}

func (t *postgresTx) UpdateItem(ctx context.Context, item *models.Item) error {
	const query = `
		UPDATE items
		SET type = $3, version = $4, meta = $5, data = $6, updated_at = NOW(), deleted_at = $7
		WHERE user_id = $1 AND id = $2 AND version = $4 - 1
		RETURNING id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
	`

	item.UpdatedAt = time.Now()
	item.Version++

	// Marshal item data to JSONB
	dataJSON, err := t.storage.marshalItemData(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item data: %w", err)
	}

	err = t.tx.QueryRow(ctx, query,
		item.UserID, item.ID, item.Type, item.Version,
		item.Meta, dataJSON, item.DeletedAt,
	).Scan(
		&item.ID, &item.UserID, &item.Type, &item.Version,
		&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: item version mismatch", models.ErrVersionConflict)
		}
		return fmt.Errorf("failed to update item: %w", err)
	}

	// Unmarshal data back to struct
	if err := t.storage.unmarshalItemData(item, dataJSON); err != nil {
		return fmt.Errorf("failed to unmarshal item data: %w", err)
	}

	return nil
}

func (t *postgresTx) DeleteItem(ctx context.Context, userID, itemID string) error {
	const query = `
		UPDATE items
		SET deleted_at = NOW()
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
	`

	result, err := t.tx.Exec(ctx, query, userID, itemID)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return models.ErrItemNotFound
	}

	return nil
}

func (t *postgresTx) GetItemsSince(ctx context.Context, userID string, since time.Time, limit, offset int) ([]*models.Item, error) {
	query := `
		SELECT id, user_id, type, version, meta, data, created_at, updated_at, deleted_at
		FROM items
		WHERE user_id = $1 AND updated_at > $2
		ORDER BY updated_at DESC
	`
	args := []any{userID, since}
	argPos := 3

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get items since: %w", err)
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		var item models.Item
		var dataJSON []byte

		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Type, &item.Version,
			&item.Meta, &dataJSON, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item row: %w", err)
		}

		// Unmarshal data to struct
		if err := t.storage.unmarshalItemData(&item, dataJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal item data: %w", err)
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}

func (t *postgresTx) GetLastSync(ctx context.Context, userID string) (time.Time, error) {
	const query = `
		SELECT last_sync_at
		FROM user_syncs
		WHERE user_id = $1
	`

	var lastSync time.Time
	err := t.tx.QueryRow(ctx, query, userID).Scan(&lastSync)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil // No sync yet
		}
		return time.Time{}, fmt.Errorf("failed to get last sync: %w", err)
	}

	return lastSync, nil
}

func (t *postgresTx) UpdateLastSync(ctx context.Context, userID string, timestamp time.Time) error {
	const query = `
		INSERT INTO user_syncs (user_id, last_sync_at, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET last_sync_at = $2, updated_at = NOW()
	`

	_, err := t.tx.Exec(ctx, query, userID, timestamp)
	if err != nil {
		return fmt.Errorf("failed to update last sync: %w", err)
	}

	return nil
}

func (t *postgresTx) BeginTx(ctx context.Context) (storage.Transaction, error) {
	// Already in a transaction
	return t, nil
}

func (t *postgresTx) Ping(ctx context.Context) error {
	return nil // Transaction already established
}

func (t *postgresTx) Close() error {
	return nil // Managed by the pool
}
