package guests

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// GroupDashboardView is one group with its members.
type GroupDashboardView struct {
	GroupID string            `json:"group_id" format:"uuid"`
	Label   string            `json:"label"`
	Members []AdminMemberView `json:"members"`
}

// AdminMemberView is a guest as the couple sees them: everything the public
// view carries, plus the bookkeeping from their spreadsheet.
//
// Kept separate from MemberView on purpose. Side, circle, ceremony role and
// notes are the couple's private annotations — the notes are mostly kinship
// ("Marido Renata Gonçalves") — and any guest can open the public view by
// typing a name. None of that belongs there.
type AdminMemberView struct {
	GuestID      string  `json:"guest_id" format:"uuid"`
	FullName     string  `json:"full_name"`
	IsPrimary    bool    `json:"is_primary"`
	Category     *string `json:"category,omitempty" enum:"adult,teen,child,baby,elderly"`
	Attending    string  `json:"attending" enum:"pending,yes,no"`
	AddedByGuest bool    `json:"added_by_guest" doc:"True quando o próprio convidado trouxe essa pessoa."`
	Gender       *string `json:"gender,omitempty" enum:"female,male"`
	Side         *string `json:"side,omitempty" enum:"bride,groom,both" doc:"De quem é o convidado."`
	Circle       string  `json:"circle,omitempty" doc:"Círculo social: amigos, família, trabalho."`
	CeremonyRole string  `json:"ceremony_role,omitempty" doc:"Padrinho, madrinha, celebrante…"`
	Notes        string  `json:"notes,omitempty" doc:"Anotação livre, normalmente parentesco."`
}

// HeadcountsView aggregates RSVP state for planning.
type HeadcountsView struct {
	Total         int            `json:"total"`
	Yes           int            `json:"yes"`
	No            int            `json:"no"`
	Pending       int            `json:"pending"`
	YesByCategory map[string]int `json:"yes_by_category" doc:"Confirmed guests per category (adult, child, baby, elderly, uncategorized)."`
}

// DashboardOutput is the admin guest dashboard.
type DashboardOutput struct {
	Body struct {
		Groups     []GroupDashboardView `json:"groups"`
		Headcounts HeadcountsView       `json:"headcounts"`
	}
}

// ExportOutput streams the guest list as CSV.
type ExportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

// RegisterAdmin mounts the admin guest surface on the (already authenticated)
// admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-guest-dashboard",
		Method:      http.MethodGet,
		Path:        "/guests",
		Summary:     "Guest dashboard",
		Description: "Every group with per-guest RSVP state, plus headcounts by category.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, _ *struct{}) (*DashboardOutput, error) {
		groups, counts, err := svc.AdminDashboard(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "request failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao carregar o painel de convidados")
		}
		out := &DashboardOutput{}
		out.Body.Groups = make([]GroupDashboardView, len(groups))
		for i, g := range groups {
			view := GroupDashboardView{GroupID: g.ID.String(), Label: g.Label, Members: make([]AdminMemberView, len(g.Members))}
			for j, m := range g.Members {
				view.Members[j] = AdminMemberView{
					GuestID: m.ID.String(), FullName: m.FullName, IsPrimary: m.IsPrimary,
					Category: m.Category, Attending: m.Attending, AddedByGuest: m.AddedByGuest,
					Gender: m.Gender, Side: m.Side, Circle: m.Circle,
					CeremonyRole: m.CeremonyRole, Notes: m.Notes,
				}
			}
			out.Body.Groups[i] = view
		}
		out.Body.Headcounts = HeadcountsView(counts)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-export-guests",
		Method:      http.MethodGet,
		Path:        "/guests/export",
		Summary:     "Export guest list as CSV",
		Description: "PT-BR headers and values (convite, nome, principal, categoria, presenca). Re-importable as-is.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, _ *struct{}) (*ExportOutput, error) {
		data, err := svc.ExportCSV(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "request failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao exportar a lista de convidados")
		}
		return &ExportOutput{
			ContentType:        "text/csv; charset=utf-8",
			ContentDisposition: `attachment; filename="convidados.csv"`,
			Body:               data,
		}, nil
	})
}

// GuestEditInput corrects one person from the panel. Attendance is absent on
// purpose: the RSVP flow owns it, and an edit screen quietly rewriting
// somebody's answer would be a trap for the couple.
type GuestEditInput struct {
	GuestID string `path:"guest_id" format:"uuid"`
	Body    struct {
		FullName     string  `json:"full_name" minLength:"2" maxLength:"120"`
		Category     *string `json:"category,omitempty" enum:"adult,teen,child,baby,elderly"`
		Gender       *string `json:"gender,omitempty" enum:"female,male"`
		Side         *string `json:"side,omitempty" enum:"bride,groom,both" doc:"De quem é o convidado."`
		Circle       string  `json:"circle,omitempty" maxLength:"60" example:"Amigos" doc:"Círculo social: amigos, família, trabalho."`
		CeremonyRole string  `json:"ceremony_role,omitempty" maxLength:"60" example:"Madrinha"`
		Notes        string  `json:"notes,omitempty" maxLength:"500" doc:"Anotação livre, normalmente parentesco."`
	}
}

