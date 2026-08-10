package guests

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepo struct {
	group   Group
	members []Member
	applied []AttendanceUpdate
}

func (f *fakeRepo) FindGuestByNormalizedName(_ context.Context, normalized string) (Member, uuid.UUID, error) {
	for _, m := range f.members {
		if Normalize(m.FullName) == normalized {
			return m, f.group.ID, nil
		}
	}
	return Member{}, uuid.Nil, ErrNotFound
}

func (f *fakeRepo) GetGroup(_ context.Context, id uuid.UUID) (Group, error) {
	if id != f.group.ID {
		return Group{}, ErrNotFound
	}
	return f.group, nil
}

func (f *fakeRepo) ListMembers(_ context.Context, groupID uuid.UUID) ([]Member, error) {
	if groupID != f.group.ID {
		return nil, nil
	}
	return f.members, nil
}

func (f *fakeRepo) UpdateAttendances(_ context.Context, _ uuid.UUID, updates []AttendanceUpdate) error {
	f.applied = updates
	for _, u := range updates {
		for i := range f.members {
			if f.members[i].ID == u.GuestID {
				f.members[i].Attending = u.Attending
			}
		}
	}
	return nil
}

func (f *fakeRepo) SuggestNames(_ context.Context, normalizedPrefix string) ([]string, error) {
	var out []string
	for _, m := range f.members {
		if len(out) < 8 && strings.HasPrefix(Normalize(m.FullName), normalizedPrefix) {
			out = append(out, m.FullName)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListAllGroups(_ context.Context) ([]Group, error) {
	return []Group{f.group}, nil
}

func (f *fakeRepo) ListAllGuests(_ context.Context) (map[uuid.UUID][]Member, error) {
	return map[uuid.UUID][]Member{f.group.ID: f.members}, nil
}

type fixedDeadline string

func (d fixedDeadline) RSVPDeadline(context.Context) (string, error) { return string(d), nil }

func newFixture() (*fakeRepo, uuid.UUID, uuid.UUID) {
	g1, g2 := uuid.New(), uuid.New()
	repo := &fakeRepo{
		group: Group{ID: uuid.New(), Label: "Eduardo e família"},
		members: []Member{
			{ID: g1, FullName: "Eduardo Silva", IsPrimary: true, Attending: "pending"},
			{ID: g2, FullName: "Ana Clara Silva", Attending: "pending"},
		},
	}
	return repo, g1, g2
}

func at(day string) func() time.Time {
	return func() time.Time {
		ts, err := time.ParseInLocation("2006-01-02 15:04", day, saoPaulo)
		if err != nil {
			panic(err)
		}
		return ts
	}
}

func TestLookupNormalizesInput(t *testing.T) {
	repo, _, _ := newFixture()
	svc := NewService(repo, fixedDeadline("2027-07-07"), nil, nil)

	group, members, err := svc.Lookup(context.Background(), "  EDUARDO   sílva ")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if group.Label != "Eduardo e família" || len(members) != 2 {
		t.Fatalf("unexpected result: %+v %+v", group, members)
	}
}

func TestLookupMissAndEmpty(t *testing.T) {
	repo, _, _ := newFixture()
	svc := NewService(repo, fixedDeadline(""), nil, nil)

	if _, _, err := svc.Lookup(context.Background(), "Fulano Inexistente"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("miss: got %v, want ErrNotFound", err)
	}
	if _, _, err := svc.Lookup(context.Background(), "   "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("blank: got %v, want ErrNotFound", err)
	}
}

func TestSubmitRSVPHappyAndIdempotent(t *testing.T) {
	repo, m1, m2 := newFixture()
	// On the deadline day itself: still allowed (inclusive).
	svc := NewService(repo, fixedDeadline("2027-07-07"), at("2027-07-07 23:30"), nil)

	_, members, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "no"},
	})
	if err != nil {
		t.Fatalf("SubmitRSVP: %v", err)
	}
	if members[0].Attending != "yes" || members[1].Attending != "no" {
		t.Fatalf("answers not applied: %+v", members)
	}

	// Resubmission overwrites.
	_, members, err = svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "no"}, {GuestID: m2, Attending: "no"},
	})
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if members[0].Attending != "no" {
		t.Fatalf("resubmission did not overwrite: %+v", members)
	}
}

