package gifts

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func newAdminAPI(t *testing.T, repo *fakeRepo) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(repo, testIdentity))
	return api
}

func TestAdminGiftCRUD(t *testing.T) {
	repo := &fakeRepo{}
	api := newAdminAPI(t, repo)

	// Create.
	resp := api.Post("/gifts", map[string]any{
		"title": "Cafeteira dos noivos", "goal_centavos": 50000, "active": true, "sort": 5,
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.Code, resp.Body.String())
	}

	// max_units without quota is rejected.
	resp = api.Post("/gifts", map[string]any{"title": "Inválido", "max_units": 3, "active": true, "sort": 1})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid config = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "quota_centavos") {
		t.Fatalf("expected config detail: %s", resp.Body.String())
	}

	// Update (kind stated explicitly, as the contract now requires).
	id := repo.gifts[0].ID
	resp = api.Put(fmt.Sprintf("/gifts/%s", id), map[string]any{
		"title": "Cafeteira turbinada", "kind": "pix", "goal_centavos": 60000, "active": false, "sort": 5,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update = %d: %s", resp.Code, resp.Body.String())
	}
	if repo.gifts[0].Title != "Cafeteira turbinada" || repo.gifts[0].Active {
		t.Fatalf("update not applied: %+v", repo.gifts[0])
	}

	// PUT without kind: 422 with the explicit-kind message.
	resp = api.Put(fmt.Sprintf("/gifts/%s", id), map[string]any{
		"title": "Sem kind", "active": true, "sort": 5,
	})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("kindless update = %d, want 422", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Informe o tipo") {
		t.Fatalf("expected explicit-kind detail: %s", resp.Body.String())
	}
}

func TestAdminDeleteGiftSoftDeleteRule(t *testing.T) {
	repo, _, quotaGift := newGiftFixture()
	api := newAdminAPI(t, repo)

	// Gift with a contribution: deactivated, never hard-deleted.
	if _, err := repo.CreateContribution(context.Background(), NewContribution{
		GiftID: quotaGift.ID, ContributorName: "Eduardo", AmountCentavos: 15000,
	}); err != nil {
		t.Fatalf("seed contribution: %v", err)
	}
	resp := api.Delete(fmt.Sprintf("/gifts/%s", quotaGift.ID))
	if resp.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"outcome":"deactivated"`) {
		t.Fatalf("want deactivated outcome: %s", resp.Body.String())
	}
	if len(repo.gifts) != 2 {
		t.Fatal("gift with contributions must not be hard-deleted")
	}

	// Gift without contributions: hard delete.
	free := repo.gifts[0]
	resp = api.Delete(fmt.Sprintf("/gifts/%s", free.ID))
	if !strings.Contains(resp.Body.String(), `"outcome":"deleted"`) {
		t.Fatalf("want deleted outcome: %s", resp.Body.String())
	}
	if len(repo.gifts) != 1 {
		t.Fatalf("gift not removed: %+v", repo.gifts)
	}
}

func TestAdminContributionModeration(t *testing.T) {
	repo, _, quotaGift := newGiftFixture()
	api := newAdminAPI(t, repo)

	c, err := repo.CreateContribution(context.Background(), NewContribution{
		GiftID: quotaGift.ID, ContributorName: "Eduardo", AmountCentavos: 15000,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := api.Get("/contributions?status=declared")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Eduardo") {
		t.Fatalf("list = %d: %s", resp.Code, resp.Body.String())
	}

	resp = api.Patch(fmt.Sprintf("/contributions/%s", c.ID), map[string]any{"status": "confirmed"})
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"status":"confirmed"`) {
		t.Fatalf("confirm = %d: %s", resp.Code, resp.Body.String())
	}

	// Back to waiting is allowed now: confirming the wrong line is a mistake
	// somebody has to be able to take back.
	resp = api.Patch(fmt.Sprintf("/contributions/%s", c.ID), map[string]any{"status": "declared"})
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"status":"declared"`) {
		t.Fatalf("back to declared = %d: %s", resp.Code, resp.Body.String())
	}

	// A status that does not exist still is not one.
	resp = api.Patch(fmt.Sprintf("/contributions/%s", c.ID), map[string]any{"status": "sla"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown status = %d, want 422", resp.Code)
	}

	resp = api.Patch("/contributions/1e8f2c1a-0000-0000-0000-000000000000", map[string]any{"status": "cancelled"})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("unknown = %d, want 404", resp.Code)
	}
}
