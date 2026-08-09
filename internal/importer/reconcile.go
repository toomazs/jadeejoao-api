package importer

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/guests"
)

// ExistingGroup is a guest group as it exists in the database.
type ExistingGroup struct {
	ID    uuid.UUID
	Label string
}

// ExistingGuest is a guest as it exists in the database.
type ExistingGuest struct {
	ID             uuid.UUID
	GroupID        uuid.UUID
	FullName       string
	NormalizedName string
	IsPrimary      bool
	Category       *string
}

// Snapshot is the database state reconciliation runs against.
type Snapshot struct {
	Groups []ExistingGroup
	Guests []ExistingGuest
}

// GuestPlan is one guest write inside a group plan.
type GuestPlan struct {
	ExistingID     *uuid.UUID // nil = insert
	FullName       string
	NormalizedName string
	Category       *string
	IsPrimary      bool
}

// GroupPlan is every write targeting one guest group.
type GroupPlan struct {
	ExistingID *uuid.UUID // nil = create with Label
	Label      string
	Guests     []GuestPlan
}

// Plan is the full set of writes an import will perform.
type Plan struct {
	Groups []GroupPlan
}

// Conflict is a row the import refuses to write.
type Conflict struct {
	Name      string `json:"name"`
	FileGroup string `json:"file_group,omitempty"`
	DBGroup   string `json:"db_group,omitempty"`
	Reason    string `json:"reason"`
}

// Report is what the admin sees after an import (also stored in imports).
type Report struct {
	Added     []string   `json:"added"`
	Updated   []string   `json:"updated"`
	Unmatched []string   `json:"unmatched"`
	Conflicts []Conflict `json:"conflicts"`
	Errors    []RowIssue `json:"errors"`
}

// Reconcile diffs parsed rows against the database snapshot and produces the
// write plan plus the report. Pure function — all decisions happen here.
//
// Rules (AD-10): matched by normalized name; identity fields only (the
// importer never touches attendance); a name living in a different group is a
// reported conflict, never a write; DB guests absent from the file are
// reported as unmatched, never deleted.
func Reconcile(rows []Row, snap Snapshot) (Plan, Report) {
	var report Report

	groupByID := make(map[uuid.UUID]ExistingGroup, len(snap.Groups))
	groupByNormLabel := make(map[string]ExistingGroup, len(snap.Groups))
	for _, g := range snap.Groups {
		groupByID[g.ID] = g
		if key := guests.Normalize(g.Label); key != "" {
			if _, taken := groupByNormLabel[key]; !taken {
				groupByNormLabel[key] = g
			}
		}
	}
	guestByNorm := make(map[string]ExistingGuest, len(snap.Guests))
	for _, g := range snap.Guests {
		guestByNorm[g.NormalizedName] = g
	}

	// Group the file rows. Empty grupo = the guest is their own group.
	type fileGroup struct {
		label string
		rows  []Row
	}
	var order []string
	grouped := map[string]*fileGroup{}
	seenInFile := map[string]int{} // normalized name -> line
	fileNames := map[string]bool{}

	for _, row := range rows {
		report.Errors = append(report.Errors, row.Issues...)
		if row.Nome == "" {
			continue
		}
		normalized := guests.Normalize(row.Nome)
		if line, dup := seenInFile[normalized]; dup {
			report.Conflicts = append(report.Conflicts, Conflict{
				Name:   row.Nome,
				Reason: fmt.Sprintf("nome duplicado na planilha (linhas %d e %d)", line, row.Line),
			})
			continue
		}
		seenInFile[normalized] = row.Line
		fileNames[normalized] = true

		label := row.Grupo
		if label == "" {
			label = row.Nome
		}
		key := guests.Normalize(label)
		fg, ok := grouped[key]
		if !ok {
			fg = &fileGroup{label: label}
			grouped[key] = fg
			order = append(order, key)
		}
		fg.rows = append(fg.rows, row)
	}

	var plan Plan
	for _, key := range order {
		fg := grouped[key]

		gp := GroupPlan{Label: fg.label}
		if existing, ok := groupByNormLabel[key]; ok {
			id := existing.ID
			gp.ExistingID = &id
		}

		// Filter conflicts first: a name living in a different group is
		// reported and never written.
		type survivor struct {
			row        Row
			existingID *uuid.UUID
		}
		var survivors []survivor
		for _, row := range fg.rows {
			normalized := guests.Normalize(row.Nome)
			s := survivor{row: row}
			if existing, ok := guestByNorm[normalized]; ok {
				sameGroup := gp.ExistingID != nil && existing.GroupID == *gp.ExistingID
				if !sameGroup {
					report.Conflicts = append(report.Conflicts, Conflict{
						Name:      row.Nome,
						FileGroup: fg.label,
						DBGroup:   groupByID[existing.GroupID].Label,
						Reason:    "já existe em outro grupo — resolva com um qualificador (ex.: \"Maria Silva (mãe do João)\") ou ajuste manualmente",
					})
					continue
				}
				id := existing.ID
				s.existingID = &id
			}
			survivors = append(survivors, s)
		}

		// Choose the group's primary among the survivors: first row marked
		// principal wins, else the first survivor.
		primaryIdx := 0
		for i, s := range survivors {
			if s.row.Principal {
				primaryIdx = i
				break
			}
		}

		for i, s := range survivors {
			guestPlan := GuestPlan{
				ExistingID:     s.existingID,
				FullName:       s.row.Nome,
				NormalizedName: guests.Normalize(s.row.Nome),
				Category:       s.row.Categoria,
				IsPrimary:      i == primaryIdx,
			}
			if s.existingID != nil {
				report.Updated = append(report.Updated, s.row.Nome)
			} else {
				report.Added = append(report.Added, s.row.Nome)
			}
			gp.Guests = append(gp.Guests, guestPlan)
		}

		if len(gp.Guests) > 0 {
			plan.Groups = append(plan.Groups, gp)
		}
	}

	// DB guests the file no longer mentions: reported, never deleted.
	for _, g := range snap.Guests {
		if !fileNames[g.NormalizedName] {
			report.Unmatched = append(report.Unmatched, g.FullName)
		}
	}

	return plan, report
}
