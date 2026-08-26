package messages

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
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
		Description: "Every message, newest first. There is nothing to filter by: the guestbook " +
			"is write-only in public (AD-14), so a message is never shown to anyone but them.",
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
