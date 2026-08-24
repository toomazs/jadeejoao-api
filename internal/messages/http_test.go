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
}

func (f *fakeRepo) Insert(_ context.Context, groupID *uuid.UUID, authorName, body string) (Message, error) {
	m := Message{ID: uuid.New(), GroupID: groupID, AuthorName: authorName, Body: body, Status: "pending"}
	f.inserted = append(f.inserted, m)
	return m, nil
}

func (f *fakeRepo) List(_ context.Context, statusFilter string) ([]Message, error) {
	var out []Message
	for _, m := range f.inserted {
		if statusFilter == "" || m.Status == statusFilter {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) (Message, error) {
	for i := range f.inserted {
		if f.inserted[i].ID == id {
			f.inserted[i].Status = status
			return f.inserted[i], nil
		}
	}
	return Message{}, ErrNotFound
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
	if !strings.Contains(resp.Body.String(), `"status":"pending"`) {
		t.Fatalf("message must start pending: %s", resp.Body.String())
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
