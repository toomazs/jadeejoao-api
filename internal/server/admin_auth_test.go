package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/jadeejoao/jadeejoao-api/internal/content"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

type fakeContentRepo struct{}

func (fakeContentRepo) ListSections(context.Context) ([]content.Row, error) {
	return []content.Row{{Slug: "hero", Payload: []byte(`{"title":"Jade & João"}`), Enabled: true}}, nil
}

func (fakeContentRepo) UpdateSection(_ context.Context, slug string, payload []byte, enabled bool) (content.Row, error) {
	return content.Row{Slug: slug, Payload: payload, Enabled: enabled}, nil
}

type fakeAuth struct {
	err error
}

func (f fakeAuth) ValidateBearer(context.Context, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "jade@example.com", nil
}

func newAuthedAPI(t *testing.T, authErr error) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	Register(api, Deps{
		Content: content.NewService(fakeContentRepo{}),
		Auth:    fakeAuth{err: authErr},
	})
	return api
}

func TestAdminRoutesRequireAuth(t *testing.T) {
	// Authenticated: 200.
	api := newAuthedAPI(t, nil)
	if resp := api.Get("/api/v1/admin/sections", "Authorization: Bearer token"); resp.Code != http.StatusOK {
		t.Fatalf("authed admin = %d: %s", resp.Code, resp.Body.String())
	}

	// Invalid token: 401.
	api = newAuthedAPI(t, platform.ErrUnauthorized)
	if resp := api.Get("/api/v1/admin/sections"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed admin = %d, want 401", resp.Code)
	}

	// Valid token, not the couple: 403.
	api = newAuthedAPI(t, platform.ErrForbidden)
	if resp := api.Get("/api/v1/admin/sections"); resp.Code != http.StatusForbidden {
		t.Fatalf("forbidden admin = %d, want 403", resp.Code)
	}

	// Public routes are never behind auth.
	api = newAuthedAPI(t, platform.ErrUnauthorized)
	if resp := api.Get("/api/v1/content"); resp.Code != http.StatusOK {
		t.Fatalf("public content = %d, want 200 even with broken auth", resp.Code)
	}
}
