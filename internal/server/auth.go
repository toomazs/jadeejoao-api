package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// AdminAuthenticator validates an Authorization header and returns the admin
// email. Implemented by platform.AuthValidator; faked in tests.
type AdminAuthenticator interface {
	ValidateBearer(ctx context.Context, authorization string) (string, error)
}

// adminAuthMiddleware guards every /api/v1/admin/** operation: valid Supabase
// JWT plus email in the ADMIN_EMAILS allowlist. 401 responses carry no grace —
// the admin SPA treats them as "re-authenticate".
func adminAuthMiddleware(api huma.API, auth AdminAuthenticator) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if auth == nil {
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		_, err := auth.ValidateBearer(ctx.Context(), ctx.Header("Authorization"))
		switch {
		case errors.Is(err, platform.ErrForbidden):
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "Acesso restrito aos noivos.")
			return
		case err != nil:
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "Sessão inválida ou expirada. Entre novamente.")
			return
		}
		next(ctx)
	}
}
