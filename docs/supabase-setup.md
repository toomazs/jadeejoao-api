# Supabase manual setup checklist

One-time dashboard steps for the project backing `jadeejoao-api`. The API is
the **single writer** — browsers never talk to Supabase directly except for
the admin login handshake and public CDN image loads.

## 1. Project

- Create the project (pick the region closest to Railway; note it for
  `STORAGE_S3_REGION`). Postgres 17 is the default.
- `SUPABASE_URL` = Settings → API → Project URL.

## 2. Close the Data API (PostgREST)

Settings → API → **Data API**: remove every exposed schema (or disable the
Data API entirely). No table is ever served by PostgREST — all data flows
through this API.

Defense-in-depth: migration `00001` also enables **RLS with zero policies on
every table**, so even if the Data API were re-exposed by accident, the anon
key that ships inside the admin SPA bundle opens nothing. Never add RLS
policies to this project.

## 3. Database connection

Connect → **Session pooler** (Supavisor, IPv4, port **5432**) → copy the
string into `DATABASE_URL`.

- Do **not** use the direct connection string: it is IPv6-only and Railway
  has no outbound IPv6.
- Do **not** use transaction mode (port 6543): it breaks pgx prepared
  statements.

Migrations run automatically at API startup (goose, embedded, under an
advisory lock) — nothing to run by hand.

## 4. Storage

- Storage → create bucket **`site-media`**, marked **Public**.
- Storage → Settings → **S3 access keys** → create a key pair →
  `STORAGE_S3_ACCESS_KEY` / `STORAGE_S3_SECRET_KEY`.
- Endpoint and region: shown on the same page (endpoint defaults to
  `{SUPABASE_URL}/storage/v1/s3` in the API config).

Only couple-managed content images live here. Brand assets (logo, seriguela
leaf, fonts) ship inside the frontend repos.

## 5. Auth (admin login)

- Authentication → Users → **create the two accounts** (Jade and João,
  email + password). Put both emails in `ADMIN_EMAILS`.
- Authentication → **Signing Keys** → migrate to **asymmetric** keys
  (ECC P-256 recommended). The API validates JWTs locally against
  `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` — no per-request Supabase
  round-trip.
- Authentication → Providers: leave only Email enabled. Disable sign-ups
  (Authentication → Settings → disable new user sign-ups) so nobody else can
  create an account.

## 6. Before invitations go out

- **Upgrade to Pro** (or schedule `pg_dump` + keep-alive): the Free tier has
  no automated backups and pauses projects after ~1 week idle.
- Storage objects are never inside DB backups — export the bucket separately
  if you want belt and suspenders.
