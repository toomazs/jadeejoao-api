// Package gifts owns the gift list, computed progress, PIX generation, and
// the contribution ledger. A gift is a goal (meta), optionally sold in fixed
// quotas (cotas); progress is always computed from contributions, never
// stored (AD-6).
package gifts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform/pix"
)

var (
	// ErrNotFound: gift does not exist or is inactive (public surface treats
	// both the same to avoid leaking catalog state).
	ErrNotFound = errors.New("gift not found")
	// ErrInvalidAmount: amount missing, non-positive, or not n × quota.
	ErrInvalidAmount = errors.New("invalid contribution amount")
	// ErrNoUnitsLeft: the gift's remaining quotas cannot cover the request.
	ErrNoUnitsLeft = errors.New("no quota units left")
	// ErrInvalidGift: inconsistent gift configuration (e.g. max_units without
	// quota_centavos).
	ErrInvalidGift = errors.New("invalid gift configuration")
)

// Gift is a gift with its computed progress.
type Gift struct {
	ID                uuid.UUID
	Title             string
	Description       string
	ImageURL          *string
	GoalCentavos      *int64
	QuotaCentavos     *int64
	MaxUnits          *int32
	Active            bool
	Sort              int32
	DeclaredCentavos  int64
	ConfirmedCentavos int64
}

// UnitsLeft returns the remaining quota units, or nil when the gift is not
// unit-limited. Public progress is optimistic: declared + confirmed consume
// units; cancelled never counts.
func (g Gift) UnitsLeft() *int64 {
	if g.MaxUnits == nil || g.QuotaCentavos == nil || *g.QuotaCentavos == 0 {
		return nil
	}
	used := (g.DeclaredCentavos + g.ConfirmedCentavos) / *g.QuotaCentavos
	left := int64(*g.MaxUnits) - used
	if left < 0 {
		left = 0
	}
	return &left
}

// Contribution is one ledger entry.
type Contribution struct {
	ID              uuid.UUID
	GiftID          uuid.UUID
	GroupID         *uuid.UUID
	ContributorName string
	AmountCentavos  int64
	Status          string
	CreatedAt       time.Time
	ConfirmedAt     *time.Time
}

// ContributionDetail is a ledger entry joined with its gift title, for the
// admin dashboard.
type ContributionDetail struct {
	Contribution
	GiftTitle string
}

// GiftParams carries the admin-editable fields of a gift.
type GiftParams struct {
	Title         string
	Description   string
	ImageURL      *string
	GoalCentavos  *int64
	QuotaCentavos *int64
	MaxUnits      *int32
	Active        bool
	Sort          int32
}

// DeleteOutcome reports what DeleteGift actually did.
type DeleteOutcome string

const (
	// OutcomeDeleted: the gift had no contributions and was removed.
	OutcomeDeleted DeleteOutcome = "deleted"
	// OutcomeDeactivated: the gift has ledger history and was soft-deleted.
	OutcomeDeactivated DeleteOutcome = "deactivated"
)

// NewContribution is the commit-point request.
type NewContribution struct {
	GiftID          uuid.UUID
	GroupID         *uuid.UUID
	ContributorName string
	AmountCentavos  int64
}

// Repo is the persistence surface for gifts. CreateContribution must lock the
// gift row (SELECT … FOR UPDATE) and re-validate inside the transaction so
// the last quota cannot be double-claimed.
type Repo interface {
	ListGifts(ctx context.Context, onlyActive bool) ([]Gift, error)
	GetGift(ctx context.Context, id uuid.UUID) (Gift, error)
	CreateContribution(ctx context.Context, req NewContribution) (Contribution, error)

	InsertGift(ctx context.Context, p GiftParams) (uuid.UUID, error)
	UpdateGift(ctx context.Context, id uuid.UUID, p GiftParams) error
	CountContributions(ctx context.Context, giftID uuid.UUID) (int64, error)
	DeleteGiftRow(ctx context.Context, id uuid.UUID) error
	DeactivateGift(ctx context.Context, id uuid.UUID) error
	ListContributions(ctx context.Context, statusFilter string) ([]ContributionDetail, error)
	UpdateContributionStatus(ctx context.Context, id uuid.UUID, status string) (Contribution, error)
}

// PixIdentity is the couple's PIX identity, from env.
type PixIdentity struct {
	Key          string
	MerchantName string
	MerchantCity string
}

// Service implements the public gift surface.
type Service struct {
	repo Repo
	pix  PixIdentity
}

// NewService wires the gifts service.
func NewService(repo Repo, identity PixIdentity) *Service {
	return &Service{repo: repo, pix: identity}
}

// ListGifts returns active gifts with computed progress.
func (s *Service) ListGifts(ctx context.Context) ([]Gift, error) {
	return s.repo.ListGifts(ctx, true)
}

// PixPreview renders the copia-e-cola for window-shopping. Side-effect-free:
// nothing is written, ever. amount may be nil for quota gifts (defaults to
// one quota).
func (s *Service) PixPreview(ctx context.Context, giftID uuid.UUID, amount *int64) (string, int64, error) {
	gift, err := s.repo.GetGift(ctx, giftID)
	if err != nil {
		return "", 0, err
	}
	if !gift.Active {
		return "", 0, ErrNotFound
	}
	value, err := resolveAmount(gift.QuotaCentavos, amount)
	if err != nil {
		return "", 0, err
	}
	if err := checkAvailability(gift.QuotaCentavos, gift.MaxUnits, gift.DeclaredCentavos+gift.ConfirmedCentavos, value); err != nil {
		return "", 0, err
	}
	return s.brCode(value), value, nil
}

