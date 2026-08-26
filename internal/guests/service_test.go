package guests

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

type fakeRepo struct {
	group   Group
	members []Member
	applied []AttendanceUpdate
	// companions counts who was gathered in, and added remembers which rows
	// those were — together they stand in for the added_by_guest column.
	companions int
	added      map[uuid.UUID]bool
	// elsewhere models the rest of the couple's list: everyone NOT on this
	// invitation, with how many people share their own invitation. Size > 1
	// means they head a family and cannot be moved.
	elsewhere map[uuid.UUID]outsider
}

type outsider struct {
	name      string
	groupSize int
}

func (f *fakeRepo) FindGuestByNormalizedName(_ context.Context, normalized string) (Member, uuid.UUID, error) {
	for _, m := range f.members {
		if Normalize(m.FullName) == normalized {
			return m, f.group.ID, nil
		}
	}
	return Member{}, uuid.Nil, ErrNotFound
}

func (f *fakeRepo) SuggestAvailableCompanions(_ context.Context, groupID uuid.UUID, prefix string) ([]CompanionOption, error) {
	if groupID != f.group.ID {
		return nil, nil
	}
	var out []CompanionOption
	for id, o := range f.elsewhere {
		if o.groupSize == 1 && strings.HasPrefix(Normalize(o.name), prefix) {
			out = append(out, CompanionOption{ID: id, FullName: o.name})
		}
	}
	return out, nil
}

func (f *fakeRepo) AddCompanion(_ context.Context, groupID, guestID uuid.UUID) error {
	if groupID != f.group.ID {
		return ErrNotFound
	}
	if f.companions >= MaxCompanionsPerGroup {
		return ErrCompanionLimit
	}
	for _, m := range f.members {
		if m.ID == guestID {
			return ErrAlreadyOnInvitation
		}
	}
	o, ok := f.elsewhere[guestID]
	if !ok {
		return ErrNotFound
	}
	if o.groupSize > 1 {
		return ErrGuestUnavailable
	}
	delete(f.elsewhere, guestID)
	f.members = append(f.members, Member{
		ID: guestID, FullName: o.name, Attending: "pending", AddedByGuest: true,
	})
	f.companions++
	if f.added == nil {
		f.added = map[uuid.UUID]bool{}
	}
	f.added[guestID] = true
	return nil
}

func (f *fakeRepo) RemoveCompanion(_ context.Context, groupID, guestID uuid.UUID) error {
	if groupID != f.group.ID {
		return ErrNotRemovable
	}
	for i, m := range f.members {
		if m.ID != guestID {
			continue
		}
		if !f.added[guestID] {
			return ErrNotRemovable
		}
		f.members = append(f.members[:i], f.members[i+1:]...)
		delete(f.added, guestID)
		f.companions--
		// Back to their own invitation, alone — pickable again.
		f.elsewhere[guestID] = outsider{name: m.FullName, groupSize: 1}
		return nil
	}
	return ErrNotRemovable
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

// SuggestNames speaks real LIKE semantics (escapes, %, _) so the escaping in
// the service is load-bearing in tests: if escapeLikePrefix were the identity
// function, a "%%%" query would match every name here too.
func (f *fakeRepo) SuggestNames(_ context.Context, normalizedPrefix string) ([]string, error) {
	var out []string
	for _, m := range f.members {
		if likePrefixMatch(normalizedPrefix, Normalize(m.FullName)) {
			out = append(out, m.FullName)
		}
	}
	return out, nil
}

// likePrefixMatch interprets pattern as a SQL LIKE prefix (escape '\').
func likePrefixMatch(pattern, s string) bool {
	var sb strings.Builder
	sb.WriteString("^")
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			if i+1 < len(runes) {
				i++
				sb.WriteString(regexp.QuoteMeta(string(runes[i])))
			}
		case '%':
			sb.WriteString(".*")
		case '_':
			sb.WriteString(".")
		default:
			sb.WriteString(regexp.QuoteMeta(string(runes[i])))
		}
	}
	return regexp.MustCompile(sb.String()).MatchString(s)
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
		elsewhere: elsewhereFixture(),
	}
	return repo, g1, g2
}

