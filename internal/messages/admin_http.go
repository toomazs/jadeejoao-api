package messages

import (
	"context"
	"errors"
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
	Status     string    `json:"status" enum:"pending,approved,rejected"`
	CreatedAt  time.Time `json:"created_at"`
}

// MessagesOutput lists guestbook entries.
type MessagesOutput struct {
	Body struct {
		Messages []MessageView `json:"messages"`
	}
}

// ListMessagesInput filters by status.
type ListMessagesInput struct {
	Status string `query:"status" enum:"pending,approved,rejected" required:"false" doc:"Filter by status; omit for all."`
}

// ModerateMessageInput approves or rejects one message.
type ModerateMessageInput struct {
	MessageID string `path:"message_id" format:"uuid"`
	Body      struct {
		Status string `json:"status" enum:"approved,rejected"`
	}
}

// ModeratedMessageOutput wraps the updated message.
type ModeratedMessageOutput struct {
	Body MessageView
}

// RegisterAdmin mounts the moderation surface on the (already authenticated)
// admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-messages",
		Method:      http.MethodGet,
		Path:        "/messages",
		Summary:     "List guestbook messages",
		Tags:        []string{"messages"},
	}, func(ctx context.Context, in *ListMessagesInput) (*MessagesOutput, error) {
		items, err := svc.List(ctx, in.Status)
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao listar os recados", err)
		}
		out := &MessagesOutput{}
		out.Body.Messages = make([]MessageView, len(items))
		for i, m := range items {
			out.Body.Messages[i] = messageView(m)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-moderate-message",
		Method:      http.MethodPatch,
		Path:        "/messages/{message_id}",
		Summary:     "Approve or reject a message",
		Description: "Moderation exists so a future public guestbook wall is purely additive (AD-14).",
		Tags:        []string{"messages"},
	}, func(ctx context.Context, in *ModerateMessageInput) (*ModeratedMessageOutput, error) {
		id, err := uuid.Parse(in.MessageID)
		if err != nil {
			return nil, huma.Error404NotFound("Recado não encontrado.")
		}
		updated, err := svc.Moderate(ctx, id, in.Body.Status)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("Recado não encontrado.")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao moderar o recado", err)
		}
		return &ModeratedMessageOutput{Body: messageView(updated)}, nil
	})
}

func messageView(m Message) MessageView {
	v := MessageView{
		MessageID:  m.ID.String(),
		AuthorName: m.AuthorName,
		Body:       m.Body,
		Status:     m.Status,
		CreatedAt:  m.CreatedAt,
	}
	if m.GroupID != nil {
		s := m.GroupID.String()
		v.GroupID = &s
	}
	return v
}