// Declare is the guest's "já fiz o PIX" commit point: validates and inserts a
// declared contribution under the gift row lock, then returns the same PIX
// string for convenience.
func (s *Service) Declare(ctx context.Context, giftID uuid.UUID, groupID *uuid.UUID, contributorName string, amount *int64) (Contribution, string, error) {
	gift, err := s.repo.GetGift(ctx, giftID)
	if err != nil {
		return Contribution{}, "", err
	}
	value, err := resolveAmount(gift.QuotaCentavos, amount)
	if err != nil {
		return Contribution{}, "", err
	}
	contribution, err := s.repo.CreateContribution(ctx, NewContribution{
		GiftID:          giftID,
		GroupID:         groupID,
		ContributorName: contributorName,
		AmountCentavos:  value,
	})
	if err != nil {
		return Contribution{}, "", err
	}
	return contribution, s.brCode(value), nil
}

// AdminListGifts returns every gift, including deactivated ones.
func (s *Service) AdminListGifts(ctx context.Context) ([]Gift, error) {
	return s.repo.ListGifts(ctx, false)
}

// CreateGift validates and creates a gift, returning it with (zero) progress.
func (s *Service) CreateGift(ctx context.Context, p GiftParams) (Gift, error) {
	if err := validateGiftParams(p); err != nil {
		return Gift{}, err
	}
	id, err := s.repo.InsertGift(ctx, p)
	if err != nil {
		return Gift{}, err
	}
	return s.repo.GetGift(ctx, id)
}

// UpdateGift validates and replaces a gift's editable fields.
func (s *Service) UpdateGift(ctx context.Context, id uuid.UUID, p GiftParams) (Gift, error) {
	if err := validateGiftParams(p); err != nil {
		return Gift{}, err
	}
	if err := s.repo.UpdateGift(ctx, id, p); err != nil {
		return Gift{}, err
	}
	return s.repo.GetGift(ctx, id)
}

// DeleteGift hard-deletes a gift with no contributions; a gift with ledger
// history is deactivated instead — the contribution ledger is append-only
// and never loses its referent (AD-6).
func (s *Service) DeleteGift(ctx context.Context, id uuid.UUID) (DeleteOutcome, error) {
	if _, err := s.repo.GetGift(ctx, id); err != nil {
		return "", err
	}
	count, err := s.repo.CountContributions(ctx, id)
	if err != nil {
		return "", err
	}
	if count == 0 {
		if err := s.repo.DeleteGiftRow(ctx, id); err != nil {
			return "", err
		}
		return OutcomeDeleted, nil
	}
	if err := s.repo.DeactivateGift(ctx, id); err != nil {
		return "", err
	}
	return OutcomeDeactivated, nil
}

// ListContributions returns the ledger for the admin, optionally filtered by
// status.
func (s *Service) ListContributions(ctx context.Context, statusFilter string) ([]ContributionDetail, error) {
	return s.repo.ListContributions(ctx, statusFilter)
}

// ModerateContribution transitions one ledger entry to confirmed or
// cancelled (admin only; the transport enforces the enum). Confirming stamps
// confirmed_at.
func (s *Service) ModerateContribution(ctx context.Context, id uuid.UUID, status string) (Contribution, error) {
	return s.repo.UpdateContributionStatus(ctx, id, status)
}

// validateGiftParams rejects configurations the domain cannot honor.
func validateGiftParams(p GiftParams) error {
	if p.MaxUnits != nil && p.QuotaCentavos == nil {
		return ErrInvalidGift
	}
	return nil
}

func (s *Service) brCode(amountCentavos int64) string {
	return pix.BRCode(pix.Params{
		Key:            s.pix.Key,
		MerchantName:   s.pix.MerchantName,
		MerchantCity:   s.pix.MerchantCity,
		AmountCentavos: amountCentavos,
	})
}

// resolveAmount applies the amount rule: free when quota is null (explicit
// amount required), otherwise n × quota with one quota as the default.
func resolveAmount(quota *int64, provided *int64) (int64, error) {
	if provided == nil {
		if quota != nil {
			return *quota, nil
		}
		return 0, ErrInvalidAmount
	}
	if err := validateAmount(quota, *provided); err != nil {
		return 0, err
	}
	return *provided, nil
}

// validateAmount enforces: positive, and an exact quota multiple when the
// gift is sold in quotas.
func validateAmount(quota *int64, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if quota == nil {
		return nil
	}
	if *quota <= 0 || amount%*quota != 0 {
		return ErrInvalidAmount
	}
	return nil
}

// checkAvailability enforces max_units on quota gifts: the requested units
// must fit in what non-cancelled contributions have not yet consumed. Gifts
// without max_units may exceed their goal freely.
func checkAvailability(quota *int64, maxUnits *int32, sumNonCancelled, amount int64) error {
	if maxUnits == nil || quota == nil || *quota == 0 {
		return nil
	}
	used := sumNonCancelled / *quota
	requested := amount / *quota
	if used+requested > int64(*maxUnits) {
		return ErrNoUnitsLeft
	}
	return nil
}