// The couple's list beyond this invitation. soloGuest is alone in their own
// invitation and may be gathered in; familyHead heads an invitation of four
// and must never be movable — taking them would orphan the other three.
var (
	soloGuest  = uuid.New()
	familyHead = uuid.New()
)

func elsewhereFixture() map[uuid.UUID]outsider {
	m := map[uuid.UUID]outsider{
		soloGuest:  {name: "Marina Prado", groupSize: 1},
		familyHead: {name: "Ronaldo Nascimento", groupSize: 4},
	}
	// Spares, so a test can push past the per-invitation ceiling.
	for i := 0; i < MaxCompanionsPerGroup+2; i++ {
		m[uuid.New()] = outsider{name: fmt.Sprintf("Sozinho %02d", i), groupSize: 1}
	}
	return m
}

// availableIDs lists everyone the fake would let this invitation gather in.
func (f *fakeRepo) availableIDs() []uuid.UUID {
	var out []uuid.UUID
	for id, o := range f.elsewhere {
		if o.groupSize == 1 {
			out = append(out, id)
		}
	}
	return out
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
	failures int   // fail this many Sends before succeeding
	failErr  error // when set, every Send fails with this error
	attempts int
}

func (f *fakeMailer) Send(_ context.Context, subject, html string) error {
	f.attempts++
	if f.failErr != nil {
		return f.failErr
	}
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
		if !strings.Contains(body, "Eduardo Silva") || !strings.Contains(body, "vai ✓") ||
			!strings.Contains(body, "Ana Clara Silva") || !strings.Contains(body, "não vai") ||
			!strings.Contains(body, "1 sim, 1 não") {
			t.Fatalf("body = %q", body)
		}
		// Dressed in the wedding's own colours, with their wordmark on top.
		if !strings.Contains(body, "#50590d") || !strings.Contains(body, "brand/logo-vertical.png") {
			t.Error("notification is not wearing the couple's identity")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification never sent")
	}
	if mailer.attempts != 1 {
		t.Fatalf("attempts = %d, want exactly 1", mailer.attempts)
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

// Permanent (4xx) rejections are not retried — an identical request cannot
// succeed and the retry would only delay the log line.
func TestNotifyNoRetryOnPermanent(t *testing.T) {
	repo, m1, m2 := newFixture()
	mailer := &fakeMailer{sent: make(chan [2]string, 1), failErr: fmt.Errorf("resend: status 422: %w", platform.ErrPermanentSend)}
	svc := NewService(repo, fixedDeadline(""), at("2027-01-01 10:00"), mailer)
	svc.notifyRetryDelay = 10 * time.Millisecond

	if _, _, err := svc.SubmitRSVP(context.Background(), repo.group.ID, []AttendanceUpdate{
		{GuestID: m1, Attending: "yes"}, {GuestID: m2, Attending: "yes"},
	}); err != nil {
		t.Fatalf("SubmitRSVP: %v", err)
	}
	drainCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	svc.DrainNotifications(drainCtx)
	if mailer.attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no retry on permanent)", mailer.attempts)
	}
}

