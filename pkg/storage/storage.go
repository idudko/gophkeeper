// Package storage provides storage interfaces and implementations for the GophKeeper system.
package storage

import (
	"context"
	"time"

	"github.com/idudko/gophkeeper/internal/models"
)

// Storage defines the interface for data persistence operations.
type Storage interface {
	// User operations
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, id string) error

	// Item operations
	CreateItem(ctx context.Context, item *models.Item) error
	GetItem(ctx context.Context, userID, itemID string) (*models.Item, error)
	GetItems(ctx context.Context, userID string, dataType models.DataType, limit, offset int) ([]*models.Item, error)
	UpdateItem(ctx context.Context, item *models.Item) error
	DeleteItem(ctx context.Context, userID, itemID string) error

	// Sync operations
	GetItemsSince(ctx context.Context, userID string, since time.Time, limit, offset int) ([]*models.Item, error)
	GetLastSync(ctx context.Context, userID string) (time.Time, error)
	UpdateLastSync(ctx context.Context, userID string, timestamp time.Time) error

	// Transaction support
	BeginTx(ctx context.Context) (Transaction, error)

	// Health check
	Ping(ctx context.Context) error
	Close() error
}

// Transaction defines the interface for database transactions.
type Transaction interface {
	Commit() error
	Rollback() error

	// Storage operations within transaction
	Storage
}

// ItemRepository defines operations for managing items.
type ItemRepository interface {
	Create(ctx context.Context, item *models.Item) error
	Get(ctx context.Context, userID, itemID string) (*models.Item, error)
	List(ctx context.Context, userID string, filter *ItemFilter) ([]*models.Item, error)
	Update(ctx context.Context, item *models.Item) error
	Delete(ctx context.Context, userID, itemID string) error
}

// UserRepository defines operations for managing users.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
}

// SyncRepository defines operations for synchronizing data.
type SyncRepository interface {
	GetChanges(ctx context.Context, userID string, since time.Time) ([]*models.Item, error)
	SyncItems(ctx context.Context, userID string, items []*models.Item) ([]*models.Item, []*models.Item, error)
}

// ItemFilter defines filtering options for listing items.
type ItemFilter struct {
	DataType    models.DataType
	Limit       int
	Offset      int
	Since       *time.Time
	Until       *time.Time
	SearchQuery string
	SortBy      string
	SortOrder   string // "asc" or "desc"
}
