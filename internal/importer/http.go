package importer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// ImportForm is the multipart upload payload. The content-type list includes
// application/vnd.ms-excel (what Windows browsers send for .csv when Excel is
// installed) and text/plain — the real gate is the extension + content sniff
// in ParseFile.
type ImportForm struct {
	File huma.FormFile `form:"file" contentType:"text/csv,text/plain,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/octet-stream" required:"true"`
}

// maxImportBytes caps the uploaded sheet (raw size).
const maxImportBytes = 20 << 20 // 20 MB

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
		data, err := io.ReadAll(io.LimitReader(form.File, maxImportBytes+1))
		if err != nil {
			slog.ErrorContext(ctx, "read import upload failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao ler o arquivo enviado")
		}
		if len(data) > maxImportBytes {
			return nil, huma.Error422UnprocessableEntity("Arquivo muito grande: o limite é 20 MB.")
		}
		report, err := svc.Import(ctx, form.File.Filename, data, Options{})
		var parseErr *ParseError
		if errors.As(err, &parseErr) {
			return nil, huma.Error422UnprocessableEntity(parseErr.Message)
		}
		if errors.Is(err, ErrNameCollision) {
			return nil, huma.Error409Conflict("A lista mudou enquanto o arquivo era processado e um nome entrou em conflito. Nada foi gravado — tente importar novamente.")
		}
		if err != nil {
			slog.ErrorContext(ctx, "import failed", "error", err)
			return nil, huma.Error500InternalServerError("erro ao processar a importação")
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