// GuestCreateInput adds one person the spreadsheet did not have.
type GuestCreateInput struct {
	Body struct {
		FullName     string  `json:"full_name" minLength:"2" maxLength:"120"`
		Category     *string `json:"category,omitempty" enum:"adult,teen,child,baby,elderly"`
		Gender       *string `json:"gender,omitempty" enum:"female,male"`
		Side         *string `json:"side,omitempty" enum:"bride,groom,both" doc:"De quem é o convidado."`
		Circle       string  `json:"circle,omitempty" maxLength:"60" example:"Amigos"`
		CeremonyRole string  `json:"ceremony_role,omitempty" maxLength:"60" example:"Madrinha"`
		Notes        string  `json:"notes,omitempty" maxLength:"500"`
		GroupID      string  `json:"group_id,omitempty" format:"uuid" doc:"Add to this invitation. Omit to give the person one of their own."`
	}
}

// GuestCreatedOutput carries the new id, so the panel can open the person it
// just made.
type GuestCreatedOutput struct {
	Body struct {
		GuestID string `json:"guest_id" format:"uuid"`
	}
}

// GuestIDInput addresses one guest.
type GuestIDInput struct {
	GuestID string `path:"guest_id" format:"uuid"`
}

// GroupRenameInput relabels one invitation.
type GroupRenameInput struct {
	GroupID string `path:"group_id" format:"uuid"`
	Body    struct {
		Label string `json:"label" minLength:"2" maxLength:"120" example:"Família Nascimento"`
	}
}

// GroupMemberInput moves a guest into an invitation, or hands it to them.
type GroupMemberInput struct {
	GroupID string `path:"group_id" format:"uuid"`
	Body    struct {
		GuestID string `json:"guest_id" format:"uuid"`
	}
}

// OkOutput is a bare acknowledgement for the management operations; the panel
// refetches the dashboard, which is the only shape it renders from.
type OkOutput struct {
	Body struct {
		Ok bool `json:"ok"`
	}
}

func ok() *OkOutput {
	out := &OkOutput{}
	out.Body.Ok = true
	return out
}

