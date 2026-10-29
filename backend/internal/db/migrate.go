package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	postgresmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func newMigrator(sqlDB *sql.DB, migrationsPath string) (*migrate.Migrate, error) {
	driver, err := postgresmigrate.WithInstance(sqlDB, &postgresmigrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("create postgres migrate driver: %w", err)
	}

	sourceURL, err := MigrationSourceURL(migrationsPath)
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("create migrate instance: %w", err)
	}
	return m, nil
}

func RunMigrations(sqlDB *sql.DB, migrationsPath string) error {
	m, err := newMigrator(sqlDB, migrationsPath)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run up migrations: %w", err)
	}

	return nil
}

// RunMigrationSteps applies n up/down steps (negative = down).
func RunMigrationSteps(sqlDB *sql.DB, migrationsPath string, steps int) error {
	m, err := newMigrator(sqlDB, migrationsPath)
	if err != nil {
		return err
	}

	if steps == 0 {
		return nil
	}
	if err := m.Steps(steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migration steps (%d): %w", steps, err)
	}
	return nil
}

// MigrateToVersion moves the schema to the requested migration version.
func MigrateToVersion(sqlDB *sql.DB, migrationsPath string, version uint) error {
	m, err := newMigrator(sqlDB, migrationsPath)
	if err != nil {
		return err
	}

	if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate to version %d: %w", version, err)
	}
	return nil
}

// MigrationVersion returns the current migration version and dirty flag.
func MigrationVersion(sqlDB *sql.DB, migrationsPath string) (uint, bool, error) {
	m, err := newMigrator(sqlDB, migrationsPath)
	if err != nil {
		return 0, false, err
	}
	return m.Version()
}
