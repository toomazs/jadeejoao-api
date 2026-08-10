package gifts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

// fakeRepo mirrors pgRepo's commit-point semantics using the same pure
// validation helpers, so service and handler tests exercise realistic paths.
type fakeRepo struct {
	gifts    []Gift
	contribs []Contribution
	inserts  int
}

func (f *fakeRepo) ListGifts(_ context.Context, onlyActive bool) ([]Gift, error) {
	var out []Gift
	for _, g := range f.gifts {
		if !onlyActive || g.Active {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetGift(_ context.Context, id uuid.UUID) (Gift, error) {
	for _, g := range f.gifts {
		if g.ID == id {
			return g, nil
		}
	}
	return Gift{}, ErrNotFound
}

func (f *fakeRepo) CreateContribution(ctx context.Context, req NewContribution) (Contribution, error) {
	g, err := f.GetGift(ctx, req.GiftID)
	if err != nil {
		return Contribution{}, err
	}
	if !g.Active {
		return Contribution{}, ErrNotFound
	}
	if err := validateAmount(g.QuotaCentavos, req.AmountCentavos); err != nil {
		return Contribution{}, err
	}
	if err := checkAvailability(g.QuotaCentavos, g.MaxUnits, g.DeclaredCentavos+g.ConfirmedCentavos, req.AmountCentavos); err != nil {
		return Contribution{}, err
	}
	f.inserts++
	for i := range f.gifts {
		if f.gifts[i].ID == req.GiftID {
			f.gifts[i].DeclaredCentavos += req.AmountCentavos
		}
	}
	c := Contribution{ID: uuid.New(), GiftID: req.GiftID, GroupID: req.GroupID,
		ContributorName: req.ContributorName, AmountCentavos: req.AmountCentavos, Status: "declared"}
	f.contribs = append(f.contribs, c)
	return c, nil
}

func (f *fakeRepo) InsertGift(_ context.Context, p GiftParams) (uuid.UUID, error) {
	g := Gift{ID: uuid.New(), Title: p.Title, Description: p.Description, ImageURL: p.ImageURL,
		GoalCentavos: p.GoalCentavos, QuotaCentavos: p.QuotaCentavos, MaxUnits: p.MaxUnits,
		Active: p.Active, Sort: p.Sort}
	f.gifts = append(f.gifts, g)
	return g.ID, nil
}

func (f *fakeRepo) UpdateGift(_ context.Context, id uuid.UUID, p GiftParams) error {
	for i := range f.gifts {
		if f.gifts[i].ID == id {
			f.gifts[i].Title, f.gifts[i].Description, f.gifts[i].ImageURL = p.Title, p.Description, p.ImageURL
			f.gifts[i].GoalCentavos, f.gifts[i].QuotaCentavos, f.gifts[i].MaxUnits = p.GoalCentavos, p.QuotaCentavos, p.MaxUnits
			f.gifts[i].Active, f.gifts[i].Sort = p.Active, p.Sort
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) DeleteGiftIfNoContributions(_ context.Context, id uuid.UUID) (bool, error) {
	for _, c := range f.contribs {
		if c.GiftID == id {
			return false, nil
		}
	}
	for i := range f.gifts {
		if f.gifts[i].ID == id {
			f.gifts = append(f.gifts[:i], f.gifts[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRepo) DeactivateGift(_ context.Context, id uuid.UUID) error {
	for i := range f.gifts {
		if f.gifts[i].ID == id {
			f.gifts[i].Active = false
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) ListContributions(_ context.Context, statusFilter string) ([]ContributionDetail, error) {
	var out []ContributionDetail
	for _, c := range f.contribs {
		if statusFilter == "" || c.Status == statusFilter {
			out = append(out, ContributionDetail{Contribution: c})
		}
	}
	return out, nil
}

// UpdateContributionStatus mirrors the SQL transition guard: confirm only
// from declared, cancel from declared|confirmed.
func (f *fakeRepo) UpdateContributionStatus(_ context.Context, id uuid.UUID, status string) (Contribution, error) {
	for i := range f.contribs {
		if f.contribs[i].ID != id {
			continue
		}
		current := f.contribs[i].Status
		legal := (status == "confirmed" && current == "declared") ||
			(status == "cancelled" && (current == "declared" || current == "confirmed"))
		if !legal {
			return Contribution{}, ErrInvalidTransition
		}
		f.contribs[i].Status = status
		return f.contribs[i], nil
	}
	return Contribution{}, ErrNotFound
}

var testIdentity = PixIdentity{Key: "casamento@jadeejoao.com.br", MerchantName: "JADE E JOAO", MerchantCity: "ATIBAIA"}

func newGiftFixture() (*fakeRepo, Gift, Gift) {
	free := Gift{ID: uuid.New(), Title: "Barba", Active: true, GoalCentavos: ptr[int64](20000), Sort: 10}
	quota := Gift{ID: uuid.New(), Title: "Jantar", Active: true,
		GoalCentavos: ptr[int64](60000), QuotaCentavos: ptr[int64](15000), MaxUnits: ptr[int32](4),
		DeclaredCentavos: 45000, Sort: 20} // 3 of 4 units consumed
	return &fakeRepo{gifts: []Gift{free, quota}}, free, quota
}

func TestValidateAmount(t *testing.T) {
	cases := []struct {
		name   string
		quota  *int64
		amount int64
		want   error
	}{
		{"free gift accepts any positive", nil, 1, nil},
		{"zero rejected", nil, 0, ErrInvalidAmount},
		{"negative rejected", nil, -100, ErrInvalidAmount},
		{"exact quota", ptr[int64](15000), 15000, nil},
		{"quota multiple", ptr[int64](15000), 45000, nil},
		{"not a multiple", ptr[int64](15000), 20000, ErrInvalidAmount},
		{"below quota", ptr[int64](15000), 5000, ErrInvalidAmount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateAmount(tc.quota, tc.amount); !errors.Is(got, tc.want) && got != tc.want {
				t.Fatalf("validateAmount(%v, %d) = %v, want %v", tc.quota, tc.amount, got, tc.want)
			}
		})
	}
}

func TestResolveAmountDefaults(t *testing.T) {
	if got, err := resolveAmount(ptr[int64](15000), nil); err != nil || got != 15000 {
		t.Fatalf("quota default: got %d, %v", got, err)
	}
	if _, err := resolveAmount(nil, nil); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("free gift without amount must be rejected, got %v", err)
	}
	if got, err := resolveAmount(nil, ptr[int64](2500)); err != nil || got != 2500 {
		t.Fatalf("explicit free amount: got %d, %v", got, err)
	}
}

func TestCheckAvailabilityLastUnit(t *testing.T) {
	quota, max := ptr[int64](15000), ptr[int32](4)
	// 3 units consumed: exactly one left.
	if err := checkAvailability(quota, max, 45000, 15000); err != nil {
		t.Fatalf("last unit must be claimable: %v", err)
	}
	if err := checkAvailability(quota, max, 45000, 30000); !errors.Is(err, ErrNoUnitsLeft) {
		t.Fatalf("two units with one left: got %v, want ErrNoUnitsLeft", err)
	}
	if err := checkAvailability(quota, max, 60000, 15000); !errors.Is(err, ErrNoUnitsLeft) {
		t.Fatalf("sold out: got %v, want ErrNoUnitsLeft", err)
	}
	// No max_units: exceeding the goal is fine.
	if err := checkAvailability(quota, nil, 900000, 15000); err != nil {
		t.Fatalf("unlimited gift must accept: %v", err)
	}
}

func TestUnitsLeft(t *testing.T) {
	_, _, quotaGift := newGiftFixture()
	if left := quotaGift.UnitsLeft(); left == nil || *left != 1 {
		t.Fatalf("UnitsLeft = %v, want 1", left)
	}
	free := Gift{GoalCentavos: ptr[int64](20000)}
	if left := free.UnitsLeft(); left != nil {
		t.Fatalf("free gift UnitsLeft = %v, want nil", left)
	}
}

func TestPixPreviewIsSideEffectFree(t *testing.T) {
	repo, free, _ := newGiftFixture()
	svc := NewService(repo, testIdentity)

	code, amount, err := svc.PixPreview(context.Background(), free.ID, ptr[int64](5000))
	if err != nil {
		t.Fatalf("PixPreview: %v", err)
	}
	if amount != 5000 || !strings.HasPrefix(code, "000201") || !strings.Contains(code, "540550.00") {
		t.Fatalf("unexpected pix: amount=%d code=%s", amount, code)
	}
	if repo.inserts != 0 {
		t.Fatalf("preview wrote %d contributions — must be zero", repo.inserts)
	}
}

func TestPixPreviewDefaultsToOneQuota(t *testing.T) {
	repo, _, quotaGift := newGiftFixture()
	svc := NewService(repo, testIdentity)

	_, amount, err := svc.PixPreview(context.Background(), quotaGift.ID, nil)
	if err != nil {
		t.Fatalf("PixPreview: %v", err)
	}
	if amount != 15000 {
		t.Fatalf("default amount = %d, want one quota (15000)", amount)
	}
}

func TestDeclareHappyPath(t *testing.T) {
	repo, _, quotaGift := newGiftFixture()
	svc := NewService(repo, testIdentity)

	contribution, code, err := svc.Declare(context.Background(), quotaGift.ID, nil, "Eduardo Silva", nil)
	if err != nil {
		t.Fatalf("Declare: %v", err)
	}
	if contribution.Status != "declared" || contribution.AmountCentavos != 15000 {
		t.Fatalf("unexpected contribution: %+v", contribution)
	}
	if !strings.Contains(code, "5406150.00") {
		t.Fatalf("pix code amount wrong: %s", code)
	}
	if repo.inserts != 1 {
		t.Fatalf("inserts = %d, want 1", repo.inserts)
	}
}

func TestDeclareNoUnitsLeft(t *testing.T) {
	repo, _, quotaGift := newGiftFixture()
	svc := NewService(repo, testIdentity)

	// Two units requested with only one left.
	_, _, err := svc.Declare(context.Background(), quotaGift.ID, nil, "Eduardo", ptr[int64](30000))
	if !errors.Is(err, ErrNoUnitsLeft) {
		t.Fatalf("got %v, want ErrNoUnitsLeft", err)
	}
	if repo.inserts != 0 {
		t.Fatal("rejected declare must not write")
	}
}
