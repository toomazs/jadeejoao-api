// Command api is the jadeejoao-api entrypoint: config → migrations → pool →
// HTTP server with graceful shutdown.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // America/Sao_Paulo must resolve inside slim containers

	"github.com/jadeejoao/jadeejoao-api/db"
	"github.com/jadeejoao/jadeejoao-api/internal/content"
	"github.com/jadeejoao/jadeejoao-api/internal/guests"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
	"github.com/jadeejoao/jadeejoao-api/internal/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	platform.LoadDotEnv(".env")
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	if err := platform.Migrate(ctx, cfg.DatabaseURL, migrations); err != nil {
		return err
	}

	pool, err := platform.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	contentSvc := content.NewService(content.NewRepo(pool))
	deps := server.Deps{
		Pool:    pool,
		Content: contentSvc,
		Guests:  guests.NewService(guests.NewRepo(pool), contentSvc, nil),
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.NewRouter(cfg, deps),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "port", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}
