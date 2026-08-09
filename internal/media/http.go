package media

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// MediaView is one uploaded image as the admin sees it.
type MediaView struct {
	MediaID    string    `json:"media_id" format:"uuid"`
	BucketPath string    `json:"bucket_path"`
	PublicURL  string    `json:"public_url" format:"uri" doc:"CDN URL browsers load the image from; paste into section payloads."`
	Alt        *string   `json:"alt,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// UploadForm is the multipart upload payload.
type UploadForm struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/webp,image/gif,image/avif" required:"true"`
	Alt  string        `form:"alt" doc:"Optional accessibility text."`
}

// UploadInput wraps the multipart form.
type UploadInput struct {
	RawBody huma.MultipartFormFiles[UploadForm]
}

// MediaOutput wraps one media record.
type MediaOutput struct {
	Body MediaView
}

// MediaListOutput lists uploads, newest first.
type MediaListOutput struct {
	Body struct {
		Media []MediaView `json:"media"`
	}
}

// DeleteMediaInput selects one media record.
type DeleteMediaInput struct {
	MediaID string `path:"media_id" format:"uuid"`
}

// RegisterAdmin mounts the media surface on the (already authenticated)
// admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "admin-upload-media",
		Method:        http.MethodPost,
		Path:          "/media",
		Summary:       "Upload an image",
		Description:   "Stores the image in the public bucket and records its CDN URL.",
		Tags:          []string{"media"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *UploadInput) (*MediaOutput, error) {
		form := in.RawBody.Data()
		if !form.File.IsSet {
			return nil, huma.Error422UnprocessableEntity("Envie o arquivo no campo \"file\".")
		}
		var alt *string
		if form.Alt != "" {
			alt = &form.Alt
		}
		m, err := svc.Upload(ctx, form.File.Filename, form.File.ContentType, form.File, alt)
		if errors.Is(err, ErrUnsupportedType) {
			return nil, huma.Error422UnprocessableEntity("Formato não suportado. Envie uma imagem JPG, PNG, WebP, GIF ou AVIF.")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao enviar a imagem", err)
		}
		return &MediaOutput{Body: view(m)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-media",
		Method:      http.MethodGet,
		Path:        "/media",
		Summary:     "List uploaded images",
		Tags:        []string{"media"},
	}, func(ctx context.Context, _ *struct{}) (*MediaListOutput, error) {
		items, err := svc.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao listar as imagens", err)
		}
		out := &MediaListOutput{}
		out.Body.Media = make([]MediaView, len(items))
		for i, m := range items {
			out.Body.Media[i] = view(m)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "admin-delete-media",
		Method:        http.MethodDelete,
		Path:          "/media/{media_id}",
		Summary:       "Delete an image",
		Description:   "Removes the object from the bucket and the record from the library. Sections still referencing the URL will show a broken image — update them first.",
		Tags:          []string{"media"},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *DeleteMediaInput) (*struct{}, error) {
		id, err := uuid.Parse(in.MediaID)
		if err != nil {
			return nil, huma.Error404NotFound("Imagem não encontrada.")
		}
		switch err := svc.Delete(ctx, id); {
		case errors.Is(err, ErrNotFound):
			return nil, huma.Error404NotFound("Imagem não encontrada.")
		case err != nil:
			return nil, huma.Error500InternalServerError("erro ao excluir a imagem", err)
		}
		return nil, nil
	})
}

func view(m Media) MediaView {
	return MediaView{
		MediaID:    m.ID.String(),
		BucketPath: m.BucketPath,
		PublicURL:  m.PublicURL,
		Alt:        m.Alt,
		CreatedAt:  m.CreatedAt,
	}
}
