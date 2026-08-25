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
	"github.com/jadeejoao/jadeejoao-api/internal/gifts"
	"github.com/jadeejoao/jadeejoao-api/internal/guests"
	"github.com/jadeejoao/jadeejoao-api/internal/importer"
	"github.com/jadeejoao/jadeejoao-api/internal/instagram"
	"github.com/jadeejoao/jadeejoao-api/internal/media"
	"github.com/jadeejoao/jadeejoao-api/internal/messages"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// Deps carries the service instances handlers close over. All fields are
// nil-able handles, so the API can be constructed with zero values for spec
// export and handler tests.
type Deps struct {
	Pool      *pgxpool.Pool
	Content   *content.Service
	Guests    *guests.Service
	Gifts     *gifts.Service
	Messages  *messages.Service
	Media     *media.Service
	Importer  *importer.Service
	Instagram *instagram.Service
	Auth      AdminAuthenticator
	// AdminPassword changes an admin's own password. Nil disables the
	// endpoint, which cmd/openapi relies on to export the spec offline.
	AdminPassword AdminPasswordChanger
}

// Public rate limits, per IP, in-process (single replica). POSTs: generous
// for guests retrying a lookup, hostile to scripted abuse. The suggest GET
// gets its own roomier bucket — one typeahead session fires several debounced
// requests and must not drain the write budget of a shared IP.
const (
	publicPostPerMinute = 10
	publicPostBurst     = 20

	suggestPerMinute = 60
	suggestBurst     = 40
)

// maxRequestSize bounds any request body, chiefly the admin multipart
// uploads (sheet imports, images).
const maxRequestSize = 26 << 20 // 26 MB

// The spec freezes guest-facing messages as PT-BR. Handler-raised problems
// already are; this override localizes the detail Huma itself generates for
// schema-validation failures, keeping the per-field entries as structured
// (machine-readable) data for the SPAs.
func init() {
	origNewError := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		if status == http.StatusUnprocessableEntity && msg == "validation failed" {
			msg = "Dados inválidos. Confira os campos enviados e tente novamente."
		}
		return origNewError(status, msg, errs...)
	}
}

// NewRouter builds the production HTTP handler: middleware stack + Huma API.
func NewRouter(cfg *platform.Config, deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(requestID)
	r.Use(logRequests)
	r.Use(recoverPanics)
	r.Use(cors(cfg.CORSOrigins))
	r.Use(maxRequestBytes(maxRequestSize))
	r.Use(publicRateLimit(
		platform.NewRateLimiter(publicPostPerMinute, publicPostBurst),
		platform.NewRateLimiter(suggestPerMinute, suggestBurst),
	))
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
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"supabaseJWT": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "Supabase Auth access token of one of the couple's accounts (ADMIN_EMAILS).",
		},
	}
	api := humachi.New(r, config)
	Register(api, deps)
	return api
}

// NewSpecAPI builds the API with zero dependencies. Used by cmd/openapi to
// export openapi.yaml and by the spec-sync test; handlers are never invoked.
func NewSpecAPI() huma.API {
	return NewAPI(chi.NewRouter(), Deps{})
}
