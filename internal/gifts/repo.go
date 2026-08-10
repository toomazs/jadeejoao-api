package gifts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/gifts/giftsdb"
)

// foreignKeyViolation is the Postgres SQLSTATE for FK errors — a fabricated
// group_id must surface as a clean domain error, not a 500.
const foreignKeyViolation = "23503"

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
			Kind: row.Kind, Platform: row.Platform, ExternalURL: row.ExternalUrl,
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
		Kind: row.Kind, Platform: row.Platform, ExternalURL: row.ExternalUrl,
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
	if gift.Kind == KindLink {
		return Contribution{}, ErrExternalGift
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
		return Contribution{}, ErrUnknownGroup
	}
	if err != nil {
		return Contribution{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Contribution{}, err
	}
	return contributionFromRow(row.ID, row.GiftID, row.GroupID, row.ContributorName,
		row.AmountCentavos, row.Status, row.CreatedAt, row.ConfirmedAt), nil
}

func (r *pgRepo) InsertGift(ctx context.Context, p GiftParams) (uuid.UUID, error) {
	return r.q.InsertGift(ctx, giftsdb.InsertGiftParams{
		Title: p.Title, Description: p.Description, ImageUrl: p.ImageURL,
		Kind: p.Kind, Platform: p.Platform, ExternalUrl: p.ExternalURL,
		GoalCentavos: p.GoalCentavos, QuotaCentavos: p.QuotaCentavos, MaxUnits: p.MaxUnits,
		Active: p.Active, Sort: p.Sort,
	})
}

// UpdateGift's statement refuses a kind flip on a gift with ledger rows
// (single conditional UPDATE — no check-then-act window against a concurrent
// Declare). Zero affected rows means unknown id or a locked kind.
func (r *pgRepo) UpdateGift(ctx context.Context, id uuid.UUID, p GiftParams) error {
	affected, err := r.q.UpdateGift(ctx, giftsdb.UpdateGiftParams{
		ID: id, Title: p.Title, Description: p.Description, ImageUrl: p.ImageURL,
		Kind: p.Kind, Platform: p.Platform, ExternalUrl: p.ExternalURL,
		GoalCentavos: p.GoalCentavos, QuotaCentavos: p.QuotaCentavos, MaxUnits: p.MaxUnits,
		Active: p.Active, Sort: p.Sort,
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		if _, getErr := r.q.GetGiftWithProgress(ctx, id); errors.Is(getErr, pgx.ErrNoRows) {
			return ErrNotFound
		} else if getErr != nil {
			return getErr
		}
		return ErrKindLocked
	}
	return nil
}

func (r *pgRepo) DeleteGiftIfNoContributions(ctx context.Context, id uuid.UUID) (bool, error) {
	affected, err := r.q.DeleteGiftIfNoContributions(ctx, id)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *pgRepo) DeactivateGift(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeactivateGift(ctx, id)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepo) ListContributions(ctx context.Context, statusFilter string) ([]ContributionDetail, error) {
	rows, err := r.q.ListContributions(ctx, statusFilter)
	if err != nil {
		return nil, err
	}
	out := make([]ContributionDetail, len(rows))
	for i, row := range rows {
		out[i] = ContributionDetail{
			Contribution: contributionFromRow(row.ID, row.GiftID, row.GroupID, row.ContributorName,
				row.AmountCentavos, row.Status, row.CreatedAt, row.ConfirmedAt),
			GiftTitle: row.GiftTitle,
		}
	}
	return out, nil
}

func (r *pgRepo) UpdateContributionStatus(ctx context.Context, id uuid.UUID, status string) (Contribution, error) {
	row, err := r.q.UpdateContributionStatus(ctx, giftsdb.UpdateContributionStatusParams{ID: id, Status: status})
	if errors.Is(err, pgx.ErrNoRows) {
		// Zero rows: either the id is unknown (404) or the transition is
		// illegal for the current status (409). Disambiguate.
		if _, getErr := r.q.GetContribution(ctx, id); errors.Is(getErr, pgx.ErrNoRows) {
			return Contribution{}, ErrNotFound
		} else if getErr != nil {
			return Contribution{}, getErr
		}
		return Contribution{}, ErrInvalidTransition
	}
	if err != nil {
		return Contribution{}, err
	}
	return contributionFromRow(row.ID, row.GiftID, row.GroupID, row.ContributorName,
		row.AmountCentavos, row.Status, row.CreatedAt, row.ConfirmedAt), nil
}

func contributionFromRow(id, giftID uuid.UUID, groupID uuid.NullUUID, name string, amount int64, status string, createdAt time.Time, confirmedAt *time.Time) Contribution {
	c := Contribution{
		ID: id, GiftID: giftID, ContributorName: name,
		AmountCentavos: amount, Status: status, CreatedAt: createdAt, ConfirmedAt: confirmedAt,
	}
	if groupID.Valid {
		gid := groupID.UUID
		c.GroupID = &gid
	}
	return c
}
