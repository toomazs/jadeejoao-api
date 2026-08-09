package content

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// SectionsOutput lists every section, including disabled ones.
type SectionsOutput struct {
	Body struct {
		Sections []Section `json:"sections"`
	}
}

// SectionUpdateInput is the admin edit for one slug. The slug set is closed:
// update-only, no create, no delete.
type SectionUpdateInput struct {
	Slug string `path:"slug" enum:"hero,our_story,big_day,rsvp,getting_there,stay,gifts_intro,dress_code,good_practices,messages_intro"`
	// Body reuses Section; its slug field is ignored — the path decides.
	Body Section
}

// SectionOutput wraps one updated section.
type SectionOutput struct {
	Body Section
}

// RegisterAdmin mounts the admin content surface on the (already
// authenticated) admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-sections",
		Method:      http.MethodGet,
		Path:        "/sections",
		Summary:     "List all sections",
		Description: "Every section in render order, including disabled ones.",
		Tags:        []string{"content"},
	}, func(ctx context.Context, _ *struct{}) (*SectionsOutput, error) {
		sections, err := svc.AdminContent(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao carregar as seções", err)
		}
		out := &SectionsOutput{}
		out.Body.Sections = sections
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-section",
		Method:      http.MethodPut,
		Path:        "/sections/{slug}",
		Summary:     "Update one section",
		Description: "Replaces the payload and visibility of one slug. The payload field must match the slug (e.g. body.hero for slug hero).",
		Tags:        []string{"content"},
	}, func(ctx context.Context, in *SectionUpdateInput) (*SectionOutput, error) {
		updated, err := svc.UpdateSection(ctx, in.Slug, in.Body)
		switch {
		case errors.Is(err, ErrUnknownSlug):
			return nil, huma.Error404NotFound("Seção não encontrada.")
		case errors.Is(err, ErrPayloadMismatch):
			return nil, huma.Error422UnprocessableEntity("O conteúdo enviado não corresponde à seção: preencha o campo com o mesmo nome do slug.")
		case err != nil:
			return nil, huma.Error500InternalServerError("erro ao salvar a seção", err)
		}
		return &SectionOutput{Body: updated}, nil
	})
}
