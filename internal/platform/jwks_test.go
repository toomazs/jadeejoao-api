package platform

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testIssuer = "https://proj.supabase.co/auth/v1"

// newJWKSFixture spins an httptest server serving a JWKS for a fresh ES256
// key, mirroring Supabase's asymmetric signing keys endpoint.
func newJWKSFixture(t *testing.T) (*httptest.Server, *ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kid := "test-key-1"
	b64 := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	jwks := map[string]any{
		"keys": []map[string]any{{
			"kty": "EC", "crv": "P-256", "kid": kid, "use": "sig", "alg": "ES256",
			"x": b64(key.PublicKey.X.FillBytes(make([]byte, 32))),
			"y": b64(key.PublicKey.Y.FillBytes(make([]byte, 32))),
		}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv, key, kid
}

func signToken(t *testing.T, key *ecdsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func adminClaims(email string, expiresIn time.Duration) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":   testIssuer,
		"aud":   "authenticated",
		"email": email,
		"iat":   now.Unix(),
		"exp":   now.Add(expiresIn).Unix(),
	}
}

func TestValidateBearer(t *testing.T) {
	srv, key, kid := newJWKSFixture(t)
	v := NewAuthValidator(srv.URL, testIssuer, []string{"jade@example.com", "Joao@Example.com"})
	ctx := context.Background()

	// Happy path, including case-insensitive allowlist matching.
	token := signToken(t, key, kid, adminClaims("JADE@example.com", time.Hour))
	email, err := v.ValidateBearer(ctx, "Bearer "+token)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if email != "JADE@example.com" {
		t.Fatalf("email = %q", email)
	}

	// Expired: 401, no grace.
	expired := signToken(t, key, kid, adminClaims("jade@example.com", -time.Minute))
	if _, err := v.ValidateBearer(ctx, "Bearer "+expired); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired: got %v, want ErrUnauthorized", err)
	}

	// Valid signature but not an admin: 403.
	stranger := signToken(t, key, kid, adminClaims("intruso@example.com", time.Hour))
	if _, err := v.ValidateBearer(ctx, "Bearer "+stranger); !errors.Is(err, ErrForbidden) {
		t.Fatalf("stranger: got %v, want ErrForbidden", err)
	}

	// Wrong issuer rejected.
	bad := adminClaims("jade@example.com", time.Hour)
	bad["iss"] = "https://evil.example.com"
	if _, err := v.ValidateBearer(ctx, "Bearer "+signToken(t, key, kid, bad)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong issuer: got %v, want ErrUnauthorized", err)
	}

	// Garbage and missing headers.
	if _, err := v.ValidateBearer(ctx, "Bearer not.a.jwt"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("garbage: got %v, want ErrUnauthorized", err)
	}
	if _, err := v.ValidateBearer(ctx, ""); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("missing: got %v, want ErrUnauthorized", err)
	}
}

func TestValidateBearerRejectsForeignKey(t *testing.T) {
	srv, _, kid := newJWKSFixture(t)
	v := NewAuthValidator(srv.URL, testIssuer, []string{"jade@example.com"})

	// Token signed by a DIFFERENT key but claiming the served kid.
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	forged := signToken(t, otherKey, kid, adminClaims("jade@example.com", time.Hour))
	if _, err := v.ValidateBearer(context.Background(), "Bearer "+forged); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("forged signature: got %v, want ErrUnauthorized", err)
	}
}
