package messages

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
)

type fakeRepo struct {
	inserted []Message
	// What the service asked the wall to be capped at.
	publicLimit int32
}

func (f *fakeRepo) Insert(_ context.Context, groupID *uuid.UUID, authorName, body string) (Message, error) {
	m := Message{ID: uuid.New(), GroupID: groupID, AuthorName: authorName, Body: body}
	f.inserted = append(f.inserted, m)
	return m, nil
}

func (f *fakeRepo) List(_ context.Context) ([]Message, error) {
	return f.inserted, nil
}

func (f *fakeRepo) ListPublic(_ context.Context, limit int32) ([]Message, error) {
	f.publicLimit = limit
	if int(limit) < len(f.inserted) {
		return f.inserted[:limit], nil
	}
	return f.inserted, nil
}

func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, m := range f.inserted {
		if m.ID == id {
			f.inserted = append(f.inserted[:i], f.inserted[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func TestCreateMessageEndpoint(t *testing.T) {
	repo := &fakeRepo{}
	_, api := humatest.New(t)
	RegisterPublic(api, NewService(repo, nil))

	resp := api.Post("/api/v1/messages", map[string]any{
		"author_name": "Eduardo Silva",
		"body":        "Felicidades aos noivos!",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.Code, resp.Body.String())
	}
	// The response carries the id and nothing else about the message's fate:
	// there is no fate. It is written down and the couple reads it.
	if strings.Contains(resp.Body.String(), `"status"`) {
		t.Fatalf("a message has no status to report: %s", resp.Body.String())
	}
	if len(repo.inserted) != 1 || repo.inserted[0].AuthorName != "Eduardo Silva" {
		t.Fatalf("unexpected insert state: %+v", repo.inserted)
	}

	// Attribution is mandatory.
	resp = api.Post("/api/v1/messages", map[string]any{"body": "anônimo"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing author = %d, want 422", resp.Code)
	}

	// Body is mandatory.
	resp = api.Post("/api/v1/messages", map[string]any{"author_name": "Eduardo"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing body = %d, want 422", resp.Code)
	}
}

// TestWallNeverLeaksTheInvitation guards the one thing the public list must not
// carry. The admin view has group_id because the couple may want to know who
// wrote; the wall is served to anyone who opens the site, and which invitation
// a guest belongs to is not theirs to learn.
func TestWallNeverLeaksTheInvitation(t *testing.T) {
	group := uuid.New()
	repo := &fakeRepo{}
	_, _ = repo.Insert(context.Background(), &group, "Tia Taninha", "Felicidades!")

	_, api := humatest.New(t)
	RegisterPublic(api, NewService(repo, nil))

	resp := api.Get("/api/v1/messages")
	if resp.Code != 200 {
		t.Fatalf("esperava 200, veio %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if strings.Contains(body, group.String()) || strings.Contains(body, "group_id") {
		t.Fatalf("o mural devolveu o convite: %s", body)
	}
	if !strings.Contains(body, "Tia Taninha") || !strings.Contains(body, "Felicidades!") {
		t.Fatalf("faltou o recado: %s", body)
	}
}

// TestWallIsCapped: the page draws every message it is handed, so the cap is
// what keeps it from growing heavier every month on its own.
func TestWallIsCapped(t *testing.T) {
	repo := &fakeRepo{}
	_, api := humatest.New(t)
	RegisterPublic(api, NewService(repo, nil))

	if resp := api.Get("/api/v1/messages"); resp.Code != 200 {
		t.Fatalf("esperava 200, veio %d", resp.Code)
	}
	if repo.publicLimit != PublicWallSize {
		t.Fatalf("esperava teto de %d, pediu %d", PublicWallSize, repo.publicLimit)
	}
}

// TestEmptyWallIsAList: an empty wall has to serialize as [] and not null, or
// the site needs to defend against both shapes.
func TestEmptyWallIsAList(t *testing.T) {
	_, api := humatest.New(t)
	RegisterPublic(api, NewService(&fakeRepo{}, nil))

	body := api.Get("/api/v1/messages").Body.String()
	if !strings.Contains(body, `"messages":[]`) {
		t.Fatalf("esperava uma lista vazia, veio %s", body)
	}
}

func TestAdminDeletesAMessage(t *testing.T) {
	repo := &fakeRepo{}
	written, _ := repo.Insert(context.Background(), nil, "Alguém", "apaga isso")

	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(repo, nil))

	if resp := api.Delete("/messages/" + written.ID.String()); resp.Code != 204 {
		t.Fatalf("esperava 204, veio %d: %s", resp.Code, resp.Body.String())
	}
	if len(repo.inserted) != 0 {
		t.Fatalf("o recado continua lá: %d", len(repo.inserted))
	}
	// Twice is not an error the couple should see as a failure of theirs, but
	// it is a 404: the second click found nothing to do.
	if resp := api.Delete("/messages/" + written.ID.String()); resp.Code != 404 {
		t.Fatalf("esperava 404 na segunda vez, veio %d", resp.Code)
	}
}

func TestAdminRejectsAMalformedID(t *testing.T) {
	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(&fakeRepo{}, nil))

	if resp := api.Delete("/messages/nao-e-uuid"); resp.Code != 422 {
		t.Fatalf("esperava 422, veio %d", resp.Code)
	}
}
