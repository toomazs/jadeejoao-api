package content

import (
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestAdminSectionEndpoints(t *testing.T) {
	repo := &fakeRepo{rows: []Row{
		{Slug: "hero", Payload: []byte(`{"title":"Jade & João","couple_names":"Jade & João","event_datetime":"2027-08-07T15:00:00-03:00","city_label":"Atibaia – SP"}`), Enabled: true},
		{Slug: "our_story", Payload: []byte(`{"title":"Nossa História"}`), Enabled: false},
	}}
	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(repo))

	// Admin list includes disabled sections.
	resp := api.Get("/sections")
	if resp.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"enabled":false`) {
		t.Fatalf("disabled section missing from admin list: %s", resp.Body.String())
	}

	// Update with the matching payload field.
	resp = api.Put("/sections/rsvp", map[string]any{
		"slug":    "rsvp",
		"enabled": true,
		"rsvp":    map[string]any{"title": "Confirme", "deadline": "2027-07-07"},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update = %d: %s", resp.Code, resp.Body.String())
	}
	if repo.updated == nil || repo.updated.Slug != "rsvp" {
		t.Fatalf("repo not updated: %+v", repo.updated)
	}

	// Payload field not matching the slug: 422 PT-BR.
	resp = api.Put("/sections/rsvp", map[string]any{
		"enabled": true,
		"hero": map[string]any{
			"title": "x", "couple_names": "a",
			"event_datetime": "2027-08-07T15:00:00-03:00", "city_label": "c",
		},
	})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatch = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "slug") {
		t.Fatalf("expected mismatch detail: %s", resp.Body.String())
	}

	// Unknown slug is blocked by the path enum.
	resp = api.Put("/sections/not_a_slug", map[string]any{"enabled": true})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad slug = %d, want 422 (enum)", resp.Code)
	}
}
