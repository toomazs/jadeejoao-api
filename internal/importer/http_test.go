package importer

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

type fakeImportRepo struct {
	snap    Snapshot
	applied *Plan
	report  []byte
}

func (f *fakeImportRepo) Snapshot(context.Context) (Snapshot, error) { return f.snap, nil }

func (f *fakeImportRepo) Apply(_ context.Context, plan Plan) error {
	f.applied = &plan
	return nil
}

func (f *fakeImportRepo) SaveReport(_ context.Context, report []byte) error {
	f.report = report
	return nil
}

func multipartFile(t *testing.T, filename string, content []byte) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType(), &buf
}

func TestImportEndpoint(t *testing.T) {
	repo := &fakeImportRepo{}
	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(repo))

	csv := "nome,grupo,principal,categoria\nEduardo Silva,Família Silva,sim,adulto\nAna Clara Silva,Família Silva,,criança\n"
	contentType, body := multipartFile(t, "convidados.csv", []byte(csv))

	resp := api.Post("/import", "Content-Type: "+contentType, body)
	if resp.Code != http.StatusOK {
		t.Fatalf("import = %d: %s", resp.Code, resp.Body.String())
	}
	out := resp.Body.String()
	if !strings.Contains(out, "Eduardo Silva") || !strings.Contains(out, `"conflicts":[]`) {
		t.Fatalf("unexpected report: %s", out)
	}
	if repo.applied == nil || len(repo.applied.Groups) != 1 {
		t.Fatalf("plan not applied: %+v", repo.applied)
	}
	if repo.report == nil {
		t.Fatal("report not persisted")
	}
}

func TestImportEndpointRejectsBadFile(t *testing.T) {
	repo := &fakeImportRepo{}
	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(repo))

	contentType, body := multipartFile(t, "lista.csv", []byte("nome,telefone\nEduardo,11999\n"))
	resp := api.Post("/import", "Content-Type: "+contentType, body)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad headers = %d, want 422: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "telefone") {
		t.Fatalf("expected PT-BR header detail: %s", resp.Body.String())
	}
	if repo.applied != nil {
		t.Fatal("bad file must not write")
	}
}
