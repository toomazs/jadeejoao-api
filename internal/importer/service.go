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
	Apply(ctx context.Context, plan Plan) error
	SaveReport(ctx context.Context, report []byte) error
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
func (s *Service) Import(ctx context.Context, filename string, data []byte) (Report, error) {
	rows, err := ParseFile(filename, data)
	if err != nil {
		return Report{}, err
	}
	snap, err := s.repo.Snapshot(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("load snapshot: %w", err)
	}
	plan, report := Reconcile(rows, snap)
	if err := s.repo.Apply(ctx, plan); err != nil {
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
