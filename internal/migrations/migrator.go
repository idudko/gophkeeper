// Package migrations provides database migration functionality for GophKeeper.
package migrations

import (
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog"

	"github.com/idudko/gophkeeper/migrations"
)

// Migrator handles database migrations.
type Migrator struct {
	migrate *migrate.Migrate
	logger  zerolog.Logger
}

// NewMigrator creates a new migrator instance.
func NewMigrator(databaseURL string, logger zerolog.Logger) (*Migrator, error) {
	// Create source from embedded migrations
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{
		migrate: m,
		logger:  logger,
	}, nil
}

// Up applies all pending migrations.
func (m *Migrator) Up() error {
	startTime := time.Now()

	m.logger.Info().Msg("Starting database migrations...")

	err := m.migrate.Up()
	if err != nil {
		if err == migrate.ErrNoChange {
			m.logger.Info().Msg("No new migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	version, dirty, _ := m.migrate.Version()

	m.logger.Info().
		Uint("version", version).
		Bool("dirty", dirty).
		Dur("duration", time.Since(startTime)).
		Msg("Migrations applied successfully")

	return nil
}

// Down rolls back the last migration.
func (m *Migrator) Down() error {
	startTime := time.Now()

	m.logger.Info().Msg("Rolling back last migration...")

	err := m.migrate.Down()
	if err != nil {
		if err == migrate.ErrNoChange {
			m.logger.Info().Msg("No migrations to roll back")
			return nil
		}
		return fmt.Errorf("failed to roll back migration: %w", err)
	}

	m.logger.Info().
		Dur("duration", time.Since(startTime)).
		Msg("Migration rolled back successfully")

	return nil
}

// Version returns the current migration version.
func (m *Migrator) Version() (uint, bool, error) {
	return m.migrate.Version()
}

// Close closes the migrator and releases resources.
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	if sourceErr != nil {
		return fmt.Errorf("failed to close migration source: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("failed to close database connection: %w", dbErr)
	}
	return nil
}

// MigrateTo migrates to a specific version.
func (m *Migrator) MigrateTo(version uint) error {
	m.logger.Info().Uint("target_version", version).Msg("Migrating to specific version...")

	err := m.migrate.Migrate(version)
	if err != nil {
		if err == migrate.ErrNoChange {
			m.logger.Info().Msg("Already at target version")
			return nil
		}
		return fmt.Errorf("failed to migrate to version %d: %w", version, err)
	}

	m.logger.Info().Uint("version", version).Msg("Migration to specific version completed")
	return nil
}

// Force forces a migration version (use with caution).
func (m *Migrator) Force(version int) error {
	m.logger.Warn().Int("version", version).Msg("Forcing migration version (use with caution)")

	if err := m.migrate.Force(version); err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}

	m.logger.Info().Int("version", version).Msg("Migration version forced successfully")
	return nil
}

// Steps applies n migrations (positive for up, negative for down).
func (m *Migrator) Steps(n int) error {
	m.logger.Info().Int("steps", n).Msg("Applying migration steps...")

	err := m.migrate.Steps(n)
	if err != nil {
		if err == migrate.ErrNoChange {
			m.logger.Info().Msg("No migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to apply %d migration steps: %w", n, err)
	}

	m.logger.Info().Int("steps", n).Msg("Migration steps applied successfully")
	return nil
}
