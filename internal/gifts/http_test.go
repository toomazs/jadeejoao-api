package gifts

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func newTestAPI(t *testing.T, repo Repo) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterPublic(api, NewService(repo, testIdentity))
	return api
}

func TestListGiftsEndpoint(t *testing.T) {
	repo, _, _ := newGiftFixture()
	api := newTestAPI(t, repo)

	resp := api.Get("/api/v1/gifts")
	if resp.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"units_left":1`) || !strings.Contains(body, `"declared_centavos":45000`) {
		t.Fatalf("computed progress missing: %s", body)
	}
}

func TestPixPreviewEndpoint(t *testing.T) {
	repo, free, _ := newGiftFixture()
	api := newTestAPI(t, repo)

	resp := api.Get(fmt.Sprintf("/api/v1/gifts/%s/pix?amount_centavos=5000", free.ID))
	if resp.Code != http.StatusOK {
		t.Fatalf("preview = %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"pix_code":"000201`) {
		t.Fatalf("pix_code missing: %s", resp.Body.String())
	}
	if repo.inserts != 0 {
		t.Fatal("preview must not write")
	}

	// Free-amount gift without amount: 422 with PT-BR detail.
	resp = api.Get(fmt.Sprintf("/api/v1/gifts/%s/pix", free.ID))
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing amount = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Valor inválido") {
		t.Fatalf("expected PT-BR detail: %s", resp.Body.String())
	}
}

func TestDeclareEndpoint(t *testing.T) {
	repo, _, quotaGift := newGiftFixture()
	api := newTestAPI(t, repo)
	path := fmt.Sprintf("/api/v1/gifts/%s/contributions", quotaGift.ID)

	resp := api.Post(path, map[string]any{"contributor_name": "Eduardo Silva"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("declare = %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"status":"declared"`) || !strings.Contains(body, `"amount_centavos":15000`) {
		t.Fatalf("unexpected body: %s", body)
	}

	// Wrong multiple.
	resp = api.Post(path, map[string]any{"contributor_name": "Eduardo", "amount_centavos": 20000})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad amount = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "múltiplo da cota") {
		t.Fatalf("expected PT-BR quota detail: %s", resp.Body.String())
	}

	// The fixture now has zero units left (happy path above took the last one).
	resp = api.Post(path, map[string]any{"contributor_name": "Ana"})
	if resp.Code != http.StatusConflict {
		t.Fatalf("sold out = %d, want 409", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "cotas deste presente") {
		t.Fatalf("expected PT-BR sold-out detail: %s", resp.Body.String())
	}

	// contributor_name is required by schema validation.
	resp = api.Post(path, map[string]any{"amount_centavos": 15000})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing name = %d, want 422", resp.Code)
	}
}

func TestGiftNotFoundEndpoints(t *testing.T) {
	repo, _, _ := newGiftFixture()
	api := newTestAPI(t, repo)

	resp := api.Get("/api/v1/gifts/1e8f2c1a-0000-0000-0000-000000000000/pix?amount_centavos=100")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("unknown gift preview = %d, want 404", resp.Code)
	}
	resp = api.Post("/api/v1/gifts/1e8f2c1a-0000-0000-0000-000000000000/contributions",
		map[string]any{"contributor_name": "Eduardo"})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("unknown gift declare = %d, want 404", resp.Code)
	}
}
