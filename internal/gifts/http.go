package gifts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
	"github.com/jadeejoao/jadeejoao-api/internal/platform/pix"
)

// GiftView is one gift as the public site renders it: a PIX meta/cota card
// (kind=pix, with computed progress) or an external registry card (kind=link,
// the SPA opens external_url with target=_blank).
type GiftView struct {
	GiftID            string  `json:"gift_id" format:"uuid"`
	Title             string  `json:"title" example:"Para o João fazer a barba"`
	Description       string  `json:"description,omitempty"`
	ImageURL          *string `json:"image_url,omitempty" format:"uri"`
	Kind              string  `json:"kind" enum:"pix,link" doc:"pix: metas/cotas paid by PIX through the site. link: external store registry — the card redirects and no money flows through the API."`
	Platform          *string `json:"platform,omitempty" example:"mercadolivre" doc:"Store slug for link gifts; the SPAs map it to a local logo asset."`
	ExternalURL       *string `json:"external_url,omitempty" format:"uri" doc:"The couple's registry URL (link gifts only)."`
	GoalCentavos      *int64  `json:"goal_centavos,omitempty" doc:"Goal amount in centavos (BRL). PIX gifts only."`
	QuotaCentavos     *int64  `json:"quota_centavos,omitempty" doc:"Fixed quota unit in centavos; contributions must be multiples. Null means free amount. PIX gifts only."`
	MaxUnits          *int32  `json:"max_units,omitempty" doc:"Maximum sellable quota units; null means unlimited. PIX gifts only."`
	UnitsLeft         *int64  `json:"units_left,omitempty" doc:"Remaining quota units (only for unit-limited PIX gifts). Counts confirmed contributions only — a declared PIX is a claim, not money."`
	DeclaredCentavos  int64   `json:"declared_centavos" doc:"Sum of declared contributions still waiting for the couple to confirm them. Not shown as received."`
	ConfirmedCentavos int64   `json:"confirmed_centavos" doc:"Sum of admin-confirmed contributions. Always 0 for link gifts."`
	Sort              int32   `json:"sort"`
}

// GiftsOutput lists active gifts.
type GiftsOutput struct {
	Body struct {
		Gifts []GiftView `json:"gifts"`
	}
}

// PixPreviewInput asks for a side-effect-free copia-e-cola.
type PixPreviewInput struct {
	GiftID         string `path:"gift_id" format:"uuid"`
	AmountCentavos int64  `query:"amount_centavos" minimum:"1" maximum:"10000000000" doc:"Amount in centavos. Optional for quota gifts (defaults to one quota); required for free-amount gifts."`
}

// PixOutput carries the PIX copia-e-cola string and the same payload drawn
// as a QR symbol, so a guest on a desktop can pay with their phone.
type PixOutput struct {
	Body struct {
		GiftID         string `json:"gift_id" format:"uuid"`
		AmountCentavos int64  `json:"amount_centavos"`
		PixCode        string `json:"pix_code" doc:"Static PIX BR Code (copia-e-cola), CRC included."`
		QRSVG          string `json:"qr_svg,omitempty" doc:"The same BR Code as an inline SVG QR symbol; inherits the surrounding text colour."`
	}
}

// ContributionInput is the guest's commit point ("já fiz o PIX").
type ContributionInput struct {
	GiftID string `path:"gift_id" format:"uuid"`
	Body   struct {
		ContributorName string `json:"contributor_name" minLength:"1" maxLength:"200" example:"Eduardo Silva"`
		AmountCentavos  *int64 `json:"amount_centavos,omitempty" minimum:"1" maximum:"10000000000" doc:"Amount in centavos. Optional for quota gifts (defaults to one quota); required for free-amount gifts."`
		GroupID         string `json:"group_id,omitempty" format:"uuid" doc:"Guest group, auto-filled by the site after lookup."`
	}
}

// ContributionOutput confirms the declared contribution and repeats the PIX
// string for convenience.
type ContributionOutput struct {
	Body struct {
		ContributionID string `json:"contribution_id" format:"uuid"`
		GiftID         string `json:"gift_id" format:"uuid"`
		AmountCentavos int64  `json:"amount_centavos"`
		Status         string `json:"status" enum:"declared,confirmed,cancelled"`
		PixCode        string `json:"pix_code"`
	}
}

