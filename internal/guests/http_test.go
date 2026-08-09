package guests

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func newTestAPI(t *testing.T, repo *fakeRepo, deadline string) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterPublic(api, NewService(repo, fixedDeadline(deadline), at("2027-01-01 10:00")))
	return api
}

func TestLookupEndpoint(t *testing.T) {
	repo, _, _ := newFixture()
	api := newTestAPI(t, repo, "2027-07-07")

	resp := api.Post("/api/v1/guests/lookup", map[string]any{"full_name": "eduardo silva"})
	if resp.Code != http.StatusOK {
		t.Fatalf("lookup = %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "Eduardo e família") || !strings.Contains(body, `"attending":"pending"`) {
		t.Fatalf("unexpected body: %s", body)
	}

	resp = api.Post("/api/v1/guests/lookup", map[string]any{"full_name": "Fulano de Tal"})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("miss = %d, want 404", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); !strings.Contains(ct, "application/problem+json") {
		t.Fatalf("miss content-type = %q, want problem+json", ct)
	}
	if !strings.Contains(resp.Body.String(), "fale com os noivos") {
		t.Fatalf("problem detail must be PT-BR: %s", resp.Body.String())
	}
}

func TestRSVPEndpoint(t *testing.T) {
	repo, m1, m2 := newFixture()
	api := newTestAPI(t, repo, "2027-07-07")
	path := fmt.Sprintf("/api/v1/guests/%s/rsvp", repo.group.ID)

	resp := api.Post(path, map[string]any{"responses": []map[string]any{
		{"guest_id": m1.String(), "attending": "yes"},
		{"guest_id": m2.String(), "attending": "no"},
	}})
	if resp.Code != http.StatusOK {
		t.Fatalf("rsvp = %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"attending":"yes"`) {
		t.Fatalf("refreshed group missing answers: %s", resp.Body.String())
	}

	// Invalid enum rejected by schema validation before the service runs.
	resp = api.Post(path, map[string]any{"responses": []map[string]any{
		{"guest_id": m1.String(), "attending": "maybe"},
		{"guest_id": m2.String(), "attending": "no"},
	}})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad enum = %d, want 422", resp.Code)
	}

	// Incomplete coverage rejected by the service.
	resp = api.Post(path, map[string]any{"responses": []map[string]any{
		{"guest_id": m1.String(), "attending": "yes"},
	}})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("partial = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "convidados do seu grupo") {
		t.Fatalf("problem detail must be PT-BR: %s", resp.Body.String())
	}
}

func TestRSVPDeadlineEndpoint(t *testing.T) {
	repo, m1, m2 := newFixture()
	_, api := humatest.New(t)
	// Fixed clock after the deadline.
	RegisterPublic(api, NewService(repo, fixedDeadline("2027-07-07"), at("2027-07-08 08:00")))

	resp := api.Post(fmt.Sprintf("/api/v1/guests/%s/rsvp", repo.group.ID), map[string]any{"responses": []map[string]any{
		{"guest_id": m1.String(), "attending": "yes"},
		{"guest_id": m2.String(), "attending": "no"},
	}})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("late rsvp = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "prazo") {
		t.Fatalf("expected PT-BR deadline message: %s", resp.Body.String())
	}
}
