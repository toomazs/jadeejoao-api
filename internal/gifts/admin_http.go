package gifts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// GiftParamsBody is the admin create/update payload for a gift.
type GiftParamsBody struct {
	Title         string  `json:"title" minLength:"1" maxLength:"200"`
	Description   string  `json:"description,omitempty" maxLength:"2000"`
	ImageURL      *string `json:"image_url,omitempty" format:"uri"`
	Kind          string  `json:"kind,omitempty" enum:"pix,link" doc:"Defaults to pix on create; REQUIRED on update. pix: metas/cotas. link: external registry card (requires platform + https external_url, forbids money fields)."`
	Platform      *string `json:"platform,omitempty" minLength:"1" maxLength:"40" example:"mercadolivre" doc:"Store slug, required for link gifts (mercadolivre, amazon, camicado, …); the SPAs map it to a logo badge."`
	ExternalURL   *string `json:"external_url,omitempty" format:"uri" maxLength:"2000" doc:"https URL of the couple's registry (link gifts only)."`
	GoalCentavos  *int64  `json:"goal_centavos,omitempty" minimum:"1" maximum:"10000000000"`
	QuotaCentavos *int64  `json:"quota_centavos,omitempty" minimum:"1" maximum:"10000000000"`
	MaxUnits      *int32  `json:"max_units,omitempty" minimum:"1" doc:"Requires quota_centavos."`
	Active        bool    `json:"active"`
	Sort          int32   `json:"sort"`
}

// AdminGiftsOutput lists every gift, including deactivated ones.
type AdminGiftsOutput struct {
	Body struct {
		Gifts []GiftView `json:"gifts"`
	}
}

// GiftOutput wraps one gift.
type GiftOutput struct {
	Body GiftView
}

// CreateGiftInput creates a gift.
type CreateGiftInput struct {
	Body GiftParamsBody
}

// UpdateGiftInput replaces a gift's editable fields.
type UpdateGiftInput struct {
	GiftID string `path:"gift_id" format:"uuid"`
	Body   GiftParamsBody
}

// DeleteGiftInput selects a gift.
type DeleteGiftInput struct {
	GiftID string `path:"gift_id" format:"uuid"`
}

// DeleteGiftOutput reports whether the gift was deleted or deactivated.
type DeleteGiftOutput struct {
	Body struct {
		Outcome string `json:"outcome" enum:"deleted,deactivated" doc:"Gifts with contributions are never hard-deleted; they are deactivated."`
	}
}

// ContributionView is one ledger entry for the admin.
type ContributionView struct {
	ContributionID  string     `json:"contribution_id" format:"uuid"`
	GiftID          string     `json:"gift_id" format:"uuid"`
	GiftTitle       string     `json:"gift_title,omitempty"`
	GroupID         *string    `json:"group_id,omitempty" format:"uuid"`
	ContributorName string     `json:"contributor_name"`
	AmountCentavos  int64      `json:"amount_centavos"`
	Status          string     `json:"status" enum:"declared,confirmed,cancelled"`
	CreatedAt       time.Time  `json:"created_at"`
	ConfirmedAt     *time.Time `json:"confirmed_at,omitempty"`
}

// ContributionsOutput lists ledger entries.
type ContributionsOutput struct {
	Body struct {
		Contributions []ContributionView `json:"contributions"`
	}
}

// ListContributionsInput filters the ledger.
type ListContributionsInput struct {
	Status string `query:"status" enum:"declared,confirmed,cancelled" required:"false" doc:"Filter by status; omit for all."`
}

// ModerateContributionInput transitions one ledger entry.
type ModerateContributionInput struct {
	ContributionID string `path:"contribution_id" format:"uuid"`
	Body           struct {
		Status string `json:"status" enum:"confirmed,cancelled" doc:"confirmed: the PIX landed. cancelled: it never will."`
	}
}

// ContributionOutput wraps one updated ledger entry.
type ModeratedContributionOutput struct {
	Body ContributionView
}

