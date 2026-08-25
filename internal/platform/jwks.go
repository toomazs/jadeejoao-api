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

// AdminClaims is what a valid admin token tells us about who is calling.
type AdminClaims struct {
	Email string
	// UserID is the Supabase `sub`: needed to change that user's password
	// through the Admin API without trusting anything the browser sent.
	UserID string
	// MustChangePassword comes from app_metadata, which the user themselves
	// cannot write — unlike user_metadata. That is the whole point: a guest
	// of the panel must not be able to clear their own obligation.
	MustChangePassword bool
}

// AuthValidator validates Supabase Auth JWTs locally against the project
// JWKS (asymmetric signing keys) and enforces the ADMIN_EMAILS allowlist.
// No Supabase round-trip per request: keys are cached and refreshed in the
// background (AD-9).
type AuthValidator struct {
	jwksURL       string
	issuer        string
	allowedEmails map[string]bool

	mu      sync.Mutex
	keyfunc jwt.Keyfunc
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

// ValidateBearer checks an Authorization header value and returns who is
// calling. Errors are ErrUnauthorized (bad/absent token — the admin SPA must
// re-authenticate; there is no grace period) or ErrForbidden (authenticated
// but not the couple).
func (v *AuthValidator) ValidateBearer(_ context.Context, authorization string) (AdminClaims, error) {
	raw, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || raw == "" {
		return AdminClaims{}, fmt.Errorf("%w: missing bearer token", ErrUnauthorized)
	}

	kf, err := v.keyfuncLazy()
	if err != nil {
		return AdminClaims{}, fmt.Errorf("%w: jwks unavailable: %s", ErrUnauthorized, err)
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(raw, claims, kf,
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience("authenticated"),
	)
	if err != nil {
		return AdminClaims{}, fmt.Errorf("%w: %s", ErrUnauthorized, err)
	}

	email, _ := claims["email"].(string)
	if email == "" || !v.allowedEmails[strings.ToLower(email)] {
		return AdminClaims{}, fmt.Errorf("%w: %s is not an admin", ErrForbidden, email)
	}
	sub, _ := claims["sub"].(string)
	must := false
	if app, ok := claims["app_metadata"].(map[string]any); ok {
		must, _ = app["must_change_password"].(bool)
	}
	return AdminClaims{Email: email, UserID: sub, MustChangePassword: must}, nil
}

// keyfuncLazy initializes the JWKS client on first use and RETRIES on later
// calls if that failed — a transient fetch error (cold-start DNS blip) must
// never latch admin auth into a permanent 401.
func (v *AuthValidator) keyfuncLazy() (jwt.Keyfunc, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keyfunc != nil {
		return v.keyfunc, nil
	}
	kf, err := keyfunc.NewDefault([]string{v.jwksURL})
	if err != nil {
		return nil, err
	}
	v.keyfunc = kf.Keyfunc
	return v.keyfunc, nil
}
