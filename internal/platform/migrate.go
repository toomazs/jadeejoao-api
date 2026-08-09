package platform

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrate runs all pending goose migrations at startup under a Postgres
// session-level advisory lock, so concurrent boots (deploy overlap) never race.
// It uses a short-lived database/sql connection; application traffic goes
// through pgxpool instead.
func Migrate(ctx context.Context, databaseURL string, migrations fs.FS) error {
	sqldb, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer sqldb.Close()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create session locker: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqldb, migrations,
		goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		slog.Info("migration applied", "source", r.Source.Path, "duration", r.Duration.String())
	}
	if len(results) == 0 {
		slog.Info("migrations up to date")
	}
	return nil
}
