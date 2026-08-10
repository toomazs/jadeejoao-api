# jadeejoao-api

Go backend for the wedding website of **Jade & João** — August 7, 2027,
Atibaia-SP. Repo 1 of 3: this API **owns the cross-repo contract**. The two
React SPAs (`jadeejoao-frontend`, `jadeejoao-admin`) generate their clients
from the [`openapi.yaml`](openapi.yaml) this service emits — spec quality is a
first-class deliverable.

- **Stack:** Go 1.26 · [Huma v2](https://huma.rocks) on chi v5 (code-first
  OpenAPI 3.1) · Supabase Postgres 17 via Supavisor session pooler · sqlc +
  pgx v5 · goose migrations (embedded, run at startup) · Supabase Storage via
  the S3-compatible API · Railway (single replica).
- **Docs:** interactive at `/docs`, raw spec at `/openapi.yaml`, health at
  `/healthz`.
- Identifiers, comments, commits and docs are English; every guest-facing
  string is Brazilian Portuguese.

## Architecture in one breath

Modular monolith, one module per domain — `content`, `guests`, `gifts`,
`messages`, `media`, `importer` — each layered transport → service →
repository (sqlc). Cross-module calls are service→service only. Shared
technical concerns (config, db pool, storage client, JWKS validation, PIX
codec, rate limiting) live in `internal/platform` and never import domain
modules. Only this API holds Supabase credentials: the Data API is closed and
every table carries deny-all RLS (defense-in-depth, migration `00001`).

Key domain rules: guests have no accounts (normalized-name lookup with a
rate-limited prefix typeahead; group data only via exact match); RSVP is per
guest with a server-enforced deadline, any member answers for the whole group,
and each submission with a "yes" emails the couple via Resend (async, never
blocking the RSVP); gifts are dual-mode — `kind=pix` metas/cotas (progress
always computed from the ledger, last quota under a row lock, hand-rolled
golden-tested PIX codec) or `kind=link` external store registry cards
(Mercado Livre/Amazon/Camicado URL, no money through the API); messages are
publicly write-only; the guest list arrives only as an uploaded CSV/XLSX
export, upserted by normalized name, never touching RSVP fields.

## Local run

Prerequisites: Go 1.26+, a Supabase project prepared per
[docs/supabase-setup.md](docs/supabase-setup.md).

```bash
cp .env.example .env   # fill in the real values
go run ./cmd/api
```

Startup order: config → goose migrations (advisory-locked) → pgx pool → HTTP
server. Open `http://localhost:8080/docs`.

Tests run offline — no database or network needed:

```bash
go test ./...
```

## Regenerating generated artifacts

**OpenAPI spec** (committed; a test fails when it drifts):

```bash
go generate ./...
```

**sqlc queries** (after editing `db/queries/*.sql` or migrations):

```bash
sqlc generate
```

Install sqlc 1.31.x from the [official releases](https://github.com/sqlc-dev/sqlc/releases)
(or `choco install sqlc` / `brew install sqlc`). Note: `go run
github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.0` does **not** work — sqlc's go.mod
carries replace directives — so use a binary install, or clone the repo at
`v1.31.0` and `go build ./cmd/sqlc`.

**Lint** (CI-less for now; `go vet` is the enforced floor):

```bash
go vet ./...
golangci-lint run   # https://golangci-lint.run/docs/welcome/install/
```

## Error language note

Every guest-facing problem detail is PT-BR: business-rule errors (RSVP
deadline, quota amounts, lookup misses, importer reports) and the top-level
detail of schema-validation failures ("Dados inválidos…", via a Huma error
override). The per-field entries inside `errors[]` remain Huma's structured
English output — machine-readable data the SPAs consume, not guest-facing
copy. Internal error details are logged with the request id, never returned.

## Deploying to Railway

1. Push this repo to GitHub and create a Railway service from it — the
   Railpack Go builder detects `cmd/api` automatically.
2. Set every variable from [.env.example](.env.example) (Railway injects
   `PORT` itself).
3. Healthcheck path: `/healthz` (200 only when Postgres answers).
4. Keep **exactly 1 replica**: per-IP rate limiting and the gift-quota row
   lock assume a single process. Scaling out is a documented trigger to
   revisit both.
5. Migrations run on boot; overlapping deploys are safe (Postgres advisory
   lock).

Supabase dashboard steps (close Data API, bucket, the couple's two auth
users, asymmetric signing keys, Free-tier caveats):
[docs/supabase-setup.md](docs/supabase-setup.md).

## Repository layout

```text
cmd/api/            entrypoint: config → migrate → pool → server
cmd/openapi/        spec exporter (go generate target)
internal/platform/  config, db, storage, jwks, pix, ratelimit
internal/server/    huma setup, middleware, route registration, auth
internal/<module>/  payloads, service, repo (+ <module>db sqlc output)
db/migrations/      goose SQL (schema + RLS, PT-BR seeds)
db/queries/         sqlc inputs per module
openapi.yaml        the contract (regenerate with go generate)
docs/               supabase-setup.md
```
