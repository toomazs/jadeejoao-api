package instagram

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// MaxPosts is what the gallery holds. The chapter shows nine at most and the
// couple keeps six; the ceiling is here so a bad request cannot write a
// manifest that every guest then downloads.
const MaxPosts = 12

// AdminFeedInput selects whose gallery to read.
type AdminFeedInput struct {
	Person string `path:"person" enum:"bride,groom" doc:"Whose gallery: the bride's or the groom's."`
}

// AdminFeedOutput is the stored gallery, exactly as saved.
type AdminFeedOutput struct {
	Body struct {
		Posts []PostView `json:"posts"`
	}
}

// ReplaceFeedInput carries the whole gallery: what is sent is what is stored.
type ReplaceFeedInput struct {
	Person string `path:"person" enum:"bride,groom"`
	Body   struct {
		Posts []PostView `json:"posts" doc:"The gallery in display order. Sending fewer replaces the rest — this is not a merge."`
	}
}

// RegisterAdmin mounts the panel's read and write for the galleries.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-admin-instagram-feed",
		Method:      http.MethodGet,
		Path:        "/instagram/{person}",
		Summary:     "Read one gallery for editing",
		Description: "Returns the stored gallery, read past the public cache so the panel " +
			"always edits what is actually saved.",
		Tags: []string{"instagram"},
	}, func(ctx context.Context, in *AdminFeedInput) (*AdminFeedOutput, error) {
		posts, err := svc.Editable(ctx, in.Person)
		if err != nil {
			return nil, huma.Error502BadGateway("Não conseguimos ler as fotos.", err)
		}
		out := &AdminFeedOutput{}
		out.Body.Posts = posts
		if out.Body.Posts == nil {
			out.Body.Posts = []PostView{}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "replace-admin-instagram-feed",
		Method:      http.MethodPut,
		Path:        "/instagram/{person}",
		Summary:     "Replace one gallery",
		Description: "Writes the manifest and drops the cached copy, so the site shows the " +
			"change immediately rather than at the end of the cache window.",
		Tags: []string{"instagram"},
	}, func(ctx context.Context, in *ReplaceFeedInput) (*AdminFeedOutput, error) {
		posts, err := clean(in.Body.Posts, in.Person)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err := svc.Replace(ctx, in.Person, posts); err != nil {
			return nil, huma.Error502BadGateway("Não conseguimos salvar as fotos.", err)
		}
		out := &AdminFeedOutput{}
		out.Body.Posts = posts
		return out, nil
	})
}

// clean rejects what the site cannot draw and fills in what the panel has no
// good way to invent.
func clean(posts []PostView, person string) ([]PostView, error) {
	if len(posts) > MaxPosts {
		return nil, fmt.Errorf("no máximo %d fotos por pessoa", MaxPosts)
	}
	seen := map[string]bool{}
	out := make([]PostView, 0, len(posts))
	for i, post := range posts {
		post.MediaURL = strings.TrimSpace(post.MediaURL)
		post.Permalink = strings.TrimSpace(post.Permalink)
		post.Caption = strings.TrimSpace(post.Caption)
		if post.MediaURL == "" {
			return nil, fmt.Errorf("a foto %d está sem imagem", i+1)
		}
		if post.MediaType == "" {
			post.MediaType = "IMAGE"
		}
		// The id is React's key on the site and this list's identity here, so
		// it has to exist and has to be unique. The imported ones are
		// Instagram's; a photo added in the panel gets one from the clock,
		// which is enough for a list this size.
		if post.ID == "" || seen[post.ID] {
			post.ID = fmt.Sprintf("%s-%d-%d", person, time.Now().UnixMilli(), i)
		}
		seen[post.ID] = true
		out = append(out, post)
	}
	return out, nil
}
