package guests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestAdminDashboardAndExport(t *testing.T) {
	repo, m1, _ := newFixture()
	category := "child"
	for i := range repo.members {
		if repo.members[i].ID != m1 {
			repo.members[i].Category = &category
		}
	}
	repo.members[0].Attending = "yes"
	repo.members[1].Attending = "yes"

	_, api := humatest.New(t)
	RegisterAdmin(api, NewService(repo, fixedDeadline(""), nil, nil))

	resp := api.Get("/guests")
	if resp.Code != http.StatusOK {
		t.Fatalf("dashboard = %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"total":2`) || !strings.Contains(body, `"yes":2`) {
		t.Fatalf("headcounts wrong: %s", body)
	}
	if !strings.Contains(body, `"child":1`) || !strings.Contains(body, `"uncategorized":1`) {
		t.Fatalf("category counts wrong: %s", body)
	}

	resp = api.Get("/guests/export")
	if resp.Code != http.StatusOK {
		t.Fatalf("export = %d: %s", resp.Code, resp.Body.String())
	}
	csv := resp.Body.String()
	if !strings.Contains(csv, "grupo,nome,principal,categoria,presenca") {
		t.Fatalf("csv header missing: %s", csv)
	}
	if !strings.Contains(csv, "Eduardo e família,Eduardo Silva,sim,,sim") {
		t.Fatalf("csv row missing/wrong: %s", csv)
	}
	if !strings.Contains(csv, "criança") {
		t.Fatalf("category not translated: %s", csv)
	}
}
