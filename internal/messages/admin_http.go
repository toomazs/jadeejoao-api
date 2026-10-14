package messages

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// MessageView is one guestbook entry as the admin sees it.
type MessageView struct {
	MessageID  string    `json:"message_id" format:"uuid"`
	GroupID    *string   `json:"group_id,omitempty" format:"uuid"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// MessagesOutput lists guestbook entries.
type MessagesOutput struct {
	Body struct {
		Messages []MessageView `json:"messages"`
	}
}

// RegisterAdmin mounts the couple's reading surface on the (already
// authenticated) admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-messages",
		Method:      http.MethodGet,
		Path:        "/messages",
		Summary:     "List guestbook messages",
		Description: "Every message, newest first — including the ones beyond what the wall on " +
			"the site shows. There is nothing to filter by: a message is either there or deleted.",
		Tags: []string{"messages"},
	}, func(ctx context.Context, _ *struct{}) (*MessagesOutput, error) {
		items, err := svc.List(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "request failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao listar os recados")
		}
		out := &MessagesOutput{}
		out.Body.Messages = make([]MessageView, len(items))
		for i, m := range items {
			out.Body.Messages[i] = messageView(m)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-message",
		Method:      http.MethodDelete,
		Path:        "/messages/{message_id}",
		Summary:     "Delete a guestbook message",
		Description: "Removes a message for good — from the wall on the site and from this list. " +
			"The couple's only control over a guestbook that publishes as it is written, so it " +
			"is immediate and there is no undo.",
		Tags:          []string{"messages"},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *struct {
		MessageID string `path:"message_id" format:"uuid"`
	}) (*struct{}, error) {
		id, err := uuid.Parse(in.MessageID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de recado inválido.")
		}
		err = svc.Delete(ctx, id)
		if errors.Is(err, ErrNotFound) {
			// Already gone is the outcome that was asked for. Saying so plainly
			// beats an error for two people clicking the same row.
			return nil, huma.Error404NotFound("Esse recado não existe mais.")
		}
		if err != nil {
			slog.ErrorContext(ctx, "delete message failed", "error", err)
			return nil, huma.Error500InternalServerError("Não conseguimos apagar o recado agora.")
		}
		return nil, nil
	})
}

func messageView(m Message) MessageView {
	v := MessageView{
		MessageID:  m.ID.String(),
		AuthorName: m.AuthorName,
		Body:       m.Body,
		CreatedAt:  m.CreatedAt,
	}
	if m.GroupID != nil {
		s := m.GroupID.String()
		v.GroupID = &s
	}
	return v
}
