package gifts

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/gifts/giftsdb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	pool *pgxpool.Pool
	q    *giftsdb.Queries
}

// NewRepo builds the Postgres-backed gifts repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{pool: pool, q: giftsdb.New(pool)}
}

func (r *pgRepo) ListGifts(ctx context.Context, onlyActive bool) ([]Gift, error) {
	rows, err := r.q.ListGiftsWithProgress(ctx, onlyActive)
	if err != nil {
		return nil, err
	}
	out := make([]Gift, len(rows))
	for i, row := range rows {
		out[i] = Gift{
			ID: row.ID, Title: row.Title, Description: row.Description, ImageURL: row.ImageUrl,
			GoalCentavos: row.GoalCentavos, QuotaCentavos: row.QuotaCentavos, MaxUnits: row.MaxUnits,
			Active: row.Active, Sort: row.Sort,
			DeclaredCentavos: row.DeclaredCentavos, ConfirmedCentavos: row.ConfirmedCentavos,
		}
	}
	return out, nil
}

func (r *pgRepo) GetGift(ctx context.Context, id uuid.UUID) (Gift, error) {
	row, err := r.q.GetGiftWithProgress(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Gift{}, ErrNotFound
	}
	if err != nil {
		return Gift{}, err
	}
	return Gift{
		ID: row.ID, Title: row.Title, Description: row.Description, ImageURL: row.ImageUrl,
		GoalCentavos: row.GoalCentavos, QuotaCentavos: row.QuotaCentavos, MaxUnits: row.MaxUnits,
		Active: row.Active, Sort: row.Sort,
		DeclaredCentavos: row.DeclaredCentavos, ConfirmedCentavos: row.ConfirmedCentavos,
	}, nil
}

// CreateContribution runs the commit point in one transaction: lock the gift
// row, re-validate against the locked state, insert the declared row. The
// SELECT … FOR UPDATE serializes concurrent commits so the last quota can
// never be double-claimed (single replica, AD-12).
func (r *pgRepo) CreateContribution(ctx context.Context, req NewContribution) (Contribution, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Contribution{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)

	gift, err := q.GetGiftForUpdate(ctx, req.GiftID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Contribution{}, ErrNotFound
	}
	if err != nil {
		return Contribution{}, err
	}
	if !gift.Active {
		return Contribution{}, ErrNotFound
	}
	if err := validateAmount(gift.QuotaCentavos, req.AmountCentavos); err != nil {
		return Contribution{}, err
	}
	sum, err := q.SumGiftNonCancelled(ctx, req.GiftID)
	if err != nil {
		return Contribution{}, err
	}
	if err := checkAvailability(gift.QuotaCentavos, gift.MaxUnits, sum, req.AmountCentavos); err != nil {
		return Contribution{}, err
	}

	groupID := uuid.NullUUID{}
	if req.GroupID != nil {
		groupID = uuid.NullUUID{UUID: *req.GroupID, Valid: true}
	}
	row, err := q.InsertContribution(ctx, giftsdb.InsertContributionParams{
		GiftID:          req.GiftID,
		GroupID:         groupID,
		ContributorName: req.ContributorName,
		AmountCentavos:  req.AmountCentavos,
	})
	if err != nil {
		return Contribution{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Contribution{}, err
	}
	out := Contribution{
		ID: row.ID, GiftID: row.GiftID, ContributorName: row.ContributorName,
		AmountCentavos: row.AmountCentavos, Status: row.Status,
	}
	if row.GroupID.Valid {
		id := row.GroupID.UUID
		out.GroupID = &id
	}
	return out, nil
}
