package messages

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

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
	}
}

// PublicMessage is one entry as the wall shows it.
//
// No group_id. Which invitation somebody belongs to is the couple's business,
// and this list is served to anyone who opens the site.
type PublicMessage struct {
	MessageID  string `json:"message_id" format:"uuid"`
	AuthorName string `json:"author_name"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at" format:"date-time"`
}

// MessageWallOutput is the wall, newest first.
type MessageWallOutput struct {
	Body struct {
		Messages []PublicMessage `json:"messages"`
	}
}

// RegisterPublic mounts the public guestbook: guests write, and what they
// write goes up for everyone to read (AD-14).
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-messages",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/messages",
		Summary:     "Read the guestbook wall",
		Description: "The most recent messages guests have left, newest first, for the wall on the site.",
		Tags:        []string{"messages"},
	}, func(ctx context.Context, _ *struct{}) (*MessageWallOutput, error) {
		found, err := svc.ListPublic(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "list public messages failed", "error", err)
			return nil, huma.Error500InternalServerError("Não conseguimos carregar os recados agora.")
		}
		out := &MessageWallOutput{}
		// Never nil: an empty wall must serialize as [] and not null, or the
		// site has to defend against both.
		out.Body.Messages = make([]PublicMessage, 0, len(found))
		for _, m := range found {
			out.Body.Messages = append(out.Body.Messages, PublicMessage{
				MessageID:  m.ID.String(),
				AuthorName: m.AuthorName,
				Body:       m.Body,
				CreatedAt:  m.CreatedAt.Format(time.RFC3339),
			})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-message",
		Method:        http.MethodPost,
		Path:          platform.APIBase + "/messages",
		Summary:       "Leave a message for the couple",
		Description:   "Creates a pending guestbook message (Recado aos noivos). Publicly write-only: the site never shows a message back, and the couple reads them in the panel.",
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
		if errors.Is(err, ErrUnknownGroup) {
			return nil, huma.Error422UnprocessableEntity("Grupo de convidados inválido. Refaça a busca pelo seu nome e tente novamente.")
		}
		if err != nil {
			slog.ErrorContext(ctx, "create message failed", "error", err)
			return nil, huma.Error500InternalServerError("Não conseguimos salvar seu recado agora. Tente novamente em instantes.")
		}
		out := &MessageCreatedOutput{}
		out.Body.MessageID = msg.ID.String()
		return out, nil
	})
}
