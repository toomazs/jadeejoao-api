package main

import (
	"context"
	"io/fs"
	"log"

	"github.com/jadeejoao/jadeejoao-api/db"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

func main() {
	platform.LoadDotEnv(".env")
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	m, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		log.Fatal(err)
	}
	if err := platform.Migrate(context.Background(), cfg.DatabaseURL, m); err != nil {
		log.Fatal(err)
	}
	log.Println("migrations aplicadas")
}
