// Command migrate runs the pending migrations and stops.
//
// The API runs them at startup, which is right for a deploy and awkward
// everywhere else: applying one locally meant restarting a server somebody was
// using, and the alternative — reaching into the database by hand — is how a
// schema and its migration history stop agreeing.
package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/signal"

	"github.com/jadeejoao/jadeejoao-api/db"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		log.Fatalf("migrations: %v", err)
	}
	if err := platform.Migrate(ctx, cfg.DatabaseURL, migrations); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations em dia")
}
