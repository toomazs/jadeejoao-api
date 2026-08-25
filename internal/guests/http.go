package guests

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	Category  *string `json:"category,omitempty" enum:"adult,teen,child,baby,elderly"`
	Attending string  `json:"attending" enum:"pending,yes,no"`
	// AddedByGuest lets the invitation show how much of its allowance is
	// spent, instead of offering a button that answers with 409.
	AddedByGuest bool `json:"added_by_guest" doc:"True when a guest added this person to the invitation themselves."`
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

// CompanionInput gathers one more invited person into an existing invitation.
// It takes an id, never a name: the guest picks from the couple's list.
type CompanionInput struct {
	GroupID string `path:"group_id" format:"uuid"`
	Body    struct {
		GuestID string `json:"guest_id" format:"uuid" doc:"Quem vem junto, escolhido na busca — precisa já estar na lista dos noivos."`
	}
}

// CompanionSearchInput is the typeahead over people this invitation may gather.
type CompanionSearchInput struct {
	GroupID string `path:"group_id" format:"uuid"`
	Q       string `query:"q" required:"true" minLength:"3" maxLength:"100" example:"mar" doc:"Começo do nome de quem vem junto; a busca ignora acentos e maiúsculas."`
}

// CompanionOptionsOutput lists people the invitation may gather in.
type CompanionOptionsOutput struct {
	Body struct {
		Options []CompanionOptionView `json:"options" doc:"No máximo 8, em ordem alfabética."`
	}
}

// CompanionOptionView is one pickable person.
type CompanionOptionView struct {
	GuestID  string `json:"guest_id" format:"uuid"`
	FullName string `json:"full_name" example:"Maria Silva"`
}

// RemoveCompanionInput takes back someone the guest added.
type RemoveCompanionInput struct {
	GroupID string `path:"group_id" format:"uuid"`
	GuestID string `path:"guest_id" format:"uuid"`
}

// SuggestInput is the typeahead query (AD-5, amended 2026-08-10).
type SuggestInput struct {
	Q string `query:"q" required:"true" minLength:"3" maxLength:"100" example:"edu" doc:"Beginning of the guest's name; matching ignores case and accents."`
}

// SuggestOutput lists matching guest names (names only, max 8).
type SuggestOutput struct {
	Body struct {
		Suggestions []string `json:"suggestions" doc:"Up to 8 full names. Pick one and confirm it via POST /guests/lookup."`
	}
}

// RegisterPublic mounts suggest, lookup, and RSVP.
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "suggest-guest-names",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/guests/suggest",
		Summary:     "Suggest guest names",
		Description: "Prefix typeahead for the RSVP name field: up to 8 full names, accent- and case-insensitive, rate-limited, debounced client-side. Returns names only — group data still requires the exact lookup.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *SuggestInput) (*SuggestOutput, error) {
		names, err := svc.SuggestNames(ctx, in.Q)
		if err != nil {
			return nil, mapGuestErr(err)
		}
		out := &SuggestOutput{}
		out.Body.Suggestions = names
		return out, nil
	})

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

	huma.Register(api, huma.Operation{
		OperationID: "search-companions",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/guests/{group_id}/companions/available",
		Summary:     "Search people to bring along",
		Description: "Names this invitation may gather in: guests the couple already invited who are still alone in their own invitation. Someone heading an invitation that holds other people is excluded — moving them would orphan the rest.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *CompanionSearchInput) (*CompanionOptionsOutput, error) {
		groupID, err := uuid.Parse(in.GroupID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
		}
		options, err := svc.SuggestCompanions(ctx, groupID, in.Q)
		if err != nil {
			return nil, mapGuestErr(err)
		}
		out := &CompanionOptionsOutput{}
		out.Body.Options = make([]CompanionOptionView, len(options))
		for i, o := range options {
			out.Body.Options[i] = CompanionOptionView{GuestID: o.ID.String(), FullName: o.FullName}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "add-companion",
		Method:        http.MethodPost,
		Path:          platform.APIBase + "/guests/{group_id}/companions",
		Summary:       "Add a companion",
		Description:   fmt.Sprintf("Gathers one more ALREADY-INVITED person into this invitation, by id. The guest never types a name — the list is the couple's budget. Only people still alone in their own invitation can be gathered; capped at %d per invitation, rejected after the RSVP deadline. Returns the whole group.", MaxCompanionsPerGroup),
		Tags:          []string{"guests"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *CompanionInput) (*GroupOutput, error) {
		groupID, err := uuid.Parse(in.GroupID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
		}
		guestID, err := uuid.Parse(in.Body.GuestID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Escolha alguém da lista para vir com você.")
		}
		group, members, err := svc.AddCompanion(ctx, groupID, guestID)
		if err != nil {
			return nil, mapGuestErr(err)
		}
		return groupOutput(group, members), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "remove-companion",
		Method:      http.MethodDelete,
		Path:        platform.APIBase + "/guests/{group_id}/companions/{guest_id}",
		Summary:     "Send a companion back",
		Description: "Sends someone back to their own invitation — they stay invited, they just leave this group. Refuses for anyone the couple placed here: a guest can undo their own gathering, never edit the couple's list. Rejected after the RSVP deadline. Returns the remaining group.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *RemoveCompanionInput) (*GroupOutput, error) {
		groupID, err := uuid.Parse(in.GroupID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
		}
		guestID, err := uuid.Parse(in.GuestID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de convidado inválido.")
		}
		group, members, err := svc.RemoveCompanion(ctx, groupID, guestID)
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
			GuestID:      m.ID.String(),
			FullName:     m.FullName,
			IsPrimary:    m.IsPrimary,
			Category:     m.Category,
			Attending:    m.Attending,
			AddedByGuest: m.AddedByGuest,
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
	case errors.Is(err, ErrInvalidAnswer):
		return huma.Error422UnprocessableEntity("Resposta inválida: confirme com \"yes\" ou \"no\" para cada convidado.")
	case errors.Is(err, ErrCompanionLimit):
		return huma.Error409Conflict(fmt.Sprintf("Você já adicionou %d acompanhantes, que é o limite deste convite. Se precisar levar mais alguém, fale com os noivos.", MaxCompanionsPerGroup))
	case errors.Is(err, ErrGuestUnavailable):
		return huma.Error409Conflict("Essa pessoa já está no convite de outra pessoa. Fale com os noivos para ajustar.")
	case errors.Is(err, ErrAlreadyOnInvitation):
		return huma.Error409Conflict("Essa pessoa já está no seu convite.")
	case errors.Is(err, ErrNotRemovable):
		return huma.Error403Forbidden("Você só pode tirar do convite quem você mesmo trouxe. Para os demais, fale com os noivos.")
	default:
		slog.Error("guest request failed", "error", err)
		return huma.Error500InternalServerError("Não conseguimos processar seu pedido agora. Tente novamente em instantes.")
	}
}
