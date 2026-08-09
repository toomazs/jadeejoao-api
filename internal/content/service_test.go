package content

import (
	"context"
	"testing"
)

type fakeRepo struct {
	rows    []Row
	updated *Row
}

func (f *fakeRepo) ListSections(_ context.Context) ([]Row, error) { return f.rows, nil }

func (f *fakeRepo) UpdateSection(_ context.Context, slug string, payload []byte, enabled bool) (Row, error) {
	row := Row{Slug: slug, Payload: payload, Enabled: enabled}
	f.updated = &row
	return row, nil
}

func TestPublicContentOrdersAndFilters(t *testing.T) {
	repo := &fakeRepo{rows: []Row{
		// Deliberately out of render order, with one disabled.
		{Slug: "rsvp", Payload: []byte(`{"title":"Confirme","deadline":"2027-07-07"}`), Enabled: true},
		{Slug: "hero", Payload: []byte(`{"title":"Jade & João","couple_names":"Jade & João","event_datetime":"2027-08-07T15:00:00-03:00","city_label":"Atibaia – SP"}`), Enabled: true},
		{Slug: "our_story", Payload: []byte(`{"title":"Nossa História"}`), Enabled: false},
	}}
	svc := NewService(repo)

	sections, err := svc.PublicContent(context.Background())
	if err != nil {
		t.Fatalf("PublicContent: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2 (disabled must be filtered)", len(sections))
	}
	if sections[0].Slug != "hero" || sections[1].Slug != "rsvp" {
		t.Fatalf("wrong order: %s, %s (want hero, rsvp)", sections[0].Slug, sections[1].Slug)
	}
	if sections[0].Hero == nil || sections[0].Hero.CoupleNames != "Jade & João" {
		t.Fatalf("hero payload not decoded: %+v", sections[0])
	}
	if sections[1].RSVP == nil || sections[1].RSVP.Deadline != "2027-07-07" {
		t.Fatalf("rsvp payload not decoded: %+v", sections[1])
	}
}

func TestUpdateSectionValidation(t *testing.T) {
	svc := NewService(&fakeRepo{})

	if _, err := svc.UpdateSection(context.Background(), "not_a_slug", Section{}); err != ErrUnknownSlug {
		t.Fatalf("unknown slug: got %v, want ErrUnknownSlug", err)
	}

	// Payload field must match the slug being updated.
	upd := Section{Hero: &HeroPayload{}}
	if _, err := svc.UpdateSection(context.Background(), "rsvp", upd); err == nil {
		t.Fatal("mismatched payload accepted")
	}

	ok := Section{Enabled: true, RSVP: &RSVPPayload{SectionBase: SectionBase{Title: "Confirme"}, Deadline: "2027-07-07"}}
	got, err := svc.UpdateSection(context.Background(), "rsvp", ok)
	if err != nil {
		t.Fatalf("valid update rejected: %v", err)
	}
	if got.RSVP == nil || got.RSVP.Deadline != "2027-07-07" || !got.Enabled {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestRSVPDeadline(t *testing.T) {
	repo := &fakeRepo{rows: []Row{
		{Slug: "rsvp", Payload: []byte(`{"title":"Confirme","deadline":"2027-07-07"}`), Enabled: true},
	}}
	deadline, err := NewService(repo).RSVPDeadline(context.Background())
	if err != nil {
		t.Fatalf("RSVPDeadline: %v", err)
	}
	if deadline != "2027-07-07" {
		t.Fatalf("deadline = %q, want 2027-07-07", deadline)
	}
}