// RegisterAdminManagement mounts the list-repair surface: renaming, merging,
// promoting and deleting.
//
// The importer owns identity on bulk upload (AD-10) and never deletes, so
// without these the couple's only way to fix a misspelling or merge two
// families is direct database access.
func RegisterAdminManagement(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-guest",
		Method:        http.MethodPost,
		Path:          "/guests",
		DefaultStatus: http.StatusCreated,
		Summary:       "Add a guest",
		Description: "Adds one person by hand. Without a group_id they get an invitation of their " +
			"own, labelled with their name and theirs to answer — the shape every imported guest " +
			"starts in. Attendance is not settable here: it belongs to the RSVP flow.",
		Tags: []string{"guests"},
	}, func(ctx context.Context, in *GuestCreateInput) (*GuestCreatedOutput, error) {
		var into *uuid.UUID
		if in.Body.GroupID != "" {
			parsed, err := uuid.Parse(in.Body.GroupID)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity("Identificador de convite inválido.")
			}
			into = &parsed
		}
		id, err := svc.CreateGuest(ctx, GuestEdit{
			FullName:     in.Body.FullName,
			Category:     in.Body.Category,
			Gender:       in.Body.Gender,
			Side:         in.Body.Side,
			Circle:       in.Body.Circle,
			CeremonyRole: in.Body.CeremonyRole,
			Notes:        in.Body.Notes,
		}, into)
		if err != nil {
			return nil, mapAdminGuestErr(err)
		}
		out := &GuestCreatedOutput{}
		out.Body.GuestID = id.String()
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-edit-guest",
		Method:      http.MethodPatch,
		Path:        "/guests/{guest_id}",
		Summary:     "Edit a guest",
		Description: "Corrects identity fields. Attendance is not editable here — it belongs to the RSVP flow.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *GuestEditInput) (*OkOutput, error) {
		guestID, err := uuid.Parse(in.GuestID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de convidado inválido.")
		}
		err = svc.EditGuest(ctx, guestID, GuestEdit{
			FullName:     in.Body.FullName,
			Category:     in.Body.Category,
			Gender:       in.Body.Gender,
			Side:         in.Body.Side,
			Circle:       in.Body.Circle,
			CeremonyRole: in.Body.CeremonyRole,
			Notes:        in.Body.Notes,
		})
		if err != nil {
			return nil, mapAdminGuestErr(err)
		}
		return ok(), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-guest",
		Method:      http.MethodDelete,
		Path:        "/guests/{guest_id}",
		Summary:     "Delete a guest",
		Description: "Removes one person for good, and their invitation with them if it is left empty. Never happens automatically on import (AD-10).",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *GuestIDInput) (*OkOutput, error) {
		guestID, err := uuid.Parse(in.GuestID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de convidado inválido.")
		}
		if err := svc.DeleteGuest(ctx, guestID); err != nil {
			return nil, mapAdminGuestErr(err)
		}
		return ok(), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-rename-invitation",
		Method:      http.MethodPatch,
		Path:        "/groups/{group_id}",
		Summary:     "Rename an invitation",
		Description: "Relabels the invitation — what the guest sees at the top of their card.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *GroupRenameInput) (*OkOutput, error) {
		groupID, err := uuid.Parse(in.GroupID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
		}
		if err := svc.RenameInvitation(ctx, groupID, in.Body.Label); err != nil {
			return nil, mapAdminGuestErr(err)
		}
		return ok(), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-merge-into-invitation",
		Method:      http.MethodPost,
		Path:        "/groups/{group_id}/members",
		Summary:     "Move a guest into an invitation",
		Description: "Merges someone into this invitation, tidying away the one they leave behind if it empties. Unlike the guest-facing path, the couple may move anyone.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *GroupMemberInput) (*OkOutput, error) {
		groupID, guestID, err := parsePair(in.GroupID, in.Body.GuestID)
		if err != nil {
			return nil, err
		}
		if err := svc.MergeIntoInvitation(ctx, groupID, guestID); err != nil {
			return nil, mapAdminGuestErr(err)
		}
		return ok(), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-detach-guest",
		Method:      http.MethodDelete,
		Path:        "/guests/{guest_id}/invitation",
		Summary:     "Give a guest their own invitation",
		Description: "Takes somebody out of a shared invitation into one of their own, labelled " +
			"with their name and theirs to answer. Refused when they are already alone. If they " +
			"were the one holding the invitation, whoever is left is handed it.",
		Tags: []string{"guests"},
	}, func(ctx context.Context, in *GuestIDInput) (*OkOutput, error) {
		guestID, err := uuid.Parse(in.GuestID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Identificador de convidado inválido.")
		}
		if err := svc.DetachFromInvitation(ctx, guestID); err != nil {
			return nil, mapAdminGuestErr(err)
		}
		return ok(), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-set-primary",
		Method:      http.MethodPost,
		Path:        "/groups/{group_id}/primary",
		Summary:     "Hand an invitation to someone",
		Description: "Makes this guest the one who answers for the invitation. Applied in a single statement, so the invitation never has two primaries or none.",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, in *GroupMemberInput) (*OkOutput, error) {
		groupID, guestID, err := parsePair(in.GroupID, in.Body.GuestID)
		if err != nil {
			return nil, err
		}
		if err := svc.SetPrimary(ctx, groupID, guestID); err != nil {
			return nil, mapAdminGuestErr(err)
		}
		return ok(), nil
	})
}

func parsePair(rawGroup, rawGuest string) (uuid.UUID, uuid.UUID, error) {
	groupID, err := uuid.Parse(rawGroup)
	if err != nil {
		return uuid.Nil, uuid.Nil, huma.Error422UnprocessableEntity("Identificador de grupo inválido.")
	}
	guestID, err := uuid.Parse(rawGuest)
	if err != nil {
		return uuid.Nil, uuid.Nil, huma.Error422UnprocessableEntity("Identificador de convidado inválido.")
	}
	return groupID, guestID, nil
}

// mapAdminGuestErr speaks to the couple, not to guests: these messages appear
// in the panel, where naming the actual problem is more useful than being kind.
func mapAdminGuestErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("Não encontrado. A lista pode ter mudado — recarregue a página.")
	case errors.Is(err, ErrNameTaken):
		return huma.Error409Conflict("Já existe um convidado com esse nome. Os nomes precisam ser únicos porque é por eles que o convidado encontra o próprio convite.")
	case errors.Is(err, ErrAlreadyOnInvitation):
		return huma.Error409Conflict("Essa pessoa já está nesse convite.")
	case errors.Is(err, ErrInvalidName):
		return huma.Error422UnprocessableEntity("O nome não pode ficar em branco.")
	case errors.Is(err, ErrInvalidField):
		return huma.Error422UnprocessableEntity("Valor não reconhecido em um dos campos.")
	default:
		slog.Error("admin guest management failed", "error", err)
		return huma.Error500InternalServerError("Não conseguimos salvar agora. Tente novamente em instantes.")
	}
}
