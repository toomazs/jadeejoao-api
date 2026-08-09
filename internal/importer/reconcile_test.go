package importer

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func strPtr(s string) *string { return &s }

func row(line int, nome, grupo string, principal bool, categoria *string) Row {
	return Row{Line: line, Nome: nome, Grupo: grupo, Principal: principal, Categoria: categoria}
}

func TestReconcileFreshDatabase(t *testing.T) {
	rows := []Row{
		row(2, "Eduardo Silva", "Família Silva", true, strPtr("adult")),
		row(3, "Ana Clara Silva", "Família Silva", false, strPtr("child")),
		row(4, "João Pedro Costa", "", false, nil),
	}
	plan, report := Reconcile(rows, Snapshot{})

	if len(report.Added) != 3 || len(report.Updated) != 0 || len(report.Conflicts) != 0 {
		t.Fatalf("report wrong: %+v", report)
	}
	if len(plan.Groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(plan.Groups))
	}
	family := plan.Groups[0]
	if family.ExistingID != nil || family.Label != "Família Silva" || len(family.Guests) != 2 {
		t.Fatalf("family plan wrong: %+v", family)
	}
	if !family.Guests[0].IsPrimary || family.Guests[1].IsPrimary {
		t.Fatalf("primary marking wrong: %+v", family.Guests)
	}
	// Own group: label = the guest's name, guest is primary.
	own := plan.Groups[1]
	if own.Label != "João Pedro Costa" || !own.Guests[0].IsPrimary {
		t.Fatalf("own-group plan wrong: %+v", own)
	}
}

func TestReconcileUpdatesSameGroupIdentity(t *testing.T) {
	groupID, guestID := uuid.New(), uuid.New()
	snap := Snapshot{
		Groups: []ExistingGroup{{ID: groupID, Label: "Família Silva"}},
		Guests: []ExistingGuest{{
			ID: guestID, GroupID: groupID, FullName: "eduardo silva",
			NormalizedName: "eduardo silva", IsPrimary: true,
		}},
	}
	// Same person, fixed casing + category; group label matched case-insensitively.
	rows := []Row{row(2, "Eduardo Silva", "família silva", true, strPtr("adult"))}
	plan, report := Reconcile(rows, snap)

	if len(report.Updated) != 1 || len(report.Added) != 0 {
		t.Fatalf("report wrong: %+v", report)
	}
	g := plan.Groups[0]
	if g.ExistingID == nil || *g.ExistingID != groupID {
		t.Fatalf("existing group not matched: %+v", g)
	}
	u := g.Guests[0]
	if u.ExistingID == nil || *u.ExistingID != guestID || u.FullName != "Eduardo Silva" || *u.Category != "adult" {
		t.Fatalf("identity update wrong: %+v", u)
	}
	if len(report.Unmatched) != 0 {
		t.Fatalf("matched guest reported unmatched: %+v", report.Unmatched)
	}
}

func TestReconcileHomonymConflictNeverWrites(t *testing.T) {
	groupA, guestID := uuid.New(), uuid.New()
	snap := Snapshot{
		Groups: []ExistingGroup{{ID: groupA, Label: "Família Costa"}},
		Guests: []ExistingGuest{{
			ID: guestID, GroupID: groupA, FullName: "Maria Silva",
			NormalizedName: "maria silva", IsPrimary: true,
		}},
	}
	// Same normalized name arriving under a different group.
	rows := []Row{row(2, "MARIA SILVA", "Família Silva", true, nil)}
	plan, report := Reconcile(rows, snap)

	if len(report.Conflicts) != 1 {
		t.Fatalf("want 1 conflict: %+v", report)
	}
	c := report.Conflicts[0]
	if c.FileGroup != "Família Silva" || c.DBGroup != "Família Costa" || !strings.Contains(c.Reason, "outro grupo") {
		t.Fatalf("conflict wrong: %+v", c)
	}
	if len(plan.Groups) != 0 {
		t.Fatalf("conflicting row must not produce writes: %+v", plan.Groups)
	}
	// The name IS mentioned in the file, so the DB guest is not double-reported
	// as unmatched — the conflict entry is the actionable signal.
	if len(report.Unmatched) != 0 {
		t.Fatalf("conflicted guest must not also appear as unmatched: %+v", report.Unmatched)
	}
}

func TestReconcileDuplicateInFile(t *testing.T) {
	rows := []Row{
		row(2, "Eduardo Silva", "Família Silva", false, nil),
		row(3, "eduardo  SILVA", "Outro Grupo", false, nil),
	}
	plan, report := Reconcile(rows, Snapshot{})

	if len(report.Conflicts) != 1 || !strings.Contains(report.Conflicts[0].Reason, "duplicado") {
		t.Fatalf("duplicate not reported: %+v", report.Conflicts)
	}
	if len(report.Added) != 1 || len(plan.Groups) != 1 {
		t.Fatalf("first occurrence should still import: %+v", report)
	}
}

func TestReconcilePrimaryFallsBackWhenMarkedRowConflicts(t *testing.T) {
	groupA := uuid.New()
	snap := Snapshot{
		Groups: []ExistingGroup{{ID: groupA, Label: "Outro Grupo"}},
		Guests: []ExistingGuest{{
			ID: uuid.New(), GroupID: groupA, FullName: "Chefe da Família",
			NormalizedName: "chefe da familia", IsPrimary: true,
		}},
	}
	rows := []Row{
		row(2, "Chefe da Família", "Família Nova", true, nil), // conflicts (other group)
		row(3, "Convidado Sobrevivente", "Família Nova", false, nil),
	}
	plan, report := Reconcile(rows, snap)

	if len(report.Conflicts) != 1 {
		t.Fatalf("want conflict: %+v", report)
	}
	g := plan.Groups[0]
	if len(g.Guests) != 1 || !g.Guests[0].IsPrimary {
		t.Fatalf("surviving guest must become primary: %+v", g.Guests)
	}
}

func TestReconcileNeverTouchesAttendance(t *testing.T) {
	// Structural guarantee: the plan types carry no attendance fields, so the
	// importer cannot write them. This test documents the ownership split.
	var g GuestPlan
	_ = g.FullName
	_ = g.NormalizedName
	_ = g.Category
	_ = g.IsPrimary
	// No Attending / RespondedAt fields exist on GuestPlan by design (AD-10).
}