func TestSuggestNamesCapsAtEight(t *testing.T) {
	repo := &fakeRepo{group: Group{ID: uuid.New(), Label: "Grupo"}}
	for i := 0; i < 9; i++ {
		repo.members = append(repo.members, Member{ID: uuid.New(), FullName: fmt.Sprintf("Zeta Convidado %d", i), Attending: "pending"})
	}
	names, err := NewService(repo, fixedDeadline(""), nil, nil).SuggestNames(context.Background(), "zeta")
	if err != nil {
		t.Fatalf("SuggestNames: %v", err)
	}
	if len(names) != 8 {
		t.Fatalf("got %d names, want the cap of 8", len(names))
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

func companionSvc(repo *fakeRepo) *Service {
	return NewService(repo, fixedDeadline("2027-07-07"), at("2026-08-24 12:00"), nil)
}

func TestCompanionSearchOnlyOffersPeopleAlreadyInvited(t *testing.T) {
	repo, _, _ := newFixture()
	svc := companionSvc(repo)

	got, err := svc.SuggestCompanions(context.Background(), repo.group.ID, "mari")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 || got[0].ID != soloGuest {
		t.Fatalf("expected only Marina Prado, got %+v", got)
	}

	// Someone heading an invitation of four must never be offered: gathering
	// them would leave the other three without a primary.
	got, err = svc.SuggestCompanions(context.Background(), repo.group.ID, "ronaldo")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("head of a shared invitation was offered: %+v", got)
	}

	// A name nobody on the list has returns nothing — which is the whole point:
	// the guest cannot conjure someone who was never invited.
	got, _ = svc.SuggestCompanions(context.Background(), repo.group.ID, "fulano da silva sauro")
	if len(got) != 0 {
		t.Fatalf("an uninvited name was offered: %+v", got)
	}
}

func TestAddCompanionMovesAnInvitedPersonIn(t *testing.T) {
	repo, _, _ := newFixture()
	svc := companionSvc(repo)

	before := len(repo.members)
	_, members, err := svc.AddCompanion(context.Background(), repo.group.ID, soloGuest)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(members) != before+1 {
		t.Fatalf("invitation has %d members, want %d", len(members), before+1)
	}
	var moved *Member
	for i := range members {
		if members[i].ID == soloGuest {
			moved = &members[i]
		}
	}
	if moved == nil {
		t.Fatal("the gathered guest is not on the invitation")
	}
	if moved.FullName != "Marina Prado" {
		t.Errorf("name came from the guest instead of the couple's list: %q", moved.FullName)
	}
	if !moved.AddedByGuest {
		t.Error("gathered guest should be marked as guest-added, so only they can undo it")
	}
	// No answer is invented on their behalf — the primary answers for them in
	// the normal list, with the same buttons as everyone else.
	if moved.Attending != "pending" {
		t.Errorf("attending = %q, want pending", moved.Attending)
	}
	// They are gone from the pool, so a second invitation cannot claim them.
	if _, still := repo.elsewhere[soloGuest]; still {
		t.Error("guest still offered to other invitations after being gathered")
	}
}

func TestAddCompanionRefusesWhoeverIsNotAvailable(t *testing.T) {
	repo, m1, _ := newFixture()
	svc := companionSvc(repo)
	ctx := context.Background()

	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, familyHead); !errors.Is(err, ErrGuestUnavailable) {
		t.Errorf("head of a shared invitation: got %v, want ErrGuestUnavailable", err)
	}
	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, m1); !errors.Is(err, ErrAlreadyOnInvitation) {
		t.Errorf("someone already here: got %v, want ErrAlreadyOnInvitation", err)
	}
	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("a guest that does not exist: got %v, want ErrNotFound", err)
	}
}

func TestAddCompanionStopsAtTheCeiling(t *testing.T) {
	repo, _, _ := newFixture()
	svc := companionSvc(repo)
	ctx := context.Background()

	pool := repo.availableIDs()
	for i := 0; i < MaxCompanionsPerGroup; i++ {
		if _, _, err := svc.AddCompanion(ctx, repo.group.ID, pool[i]); err != nil {
			t.Fatalf("companion %d rejected early: %v", i, err)
		}
	}
	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, pool[MaxCompanionsPerGroup]); !errors.Is(err, ErrCompanionLimit) {
		t.Fatalf("got %v, want ErrCompanionLimit after %d", err, MaxCompanionsPerGroup)
	}
}

