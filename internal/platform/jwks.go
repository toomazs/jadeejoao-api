package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrUnauthorized: missing, malformed, expired, or unverifiable token.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden: valid token, but the email is not an admin.
	ErrForbidden = errors.New("forbidden")
)

// AuthValidator validates Supabase Auth JWTs locally against the project
// JWKS (asymmetric signing keys) and enforces the ADMIN_EMAILS allowlist.
// No Supabase round-trip per request: keys are cached and refreshed in the
// background (AD-9).
type AuthValidator struct {
	jwksURL       string
	issuer        string
	allowedEmails map[string]bool

	once    sync.Once
	keyfunc jwt.Keyfunc
	initErr error
}

// NewAuthValidator builds the validator. The JWKS is fetched lazily on first
// use so offline construction (spec export, tests) never dials out.
func NewAuthValidator(jwksURL, issuer string, adminEmails []string) *AuthValidator {
	allowed := make(map[string]bool, len(adminEmails))
	for _, e := range adminEmails {
		allowed[strings.ToLower(strings.TrimSpace(e))] = true
	}
	return &AuthValidator{jwksURL: jwksURL, issuer: issuer, allowedEmails: allowed}
}

// ValidateBearer checks an Authorization header value and returns the admin
// email. Errors are ErrUnauthorized (bad/absent token — the admin SPA must
// re-authenticate; there is no grace period) or ErrForbidden (authenticated
// but not the couple).
func (v *AuthValidator) ValidateBearer(_ context.Context, authorization string) (string, error) {
	raw, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || raw == "" {
		return "", fmt.Errorf("%w: missing bearer token", ErrUnauthorized)
	}

	v.once.Do(func() {
		kf, err := keyfunc.NewDefault([]string{v.jwksURL})
		if err != nil {
			v.initErr = err
			return
		}
		v.keyfunc = kf.Keyfunc
	})
	if v.initErr != nil {
		return "", fmt.Errorf("%w: jwks unavailable: %s", ErrUnauthorized, v.initErr)
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, v.keyfunc,
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience("authenticated"),
	)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnauthorized, err)
	}

	email, _ := claims["email"].(string)
	if email == "" || !v.allowedEmails[strings.ToLower(email)] {
		return "", fmt.Errorf("%w: %s is not an admin", ErrForbidden, email)
	}
	return email, nil
}
