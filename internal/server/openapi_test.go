package server

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

// TestSpecFileInSync regenerates the OpenAPI spec in memory and fails when the
// committed openapi.yaml is stale. Fix with: go generate ./...
func TestSpecFileInSync(t *testing.T) {
	want, err := NewSpecAPI().OpenAPI().YAML()
	if err != nil {
		t.Fatalf("generate spec: %v", err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v (run: go generate ./...)", err)
	}
	normalize := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if !bytes.Equal(normalize(got), normalize(want)) {
		t.Fatal("openapi.yaml is stale — regenerate with: go generate ./...")
	}
}

func TestHealthz(t *testing.T) {
	_, api := humatest.New(t)
	Register(api, Deps{})

	resp := api.Get("/healthz")
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", resp.Code)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("unexpected body: %s", resp.Body.String())
	}
}
