package content

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// ContentOutput is the public content response: every enabled section, fully
// typed, in the fixed render order.
type ContentOutput struct {
	Body struct {
		Sections []Section `json:"sections"`
	}
}

// RegisterPublic mounts the public content surface.
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-content",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/content",
		Summary:     "Get site content",
		Description: "Returns every enabled editorial section in render order — the single call the public site makes for all of its copy.",
		Tags:        []string{"content"},
	}, func(ctx context.Context, _ *struct{}) (*ContentOutput, error) {
		sections, err := svc.PublicContent(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao carregar o conteúdo do site", err)
		}
		out := &ContentOutput{}
		out.Body.Sections = sections
		return out, nil
	})
}
