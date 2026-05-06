package db

import (
	"embed"
	"errors"
	"fmt"
	"strings"
	// registers the "pgx5" driver

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(databaseURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("build migration source: %w", err)
	}

	// 2. Build the migrate instance.
	//    The "iofs" first arg is the source name; "pgx5://..." is the destination.
	//    NOTE: the URL scheme must be pgx5, not postgres.
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+stripScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	// 3. Run all pending up migrations.
	//    migrate.ErrNoChange is returned when there's nothing to do -- not an error.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func stripScheme(url string) string {
	parsedUrl := strings.TrimPrefix(url, "postgres://")
	parsedUrl = strings.TrimPrefix(parsedUrl, "postgresql://")
	return parsedUrl
}