func TestRemoveCompanionSendsThemBackAndOnlyReachesTheGuestsOwn(t *testing.T) {
	repo, m1, _ := newFixture()
	svc := companionSvc(repo)
	ctx := context.Background()

	// Someone the couple placed here is off limits, whatever the caller sends.
	if _, _, err := svc.RemoveCompanion(ctx, repo.group.ID, m1); !errors.Is(err, ErrNotRemovable) {
		t.Fatalf("removing a couple-placed guest: got %v, want ErrNotRemovable", err)
	}

	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, soloGuest); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, members, err := svc.RemoveCompanion(ctx, repo.group.ID, soloGuest)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	for _, m := range members {
		if m.ID == soloGuest {
			t.Fatal("still on the invitation after being sent back")
		}
	}
	// Sent back, not deleted: they are still invited and pickable again.
	if _, back := repo.elsewhere[soloGuest]; !back {
		t.Fatal("guest was dropped from the couple's list instead of returned to their own invitation")
	}
	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, soloGuest); err != nil {
		t.Fatalf("slot not freed after sending someone back: %v", err)
	}
}

func TestCompanionChangesRefusedAfterTheDeadline(t *testing.T) {
	repo, _, _ := newFixture()
	svc := NewService(repo, fixedDeadline("2026-01-01"), at("2026-08-24 12:00"), nil)
	ctx := context.Background()

	if _, _, err := svc.AddCompanion(ctx, repo.group.ID, soloGuest); !errors.Is(err, ErrDeadlinePassed) {
		t.Errorf("add: got %v, want ErrDeadlinePassed", err)
	}
	if _, _, err := svc.RemoveCompanion(ctx, repo.group.ID, soloGuest); !errors.Is(err, ErrDeadlinePassed) {
		t.Errorf("remove: got %v, want ErrDeadlinePassed", err)
	}
}

// --- the panel's list-repair surface ---

func (f *fakeRepo) CreateGuest(_ context.Context, edit GuestEdit, into *uuid.UUID) (uuid.UUID, error) {
	for _, m := range f.members {
		if Normalize(m.FullName) == Normalize(edit.FullName) {
			return uuid.Nil, ErrNameTaken
		}
	}
	id := uuid.New()
	f.members = append(f.members, Member{
		ID:        id,
		FullName:  edit.FullName,
		Category:  edit.Category,
		Notes:     edit.Notes,
		IsPrimary: into == nil,
		Attending: "pending",
	})
	return id, nil
}

