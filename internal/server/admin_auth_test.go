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
	err  error
	must bool // the account still carries its temporary password
}

func (f fakeAuth) ValidateBearer(context.Context, string) (platform.AdminClaims, error) {
	if f.err != nil {
		return platform.AdminClaims{}, f.err
	}
	return platform.AdminClaims{
		Email:              "jade@example.com",
		UserID:             "user-1",
		MustChangePassword: f.must,
	}, nil
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

// A panel screen can be skipped by anyone holding the token. The obligation to
// change the temporary password therefore lives here, where it cannot be.
func TestTemporaryPasswordBlocksEverythingButTheChange(t *testing.T) {
	_, api := humatest.New(t)
	Register(api, Deps{
		Content:       content.NewService(fakeContentRepo{}),
		Auth:          fakeAuth{must: true},
		AdminPassword: stubPasswordChanger{},
	})

	// Every ordinary admin route is closed while the obligation stands.
	for _, route := range []string{"/api/v1/admin/sections", "/api/v1/admin/guests"} {
		resp := api.Get(route, "Authorization: Bearer token")
		if resp.Code != http.StatusForbidden {
			t.Errorf("%s while password is temporary = %d, want 403", route, resp.Code)
		}
	}

	// The way out stays open — otherwise the account is bricked.
	resp := api.Post("/api/v1/admin/password", "Authorization: Bearer token",
		map[string]any{"current_password": "Mudar@1234", "new_password": "umaSenhaBoa123"})
	if resp.Code != http.StatusOK {
		t.Fatalf("password change while obliged = %d: %s", resp.Code, resp.Body.String())
	}
}

func TestPasswordChangeRefusesWeakOrWrongInput(t *testing.T) {
	_, api := humatest.New(t)
	Register(api, Deps{
		Content:       content.NewService(fakeContentRepo{}),
		Auth:          fakeAuth{},
		AdminPassword: stubPasswordChanger{wrongCurrent: true},
	})

	resp := api.Post("/api/v1/admin/password", "Authorization: Bearer token",
		map[string]any{"current_password": "errada", "new_password": "umaSenhaBoa123"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Errorf("wrong current password = %d, want 422", resp.Code)
	}

	// Too short is rejected by the schema before any of this runs.
	resp = api.Post("/api/v1/admin/password", "Authorization: Bearer token",
		map[string]any{"current_password": "Mudar@1234", "new_password": "curta"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Errorf("short password = %d, want 422", resp.Code)
	}
}

type stubPasswordChanger struct{ wrongCurrent bool }

func (s stubPasswordChanger) VerifyPassword(context.Context, string, string) error {
	if s.wrongCurrent {
		return platform.ErrWrongPassword
	}
	return nil
}

func (s stubPasswordChanger) SetPassword(context.Context, string, string) error { return nil }
