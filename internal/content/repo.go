package content

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/content/contentdb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	q *contentdb.Queries
}

// NewRepo builds the Postgres-backed content repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{q: contentdb.New(pool)}
}

func (r *pgRepo) ListSections(ctx context.Context) ([]Row, error) {
	rows, err := r.q.ListSections(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, row := range rows {
		out[i] = Row{Slug: row.Slug, Payload: row.Payload, Enabled: row.Enabled, UpdatedAt: row.UpdatedAt}
	}
	return out, nil
}

func (r *pgRepo) UpdateSection(ctx context.Context, slug string, payload []byte, enabled bool) (Row, error) {
	row, err := r.q.UpdateSection(ctx, contentdb.UpdateSectionParams{Slug: slug, Payload: payload, Enabled: enabled})
	if err != nil {
		return Row{}, err
	}
	return Row{Slug: row.Slug, Payload: row.Payload, Enabled: row.Enabled, UpdatedAt: row.UpdatedAt}, nil
}