// RegisterAdmin mounts the admin gift + contribution surface on the (already
// authenticated) admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-gifts",
		Method:      http.MethodGet,
		Path:        "/gifts",
		Summary:     "List all gifts",
		Description: "Every gift with computed progress, including deactivated ones.",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, _ *struct{}) (*AdminGiftsOutput, error) {
		gifts, err := svc.AdminListGifts(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "admin list gifts failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao listar os presentes")
		}
		out := &AdminGiftsOutput{}
		out.Body.Gifts = make([]GiftView, len(gifts))
		for i, g := range gifts {
			out.Body.Gifts[i] = giftView(g)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-gift",
		Method:        http.MethodPost,
		Path:          "/gifts",
		Summary:       "Create a gift",
		Tags:          []string{"gifts"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *CreateGiftInput) (*GiftOutput, error) {
		gift, err := svc.CreateGift(ctx, giftParams(in.Body))
		if err != nil {
			return nil, mapAdminGiftErr(err)
		}
		return &GiftOutput{Body: giftView(gift)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-gift",
		Method:      http.MethodPut,
		Path:        "/gifts/{gift_id}",
		Summary:     "Update a gift",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, in *UpdateGiftInput) (*GiftOutput, error) {
		id, err := uuid.Parse(in.GiftID)
		if err != nil {
			return nil, huma.Error404NotFound("Presente não encontrado.")
		}
		gift, err := svc.UpdateGift(ctx, id, giftParams(in.Body))
		if err != nil {
			return nil, mapAdminGiftErr(err)
		}
		return &GiftOutput{Body: giftView(gift)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-gift",
		Method:      http.MethodDelete,
		Path:        "/gifts/{gift_id}",
		Summary:     "Delete or deactivate a gift",
		Description: "Hard-deletes only gifts with zero contributions; otherwise deactivates (the ledger is append-only history).",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, in *DeleteGiftInput) (*DeleteGiftOutput, error) {
		id, err := uuid.Parse(in.GiftID)
		if err != nil {
			return nil, huma.Error404NotFound("Presente não encontrado.")
		}
		outcome, err := svc.DeleteGift(ctx, id)
		if err != nil {
			return nil, mapAdminGiftErr(err)
		}
		out := &DeleteGiftOutput{}
		out.Body.Outcome = string(outcome)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-contributions",
		Method:      http.MethodGet,
		Path:        "/contributions",
		Summary:     "List contributions",
		Description: "The append-only ledger, newest first, with gift titles.",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, in *ListContributionsInput) (*ContributionsOutput, error) {
		items, err := svc.ListContributions(ctx, in.Status)
		if err != nil {
			slog.ErrorContext(ctx, "admin list contributions failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao listar as contribuições")
		}
		out := &ContributionsOutput{}
		out.Body.Contributions = make([]ContributionView, len(items))
		for i, c := range items {
			out.Body.Contributions[i] = contributionView(c.Contribution, c.GiftTitle)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-moderate-contribution",
		Method:      http.MethodPatch,
		Path:        "/contributions/{contribution_id}",
		Summary:     "Confirm or cancel a contribution",
		Description: "Only the admin transitions contribution status. Confirming stamps confirmed_at.",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, in *ModerateContributionInput) (*ModeratedContributionOutput, error) {
		id, err := uuid.Parse(in.ContributionID)
		if err != nil {
			return nil, huma.Error404NotFound("Contribuição não encontrada.")
		}
		updated, err := svc.ModerateContribution(ctx, id, in.Body.Status)
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, huma.Error404NotFound("Contribuição não encontrada.")
		case errors.Is(err, ErrInvalidTransition):
			return nil, huma.Error409Conflict("Transição inválida: contribuições canceladas não podem ser reativadas e só contribuições declaradas podem ser confirmadas.")
		case err != nil:
			slog.ErrorContext(ctx, "moderate contribution failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao atualizar a contribuição")
		}
		return &ModeratedContributionOutput{Body: contributionView(updated, "")}, nil
	})
}

func giftParams(b GiftParamsBody) GiftParams {
	return GiftParams(b)
}

func contributionView(c Contribution, giftTitle string) ContributionView {
	v := ContributionView{
		ContributionID:  c.ID.String(),
		GiftID:          c.GiftID.String(),
		GiftTitle:       giftTitle,
		ContributorName: c.ContributorName,
		AmountCentavos:  c.AmountCentavos,
		Status:          c.Status,
		CreatedAt:       c.CreatedAt,
		ConfirmedAt:     c.ConfirmedAt,
	}
	if c.GroupID != nil {
		s := c.GroupID.String()
		v.GroupID = &s
	}
	return v
}

func mapAdminGiftErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("Presente não encontrado.")
	case errors.Is(err, ErrInvalidGift):
		return huma.Error422UnprocessableEntity("Configuração inválida: presentes PIX não levam link/plataforma (e max_units exige quota_centavos); presentes de lista externa exigem external_url https e não levam meta, cota ou max_units.")
	case errors.Is(err, ErrKindLocked):
		return huma.Error422UnprocessableEntity("O tipo deste presente não pode mudar: ele já tem histórico de contribuições (mesmo canceladas). Desative-o e crie outro.")
	case errors.Is(err, ErrKindRequired):
		return huma.Error422UnprocessableEntity("Informe o tipo do presente (kind: pix ou link) ao editar — a omissão poderia converter o presente sem querer.")
	default:
		slog.Error("admin gift request failed", "error", err)
		return huma.Error500InternalServerError("erro ao salvar o presente")
	}
}
