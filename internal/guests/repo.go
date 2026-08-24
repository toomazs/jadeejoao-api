package guests

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r *pgRepo) SuggestNames(ctx context.Context, normalizedPrefix string) ([]string, error) {
	return r.q.SuggestGuestNames(ctx, normalizedPrefix)
}

func (r *pgRepo) ListAllGroups(ctx context.Context) ([]Group, error) {
	rows, err := r.q.ListAllGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Group, len(rows))
	for i, row := range rows {
		out[i] = Group{ID: row.ID, Label: row.Label}
	}
	return out, nil
}

func (r *pgRepo) ListAllGuests(ctx context.Context) (map[uuid.UUID][]Member, error) {
	rows, err := r.q.ListAllGuests(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]Member)
	for _, row := range rows {
		out[row.GroupID] = append(out[row.GroupID], Member{
			ID: row.ID, FullName: row.FullName, IsPrimary: row.IsPrimary,
			Category: row.Category, Attending: row.Attending,
		})
	}
	return out, nil
}

// UpdateAttendances applies every answer in one transaction; any update that
// does not match exactly one guest of the group aborts the whole submission.
// AddCompanion inserts one guest the primary brought along, refusing once the
// invitation has spent its allowance.
//
// The count and the insert share a transaction that holds a row lock on the
// group, because the cheap version of this — count, then insert — lets two
// phones open the same invitation and each be told there is one slot left.
func (r *pgRepo) AddCompanion(ctx context.Context, groupID uuid.UUID, c NewCompanion) (Member, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Member{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	if _, err := q.LockGroupForCompanion(ctx, groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrNotFound
		}
		return Member{}, err
	}
	already, err := q.CountGroupCompanions(ctx, groupID)
	if err != nil {
		return Member{}, err
	}
	if already >= MaxCompanionsPerGroup {
		return Member{}, ErrCompanionLimit
	}

	inserted, err := q.InsertCompanion(ctx, guestsdb.InsertCompanionParams{
		GroupID:        groupID,
		FullName:       c.FullName,
		NormalizedName: Normalize(c.FullName),
		Attending:      c.Attending,
	})
	if err != nil {
		// normalized_name is globally unique: the name is already on the list,
		// which is a message for the guest, not a 500.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Member{}, ErrNameTaken
		}
		return Member{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Member{}, err
	}
	return Member{
		ID:        inserted.ID,
		FullName:  inserted.FullName,
		IsPrimary: inserted.IsPrimary,
		Category:  inserted.Category,
		Attending: inserted.Attending,
	}, nil
}

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