func TestSubmitRSVPDeadlineEnforced(t *testing.T) {
	repo, m1, m2 := newFixture()
	svc := NewService(repo, fixedDeadline("2027-07-07"), at("2027-07-08 00:10"), nil)

	_, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "yes"},
	})
	if !errors.Is(err, ErrDeadlinePassed) {
		t.Fatalf("got %v, want ErrDeadlinePassed", err)
	}
	if repo.applied != nil {
		t.Fatal("late submission must not write")
	}
}

func TestSubmitRSVPEmptyDeadlineAlwaysOpen(t *testing.T) {
	repo, m1, m2 := newFixture()
	svc := NewService(repo, fixedDeadline(""), at("2030-01-01 12:00"), nil)

	if _, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "no"},
	}); err != nil {
		t.Fatalf("empty deadline should allow submissions: %v", err)
	}
}

func TestSubmitRSVPRejectsInvalidAnswers(t *testing.T) {
	repo, m1, m2 := newFixture()
	svc := NewService(repo, fixedDeadline("2027-07-07"), at("2027-01-01 10:00"), nil)

	for _, bad := range []string{"maybe", "pending", "", "YES"} {
		_, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
			{GuestID: m1, Attending: bad}, {GuestID: m2, Attending: "no"},
		})
		if !errors.Is(err, ErrInvalidAnswer) {
			t.Fatalf("answer %q: got %v, want ErrInvalidAnswer", bad, err)
		}
	}
	if repo.applied != nil {
		t.Fatal("invalid answers must not write")
	}
}

func TestSuggestNames(t *testing.T) {
	repo, _, _ := newFixture()
	svc := NewService(repo, fixedDeadline(""), nil, nil)
	ctx := context.Background()

	// Accent/case-insensitive prefix.
	names, err := svc.SuggestNames(ctx, "  EDÚ")
	if err != nil {
		t.Fatalf("SuggestNames: %v", err)
	}
	if len(names) != 1 || names[0] != "Eduardo Silva" {
		t.Fatalf("suggestions = %v", names)
	}

	// Below 3 normalized chars: empty, no repo hit needed.
	if names, _ := svc.SuggestNames(ctx, "ed"); len(names) != 0 {
		t.Fatalf("short query must return empty, got %v", names)
	}

	// Miss returns an empty (non-nil) slice.
	names, err = svc.SuggestNames(ctx, "zzz")
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("miss: %v %v", names, err)
	}

	// LIKE metacharacters are literals, never wildcards: "%%%" must match
	// nothing instead of dumping the first 8 names.
	if names, _ := svc.SuggestNames(ctx, "%%%"); len(names) != 0 {
		t.Fatalf("wildcard input must not match, got %v", names)
	}
}

