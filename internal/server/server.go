// Package server wires the Huma API, chi router, and cross-cutting middleware.
// It contains no business logic: modules register their own operations.
package server

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/content"
	"github.com/jadeejoao/jadeejoao-api/internal/guests"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// Deps carries the service instances handlers close over. All fields are
// nil-able handles, so the API can be constructed with zero values for spec
// export and handler tests.
type Deps struct {
	Pool    *pgxpool.Pool
	Content *content.Service
	Guests  *guests.Service
}

// NewRouter builds the production HTTP handler: middleware stack + Huma API.
func NewRouter(cfg *platform.Config, deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(requestID)
	r.Use(logRequests)
	r.Use(recoverPanics)
	r.Use(cors(cfg.CORSOrigins))
	NewAPI(r, deps)
	return r
}

// NewAPI mounts every operation (and the /docs UI + /openapi.yaml spec,
// served by Huma) onto the given router.
func NewAPI(r chi.Router, deps Deps) huma.API {
	config := huma.DefaultConfig("Jade & João Wedding API", "1.0.0")
	config.Info.Description = "Backend for the wedding website of Jade & João — " +
		"August 7, 2027, Atibaia-SP, Brazil. Single writer over Supabase " +
		"(Postgres + Storage). Public surface serves the one-page site; " +
		"/api/v1/admin/** requires a Supabase Auth JWT. All guest-facing " +
		"messages are in Brazilian Portuguese."
	api := humachi.New(r, config)
	Register(api, deps)
	return api
}

// NewSpecAPI builds the API with zero dependencies. Used by cmd/openapi to
// export openapi.yaml and by the spec-sync test; handlers are never invoked.
func NewSpecAPI() huma.API {
	return NewAPI(chi.NewRouter(), Deps{})
}
