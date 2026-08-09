package messages

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// MessageInput creates one guestbook message.
type MessageInput struct {
	Body struct {
		AuthorName string `json:"author_name" minLength:"1" maxLength:"200" example:"Eduardo Silva"`
		Body       string `json:"body" minLength:"1" maxLength:"2000" example:"Felicidades aos noivos!"`
		GroupID    string `json:"group_id,omitempty" format:"uuid" doc:"Guest group, auto-filled by the site after lookup."`
	}
}

// MessageCreatedOutput acknowledges the pending message.
type MessageCreatedOutput struct {
	Body struct {
		MessageID string `json:"message_id" format:"uuid"`
		Status    string `json:"status" enum:"pending,approved,rejected" doc:"New messages always start pending; the couple moderates them in the admin."`
	}
}

// RegisterPublic mounts the write-only public guestbook surface. There is no
// public read endpoint in v1 — the couple reads messages in the admin.
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-message",
		Method:        http.MethodPost,
		Path:          platform.APIBase + "/messages",
		Summary:       "Leave a message for the couple",
		Description:   "Creates a pending guestbook message (Recado aos noivos). Publicly write-only: messages are read and moderated in the admin.",
		Tags:          []string{"messages"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *MessageInput) (*MessageCreatedOutput, error) {
		var groupID *uuid.UUID
		if in.Body.GroupID != "" {
			parsed, err := uuid.Parse(in.Body.GroupID)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
			}
			groupID = &parsed
		}
		msg, err := svc.Create(ctx, groupID, in.Body.AuthorName, in.Body.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("Não conseguimos salvar seu recado agora. Tente novamente em instantes.", err)
		}
		out := &MessageCreatedOutput{}
		out.Body.MessageID = msg.ID.String()
		out.Body.Status = msg.Status
		return out, nil
	})
}
