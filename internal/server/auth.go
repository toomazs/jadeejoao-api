package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// AdminAuthenticator validates an Authorization header and returns who is
// calling. Implemented by platform.AuthValidator; faked in tests.
type AdminAuthenticator interface {
	ValidateBearer(ctx context.Context, authorization string) (platform.AdminClaims, error)
}

type claimsKey struct{}

// AdminFromContext returns the admin behind the current request. Only ever
// populated by adminAuthMiddleware, so a handler reading it is reading a token
// the API itself verified — never a header the browser chose.
func AdminFromContext(ctx context.Context) (platform.AdminClaims, bool) {
	c, ok := ctx.Value(claimsKey{}).(platform.AdminClaims)
	return c, ok
}

// PasswordChangeOperationID is the one admin operation reachable while an
// account still carries its temporary password. Named here rather than in the
// guests/content packages because the middleware is what has to know it.
const PasswordChangeOperationID = "change-admin-password"

// adminAuthMiddleware guards every /api/v1/admin/** operation: valid Supabase
// JWT plus email in the ADMIN_EMAILS allowlist. 401 responses carry no grace —
// the admin SPA treats them as "re-authenticate".
//
// It also enforces the first-login password change. Doing it here, rather than
// in the panel, is the difference between an obligation and a suggestion: the
// screen can be skipped by anyone holding the token, this cannot.
func adminAuthMiddleware(api huma.API, auth AdminAuthenticator) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if auth == nil {
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		claims, err := auth.ValidateBearer(ctx.Context(), ctx.Header("Authorization"))
		switch {
		case errors.Is(err, platform.ErrForbidden):
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "Acesso restrito aos noivos.")
			return
		case err != nil:
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "Sessão inválida ou expirada. Entre novamente.")
			return
		}

		if claims.MustChangePassword && ctx.Operation().OperationID != PasswordChangeOperationID {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden,
				"Antes de usar o painel, troque a senha temporária.")
			return
		}

		next(huma.WithValue(ctx, claimsKey{}, claims))
	}
}