func (f *fakeRepo) UpdateGuestDetails(_ context.Context, guestID uuid.UUID, edit GuestEdit) error {
	for _, m := range f.members {
		if m.ID != guestID && Normalize(m.FullName) == Normalize(edit.FullName) {
			return ErrNameTaken
		}
	}
	for i, m := range f.members {
		if m.ID == guestID {
			f.members[i].FullName = edit.FullName
			f.members[i].Category = edit.Category
			f.members[i].Notes = edit.Notes
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) DeleteGuest(_ context.Context, guestID uuid.UUID) error {
	for i, m := range f.members {
		if m.ID == guestID {
			f.members = append(f.members[:i], f.members[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) RenameGroup(_ context.Context, groupID uuid.UUID, label string) error {
	if groupID != f.group.ID {
		return ErrNotFound
	}
	f.group.Label = label
	return nil
}

func (f *fakeRepo) SetGroupPrimary(_ context.Context, groupID, guestID uuid.UUID) error {
	if groupID != f.group.ID {
		return ErrNotFound
	}
	found := false
	for i := range f.members {
		f.members[i].IsPrimary = f.members[i].ID == guestID
		found = found || f.members[i].ID == guestID
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

func (f *fakeRepo) MoveGuestAsAdmin(_ context.Context, groupID, guestID uuid.UUID) error {
	if groupID != f.group.ID {
		return ErrNotFound
	}
	for _, m := range f.members {
		if m.ID == guestID {
			return ErrAlreadyOnInvitation
		}
	}
	o, ok := f.elsewhere[guestID]
	if !ok {
		return ErrNotFound
	}
	delete(f.elsewhere, guestID)
	f.members = append(f.members, Member{ID: guestID, FullName: o.name, Attending: "pending"})
	return nil
}

func TestEditGuestKeepsNamesUniqueAndVocabularyClosed(t *testing.T) {
	repo, m1, m2 := newFixture()
	svc := companionSvc(repo)
	ctx := context.Background()

	// Names are globally unique because the guest lookup matches on them:
	// two "Ana Clara Silva" and one of them cannot open her own invitation.
	if err := svc.EditGuest(ctx, m1, GuestEdit{FullName: "Ana Clara Silva"}); !errors.Is(err, ErrNameTaken) {
		t.Errorf("duplicate name: got %v, want ErrNameTaken", err)
	}
	if err := svc.EditGuest(ctx, m1, GuestEdit{FullName: "   "}); !errors.Is(err, ErrInvalidName) {
		t.Errorf("blank name: got %v, want ErrInvalidName", err)
	}
	marciano := "marciano"
	if err := svc.EditGuest(ctx, m1, GuestEdit{FullName: "Eduardo Silva", Category: &marciano}); !errors.Is(err, ErrInvalidField) {
		t.Errorf("unknown category: got %v, want ErrInvalidField", err)
	}

	// The happy path also collapses stray whitespace, like the import does.
	if err := svc.EditGuest(ctx, m2, GuestEdit{FullName: "  Ana   Clara  Silva ", Notes: "Prima"}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if repo.members[1].FullName != "Ana Clara Silva" || repo.members[1].Notes != "Prima" {
		t.Errorf("edit not applied: %+v", repo.members[1])
	}
}

func TestSetPrimaryLeavesExactlyOne(t *testing.T) {
	repo, _, m2 := newFixture()
	svc := companionSvc(repo)

	if err := svc.SetPrimary(context.Background(), repo.group.ID, m2); err != nil {
		t.Fatalf("set primary: %v", err)
	}
	primaries := 0
	for _, m := range repo.members {
		if m.IsPrimary {
			primaries++
		}
	}
	if primaries != 1 || !repo.members[1].IsPrimary {
		t.Fatalf("want exactly one primary and it to be the new one, got %+v", repo.members)
	}
}

// TestCreateGuestAloneIsTheirOwnPrimary — somebody typed in by hand has to end
// up in the shape every imported guest starts in, or they arrive on the list
// unable to answer for themselves.
func TestCreateGuestAloneIsTheirOwnPrimary(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil, nil, nil)
	id, err := svc.CreateGuest(context.Background(), GuestEdit{FullName: "  Tia   Selma  "}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	repo := svc.repo.(*fakeRepo)
	person := repo.members[len(repo.members)-1]
	if person.ID != id {
		t.Fatalf("returned id %s, stored %s", id, person.ID)
	}
	// The name is tidied on the way in, the same as an edit does it.
	if person.FullName != "Tia Selma" {
		t.Fatalf("name not tidied: %q", person.FullName)
	}
	if !person.IsPrimary {
		t.Fatal("alone, they must be the primary of their own invitation")
	}
}

// TestCreateGuestRefusesWhatEditRefuses — the two doors into the same table
// must not accept different people.
func TestCreateGuestRefusesWhatEditRefuses(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil, nil, nil)
	ctx := context.Background()
	bad := "nope"
	cases := map[string]GuestEdit{
		"sem nome":        {FullName: "   "},
		"faixa inválida":  {FullName: "Alguém", Category: &bad},
		"gênero inválido": {FullName: "Alguém", Gender: &bad},
		"lado inválido":   {FullName: "Alguém", Side: &bad},
	}
	for name, edit := range cases {
		if _, err := svc.CreateGuest(ctx, edit, nil); err == nil {
			t.Errorf("%s: should have been refused", name)
		}
	}
}

// TestCreateGuestRefusesADuplicateName — the list is looked up by name (AD-5),
// so two people with one name is a lookup nobody can resolve.
func TestCreateGuestRefusesADuplicateName(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil, nil, nil)
	ctx := context.Background()
	if _, err := svc.CreateGuest(ctx, GuestEdit{FullName: "Tia Selma"}, nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := svc.CreateGuest(ctx, GuestEdit{FullName: "tia  selma"}, nil); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("second: got %v, want ErrNameTaken", err)
	}
}
