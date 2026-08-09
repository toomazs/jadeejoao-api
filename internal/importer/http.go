package importer

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// ImportForm is the multipart upload payload.
type ImportForm struct {
	File huma.FormFile `form:"file" contentType:"text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/octet-stream" required:"true"`
}

// ImportInput wraps the multipart form.
type ImportInput struct {
	RawBody huma.MultipartFormFiles[ImportForm]
}

// ImportOutput is the reconciliation report the admin reviews.
type ImportOutput struct {
	Body struct {
		Added     []string   `json:"added" doc:"Guests created by this import."`
		Updated   []string   `json:"updated" doc:"Existing guests whose identity fields were refreshed."`
		Unmatched []string   `json:"unmatched" doc:"Guests in the database that the file no longer mentions. Never deleted automatically."`
		Conflicts []Conflict `json:"conflicts" doc:"Rows refused: homonyms across groups or duplicates in the file."`
		Errors    []RowIssue `json:"errors" doc:"Row-level parse issues (PT-BR)."`
	}
}

// RegisterAdmin mounts the import endpoint on the (already authenticated)
// admin group.
func RegisterAdmin(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-import-guests",
		Method:      http.MethodPost,
		Path:        "/import",
		Summary:     "Import the guest list",
		Description: "Uploads a CSV or XLSX export of the couple's spreadsheet (headers: nome, grupo, principal, categoria — case/accent-insensitive). Upserts identity fields only; RSVP answers are never touched. Homonyms across groups are reported as conflicts, never written.",
		Tags:        []string{"importer"},
	}, func(ctx context.Context, in *ImportInput) (*ImportOutput, error) {
		form := in.RawBody.Data()
		if !form.File.IsSet {
			return nil, huma.Error422UnprocessableEntity("Envie o arquivo no campo \"file\".")
		}
		data, err := io.ReadAll(form.File)
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao ler o arquivo enviado", err)
		}
		report, err := svc.Import(ctx, form.File.Filename, data)
		var parseErr *ParseError
		if errors.As(err, &parseErr) {
			return nil, huma.Error422UnprocessableEntity(parseErr.Message)
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("erro ao processar a importação", err)
		}
		out := &ImportOutput{}
		out.Body.Added = orEmpty(report.Added)
		out.Body.Updated = orEmpty(report.Updated)
		out.Body.Unmatched = orEmpty(report.Unmatched)
		out.Body.Conflicts = report.Conflicts
		out.Body.Errors = report.Errors
		if out.Body.Conflicts == nil {
			out.Body.Conflicts = []Conflict{}
		}
		if out.Body.Errors == nil {
			out.Body.Errors = []RowIssue{}
		}
		return out, nil
	})
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