func TestEscapeLikePrefix(t *testing.T) {
	cases := map[string]string{
		"joao":   "joao",
		"100%":   `100\%`,
		"a_b":    `a\_b`,
		`a\b`:    `a\\b`,
		`%_\mix`: `\%\_\\mix`,
	}
	for in, want := range cases {
		if got := escapeLikePrefix(in); got != want {
			t.Fatalf("escapeLikePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

type fakeMailer struct {
	sent     chan [2]string
	failures int // fail this many Sends before succeeding
	attempts int
}

func (f *fakeMailer) Send(_ context.Context, subject, html string) error {
	f.attempts++
	if f.attempts <= f.failures {
		return errors.New("smtp down")
	}
	f.sent <- [2]string{subject, html}
	return nil
}

func TestSubmitRSVPNotifiesCouple(t *testing.T) {
	repo, m1, m2 := newFixture()
	mailer := &fakeMailer{sent: make(chan [2]string, 1)}
	svc := NewService(repo, fixedDeadline(""), at("2027-01-01 10:00"), mailer)

	if _, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "no"},
	}); err != nil {
		t.Fatalf("SubmitRSVP: %v", err)
	}

	select {
	case msg := <-mailer.sent:
		subject, body := msg[0], msg[1]
		if !strings.Contains(subject, "Eduardo e família") {
			t.Fatalf("subject = %q", subject)
		}
		if !strings.Contains(body, "Eduardo Silva") || !strings.Contains(body, "✅ vai") ||
			!strings.Contains(body, "Ana Clara Silva") || !strings.Contains(body, "❌ não vai") ||
			!strings.Contains(body, "1 sim, 1 não") {
			t.Fatalf("body = %q", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification never sent")
	}
}

// AD-15: only submissions with at least one "yes" notify — all-decline
// groups appear only in the admin dashboard.
func TestSubmitRSVPAllNoSendsNothing(t *testing.T) {
	repo, m1, m2 := newFixture()
	mailer := &fakeMailer{sent: make(chan [2]string, 1)}
	svc := NewService(repo, fixedDeadline(""), at("2027-01-01 10:00"), mailer)

	if _, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "no"}, {GuestID: m2, Attending: "no"},
	}); err != nil {
		t.Fatalf("SubmitRSVP: %v", err)
	}
	select {
	case msg := <-mailer.sent:
		t.Fatalf("all-no submission must not email, got %v", msg)
	case <-time.After(150 * time.Millisecond):
	}
	if mailer.attempts != 0 {
		t.Fatalf("attempts = %d, want 0", mailer.attempts)
	}
}

// AD-15: one retry on failure, then give up (logged only).
func TestNotifyRetriesOnce(t *testing.T) {
	repo, m1, m2 := newFixture()
	mailer := &fakeMailer{sent: make(chan [2]string, 1), failures: 1}
	svc := NewService(repo, fixedDeadline(""), at("2027-01-01 10:00"), mailer)
	svc.notifyRetryDelay = 10 * time.Millisecond

	if _, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "yes"},
	}); err != nil {
		t.Fatalf("SubmitRSVP: %v", err)
	}
	select {
	case <-mailer.sent:
		if mailer.attempts != 2 {
			t.Fatalf("attempts = %d, want 2 (fail + retry)", mailer.attempts)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("retry never delivered")
	}
}

func TestSubmitRSVPNilMailerIsSafe(t *testing.T) {
	repo, m1, m2 := newFixture()
	svc := NewService(repo, fixedDeadline(""), at("2027-01-01 10:00"), nil)

	if _, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "no"},
	}); err != nil {
		t.Fatalf("nil mailer must be a no-op: %v", err)
	}
}

func TestSubmitRSVPMemberCoverage(t *testing.T) {
	repo, m1, m2 := newFixture()
	svc := NewService(repo, fixedDeadline("2027-07-07"), at("2027-01-01 10:00"), nil)
	ctx := context.Background()

	cases := []struct {
		name    string
		updates []AttendanceUpdate
	}{
		{"missing member", []AttendanceUpdate{{GuestID: m1, Attending: "yes"}}},
		{"unknown member", []AttendanceUpdate{{GuestID: m1, Attending: "yes"}, {GuestID: uuid.New(), Attending: "no"}}},
		{"duplicate member", []AttendanceUpdate{{GuestID: m1, Attending: "yes"}, {GuestID: m1, Attending: "no"}}},
		{"extra beyond full set", []AttendanceUpdate{{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "no"}, {GuestID: uuid.New(), Attending: "no"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := svc.SubmitRSVP(ctx, repo.group.ID, tc.updates); !errors.Is(err, ErrInvalidMembers) {
				t.Fatalf("got %v, want ErrInvalidMembers", err)
			}
		})
	}
}
