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
	m := Message{ID: uuid.New(), GroupID: groupID, AuthorName: authorName, Body: body}
	f.inserted = append(f.inserted, m)
	return m, nil
}

func (f *fakeRepo) List(_ context.Context) ([]Message, error) {
	return f.inserted, nil
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
