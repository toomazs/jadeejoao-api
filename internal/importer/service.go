package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNameCollision: a guest insert hit the global normalized_name UNIQUE —
// the list changed between snapshot and apply. The whole import rolled back.
var ErrNameCollision = errors.New("guest name collision during import")

// Repo is the persistence surface for imports. Apply must run the whole plan
// in one transaction: a half-applied import is worse than a failed one.
type Repo interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	Apply(ctx context.Context, plan Plan, replace bool) error
	SaveReport(ctx context.Context, report []byte) error
}

// Options tunes one import run.
type Options struct {
	// Replace empties the guest list before writing, making the file the whole
	// truth instead of merging it into what is already there.
	//
	// This exists for the case where the sheet was rewritten wholesale — the
	// couple went from "Aldeane Nascimento" to "Aldeane Sousa do Nascimento"
	// for most of the list — and matching by name would produce a second copy
	// of nearly everyone instead of updates. Destructive: the old rows leave
	// and their RSVP answers with them, so it is deliberately not reachable
	// from the admin upload; only the operator CLI can ask for it.
	Replace bool
}

// Service runs uploads through parse → reconcile → apply.
type Service struct {
	repo Repo
}

// NewService wires the importer service.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// Import processes one uploaded file and returns the reconciliation report.
// Format/header problems abort with *ParseError (nothing written); row-level
// issues are reported and skip only their row.
func (s *Service) Import(ctx context.Context, filename string, data []byte, opts Options) (Report, error) {
	rows, err := ParseFile(filename, data)
	if err != nil {
		return Report{}, err
	}
	// Replacing means reconciling against nothing: every row is an insert, and
	// no existing name can be matched, conflicted or reported as unmatched.
	var snap Snapshot
	if !opts.Replace {
		snap, err = s.repo.Snapshot(ctx)
		if err != nil {
			return Report{}, fmt.Errorf("load snapshot: %w", err)
		}
	}
	plan, report := Reconcile(rows, snap)
	if err := s.repo.Apply(ctx, plan, opts.Replace); err != nil {
		return Report{}, fmt.Errorf("apply import: %w", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return Report{}, err
	}
	if err := s.repo.SaveReport(ctx, raw); err != nil {
		return Report{}, fmt.Errorf("save report: %w", err)
	}
	return report, nil
}
