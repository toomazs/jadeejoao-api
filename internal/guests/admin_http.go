package guests

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// GroupDashboardView is one group with its members.
type GroupDashboardView struct {
	GroupID string       `json:"group_id" format:"uuid"`
	Label   string       `json:"label"`
	Members []MemberView `json:"members"`
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
			return nil, huma.Error500InternalServerError("erro ao carregar o painel de convidados", err)
		}
		out := &DashboardOutput{}
		out.Body.Groups = make([]GroupDashboardView, len(groups))
		for i, g := range groups {
			view := GroupDashboardView{GroupID: g.ID.String(), Label: g.Label, Members: make([]MemberView, len(g.Members))}
			for j, m := range g.Members {
				view.Members[j] = MemberView{
					GuestID: m.ID.String(), FullName: m.FullName, IsPrimary: m.IsPrimary,
					Category: m.Category, Attending: m.Attending,
				}
			}
			out.Body.Groups[i] = view
		}
		out.Body.Headcounts = HeadcountsView{
			Total: counts.Total, Yes: counts.Yes, No: counts.No, Pending: counts.Pending,
			YesByCategory: counts.YesByCategory,
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-export-guests",
		Method:      http.MethodGet,
		Path:        "/guests/export",
		Summary:     "Export guest list as CSV",
		Description: "PT-BR headers and values (grupo, nome, principal, categoria, presenca).",
		Tags:        []string{"guests"},
	}, func(ctx context.Context, _ *struct{}) (*ExportOutput, error) {
		data, err := svc.ExportCSV(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao exportar a lista de convidados", err)
		}
		return &ExportOutput{
			ContentType:        "text/csv; charset=utf-8",
			ContentDisposition: `attachment; filename="convidados.csv"`,
			Body:               data,
		}, nil
	})
}
