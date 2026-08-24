package instagram

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// FeedInput selects whose feed to return.
type FeedInput struct {
	Person string `path:"person" enum:"bride,groom" doc:"Whose feed: the bride's or the groom's."`
}

// FeedOutput is the public feed response.
type FeedOutput struct {
	Body struct {
		Configured bool       `json:"configured" doc:"False until an Instagram import has been run for this person — the site links to the profile instead."`
		Posts      []PostView `json:"posts"`
	}
}

// RegisterPublic mounts the public feed endpoint.
func RegisterPublic(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-instagram-feed",
		Method:      http.MethodGet,
		Path:        platform.APIBase + "/instagram/{person}",
		Summary:     "Get an Instagram feed",
		Description: "Returns one person's imported Instagram posts, served from the site's own " +
			"storage (one-shot operator import — the site never calls Instagram at runtime). " +
			"Before any import, configured=false and posts is empty — never an error.",
		Tags: []string{"instagram"},
	}, func(ctx context.Context, in *FeedInput) (*FeedOutput, error) {
		out := &FeedOutput{}
		out.Body.Posts = []PostView{}
		posts, exists, err := svc.Posts(ctx, in.Person)
		if err != nil {
			slog.WarnContext(ctx, "instagram manifest fetch failed", "person", in.Person, "error", err)
		}
		out.Body.Configured = exists
		if len(posts) > 0 {
			out.Body.Posts = posts
		}
		return out, nil
	})
}
