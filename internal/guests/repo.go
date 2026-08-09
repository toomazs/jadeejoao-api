package guests

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/guests/guestsdb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	pool *pgxpool.Pool
	q    *guestsdb.Queries
}

// NewRepo builds the Postgres-backed guests repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{pool: pool, q: guestsdb.New(pool)}
}

func (r *pgRepo) FindGuestByNormalizedName(ctx context.Context, normalized string) (Member, uuid.UUID, error) {
	row, err := r.q.GetGuestByNormalizedName(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, uuid.Nil, ErrNotFound
	}
	if err != nil {
		return Member{}, uuid.Nil, err
	}
	member := Member{ID: row.ID, FullName: row.FullName, IsPrimary: row.IsPrimary, Category: row.Category, Attending: row.Attending}
	return member, row.GroupID, nil
}

func (r *pgRepo) GetGroup(ctx context.Context, id uuid.UUID) (Group, error) {
	row, err := r.q.GetGuestGroup(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, err
	}
	return Group{ID: row.ID, Label: row.Label}, nil
}

func (r *pgRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]Member, error) {
	rows, err := r.q.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, row := range rows {
		out[i] = Member{ID: row.ID, FullName: row.FullName, IsPrimary: row.IsPrimary, Category: row.Category, Attending: row.Attending}
	}
	return out, nil
}

// UpdateAttendances applies every answer in one transaction; any update that
// does not match exactly one guest of the group aborts the whole submission.
func (r *pgRepo) UpdateAttendances(ctx context.Context, groupID uuid.UUID, updates []AttendanceUpdate) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	for _, u := range updates {
		affected, err := q.UpdateGuestAttendance(ctx, guestsdb.UpdateGuestAttendanceParams{
			ID:        u.GuestID,
			GroupID:   groupID,
			Attending: u.Attending,
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: guest %s", ErrInvalidMembers, u.GuestID)
		}
	}
	return tx.Commit(ctx)
}