// RegisterPublic mounts the public gift surface.
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-gifts",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/gifts",
		Summary:     "List gifts",
		Description: "Active gifts with computed progress (declared + confirmed centavos, remaining units for quota gifts).",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, _ *struct{}) (*GiftsOutput, error) {
		gifts, err := svc.ListGifts(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "list gifts failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao carregar a lista de presentes")
		}
		out := &GiftsOutput{}
		out.Body.Gifts = make([]GiftView, len(gifts))
		for i, g := range gifts {
			out.Body.Gifts[i] = giftView(g)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "preview-gift-pix",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/gifts/{gift_id}/pix",
		Summary:     "Preview PIX code",
		Description: "Side-effect-free copia-e-cola preview — window-shopping creates nothing. The contribution is only recorded by POST /gifts/{gift_id}/contributions.",
		Tags:        []string{"gifts"},
	}, func(ctx context.Context, in *PixPreviewInput) (*PixOutput, error) {
		giftID, err := uuid.Parse(in.GiftID)
		if err != nil {
			return nil, huma.Error404NotFound("Presente não encontrado.")
		}
		var requested *int64
		if in.AmountCentavos > 0 {
			requested = &in.AmountCentavos
		}
		code, amount, err := svc.PixPreview(ctx, giftID, requested)
		if err != nil {
			return nil, mapGiftErr(err)
		}
		out := &PixOutput{}
		out.Body.GiftID = in.GiftID
		out.Body.AmountCentavos = amount
		out.Body.PixCode = code
		// The QR is a convenience: a failure here must not deny the guest
		// their copia-e-cola, which is the payment path that always works.
		if qr, qrErr := pix.QRSVG(code); qrErr != nil {
			slog.WarnContext(ctx, "pix qr render failed", "gift", in.GiftID, "error", qrErr)
		} else {
			out.Body.QRSVG = qr
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "declare-contribution",
		Method:        http.MethodPost,
		Path:          platform.APIBase + "/gifts/{gift_id}/contributions",
		Summary:       "Declare a contribution",
		Description:   "The guest's \"já fiz o PIX\" commit point: records a declared contribution (admin confirms when the PIX lands) and returns the copia-e-cola again. Quota gifts are serialized under a row lock so the last quota cannot be double-claimed.",
		Tags:          []string{"gifts"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *ContributionInput) (*ContributionOutput, error) {
		giftID, err := uuid.Parse(in.GiftID)
		if err != nil {
			return nil, huma.Error404NotFound("Presente não encontrado.")
		}
		var groupID *uuid.UUID
		if in.Body.GroupID != "" {
			parsed, err := uuid.Parse(in.Body.GroupID)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
			}
			groupID = &parsed
		}
		contribution, code, err := svc.Declare(ctx, giftID, groupID, in.Body.ContributorName, in.Body.AmountCentavos)
		if err != nil {
			return nil, mapGiftErr(err)
		}
		out := &ContributionOutput{}
		out.Body.ContributionID = contribution.ID.String()
		out.Body.GiftID = contribution.GiftID.String()
		out.Body.AmountCentavos = contribution.AmountCentavos
		out.Body.Status = contribution.Status
		out.Body.PixCode = code
		return out, nil
	})
}

func giftView(g Gift) GiftView {
	return GiftView{
		GiftID:            g.ID.String(),
		Title:             g.Title,
		Description:       g.Description,
		ImageURL:          g.ImageURL,
		Kind:              g.Kind,
		Platform:          g.Platform,
		ExternalURL:       g.ExternalURL,
		GoalCentavos:      g.GoalCentavos,
		QuotaCentavos:     g.QuotaCentavos,
		MaxUnits:          g.MaxUnits,
		UnitsLeft:         g.UnitsLeft(),
		DeclaredCentavos:  g.DeclaredCentavos,
		ConfirmedCentavos: g.ConfirmedCentavos,
		Sort:              g.Sort,
	}
}

func mapGiftErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("Presente não encontrado.")
	case errors.Is(err, ErrInvalidAmount):
		return huma.Error422UnprocessableEntity("Valor inválido para este presente. Para presentes com cota, o valor precisa ser um múltiplo da cota.")
	case errors.Is(err, ErrNoUnitsLeft):
		return huma.Error409Conflict("As cotas deste presente já foram todas reservadas. Escolha outro presente ou fale com os noivos.")
	case errors.Is(err, ErrUnknownGroup):
		return huma.Error422UnprocessableEntity("Grupo de convidados inválido. Refaça a busca pelo seu nome e tente novamente.")
	case errors.Is(err, ErrExternalGift):
		return huma.Error422UnprocessableEntity("Este presente é uma lista externa: reserve direto na loja pelo link do card.")
	default:
		slog.Error("gift request failed", "error", err)
		return huma.Error500InternalServerError("Não conseguimos processar seu pedido agora. Tente novamente em instantes.")
	}
}
