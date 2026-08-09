package guests

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// MemberView is one guest of a group, with their current RSVP answer.
type MemberView struct {
	GuestID   string  `json:"guest_id" format:"uuid"`
	FullName  string  `json:"full_name" example:"Eduardo Silva"`
	IsPrimary bool    `json:"is_primary" doc:"Whether this guest is the group's primary contact."`
	Category  *string `json:"category,omitempty" enum:"adult,child,baby,elderly"`
	Attending string  `json:"attending" enum:"pending,yes,no"`
}

// GroupView is the guest group returned by lookup and RSVP.
type GroupView struct {
	GroupID string       `json:"group_id" format:"uuid"`
	Label   string       `json:"label" example:"Eduardo e família"`
	Members []MemberView `json:"members"`
}

// LookupInput carries the typed full name.
type LookupInput struct {
	Body struct {
		FullName string `json:"full_name" minLength:"1" maxLength:"200" example:"Eduardo Silva" doc:"Full name as written on the invitation. Matching is exact after normalization (case and accents ignored)."`
	}
}

// GroupOutput wraps a GroupView response.
type GroupOutput struct {
	Body GroupView
}

// RSVPAnswer is one member's answer in an RSVP submission.
type RSVPAnswer struct {
	GuestID   string `json:"guest_id" format:"uuid"`
	Attending string `json:"attending" enum:"yes,no"`
}

// RSVPInput submits one answer per member of the group.
type RSVPInput struct {
	GroupID string `path:"group_id" format:"uuid"`
	Body    struct {
		Responses []RSVPAnswer `json:"responses" minItems:"1" doc:"Exactly one answer per member of the group."`
	}
}

// RegisterPublic mounts lookup and RSVP.
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "lookup-guest",
		Method:      http.MethodPost,
		Path:        platform.APIBase + "/guests/lookup",
		Summary:     "Find your invitation",
		Description: "Exact normalized full-name match. Returns the guest group with every member's current RSVP state. No fuzzy matching or suggestions — misses return 404.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *LookupInput) (*GroupOutput, error) {
		group, members, err := svc.Lookup(ctx, in.Body.FullName)
		if err != nil {
			return nil, mapGuestErr(err)
		}
		return groupOutput(group, members), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "submit-rsvp",
		Method:      http.MethodPost,
		Path:        platform.APIBase + "/guests/{group_id}/rsvp",
		Summary:     "Submit RSVP",
		Description: "Records one yes/no per member of the group. Idempotent: resubmitting overwrites earlier answers. Rejected after the RSVP deadline.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *RSVPInput) (*GroupOutput, error) {
		groupID, err := uuid.Parse(in.GroupID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
		}
		updates := make([]AttendanceUpdate, len(in.Body.Responses))
		for i, r := range in.Body.Responses {
			guestID, err := uuid.Parse(r.GuestID)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity("Identificador de convidado inválido.")
			}
			updates[i] = AttendanceUpdate{GuestID: guestID, Attending: r.Attending}
		}
		group, members, err := svc.SubmitRSVP(ctx, groupID, updates)
		if err != nil {
			return nil, mapGuestErr(err)
		}
		return groupOutput(group, members), nil
	})
}

func groupOutput(group Group, members []Member) *GroupOutput {
	out := &GroupOutput{}
	out.Body.GroupID = group.ID.String()
	out.Body.Label = group.Label
	out.Body.Members = make([]MemberView, len(members))
	for i, m := range members {
		out.Body.Members[i] = MemberView{
			GuestID:   m.ID.String(),
			FullName:  m.FullName,
			IsPrimary: m.IsPrimary,
			Category:  m.Category,
			Attending: m.Attending,
		}
	}
	return out
}

func mapGuestErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("Nome não encontrado. Confira se digitou o nome completo como está no convite ou fale com os noivos.")
	case errors.Is(err, ErrDeadlinePassed):
		return huma.Error422UnprocessableEntity("O prazo para confirmar presença já passou. Fale diretamente com os noivos.")
	case errors.Is(err, ErrInvalidMembers):
		return huma.Error422UnprocessableEntity("A confirmação precisa incluir exatamente os convidados do seu grupo, sem repetições.")
	default:
		return huma.Error500InternalServerError("Não conseguimos processar seu pedido agora. Tente novamente em instantes.", err)
	}
}
