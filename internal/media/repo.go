package media

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/media/mediadb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	q *mediadb.Queries
}

// NewRepo builds the Postgres-backed media repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{q: mediadb.New(pool)}
}

func (r *pgRepo) Insert(ctx context.Context, bucketPath, publicURL string, alt *string) (Media, error) {
	row, err := r.q.InsertMedia(ctx, mediadb.InsertMediaParams{
		BucketPath: bucketPath,
		PublicUrl:  publicURL,
		Alt:        alt,
	})
	if err != nil {
		return Media{}, err
	}
	return Media{ID: row.ID, BucketPath: row.BucketPath, PublicURL: row.PublicUrl, Alt: row.Alt, CreatedAt: row.CreatedAt}, nil
}

func (r *pgRepo) List(ctx context.Context) ([]Media, error) {
	rows, err := r.q.ListMedia(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Media, len(rows))
	for i, row := range rows {
		out[i] = Media{ID: row.ID, BucketPath: row.BucketPath, PublicURL: row.PublicUrl, Alt: row.Alt, CreatedAt: row.CreatedAt}
	}
	return out, nil
}

func (r *pgRepo) Get(ctx context.Context, id uuid.UUID) (Media, error) {
	row, err := r.q.GetMedia(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, ErrNotFound
	}
	if err != nil {
		return Media{}, err
	}
	return Media{ID: row.ID, BucketPath: row.BucketPath, PublicURL: row.PublicUrl, Alt: row.Alt, CreatedAt: row.CreatedAt}, nil
}

func (r *pgRepo) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteMedia(ctx, id)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
