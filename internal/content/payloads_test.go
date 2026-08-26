package content

import (
	"reflect"
	"strings"
	"testing"
)

// TestSlugSetsStayInSync guards the closed slug set against drift: the render
// order, the decode/update switches, the SlugEnum constant, and the enum tag
// on Section.Slug must all agree. Migration 00002 seeds the same set.
func TestSlugSetsStayInSync(t *testing.T) {
	if len(RenderOrder) != 9 {
		t.Fatalf("closed set must have 9 slugs, got %d", len(RenderOrder))
	}

	joined := strings.Join(RenderOrder, ",")
	if SlugEnum != joined {
		t.Fatalf("SlugEnum drifted:\n const %s\n order %s", SlugEnum, joined)
	}

	field, ok := reflect.TypeOf(Section{}).FieldByName("Slug")
	if !ok {
		t.Fatal("Section.Slug missing")
	}
	if tag := field.Tag.Get("enum"); tag != joined {
		t.Fatalf("Section.Slug enum tag drifted:\n tag   %s\n order %s", tag, joined)
	}

	for _, slug := range RenderOrder {
		var s Section
		if ptr := s.payloadPtr(slug); ptr == nil {
			t.Fatalf("payloadPtr has no case for %q", slug)
		}
		if val := s.payloadValue(slug); val == nil {
			t.Fatalf("payloadValue has no case for %q (after allocation)", slug)
		}
	}

	// Unknown slugs stay unknown.
	var s Section
	if s.payloadPtr("not_a_slug") != nil || IsValidSlug("not_a_slug") {
		t.Fatal("unknown slug accepted")
	}
}

func TestUpdateSectionValidatesRSVPDeadline(t *testing.T) {
	svc := NewService(&fakeRepo{})

	for _, bad := range []string{"", "  ", "2027-13-40", "07/07/2027", "2027-07-07T00:00:00Z"} {
		upd := Section{Enabled: true, RSVP: &RSVPPayload{SectionBase: SectionBase{Title: "Confirme"}, Deadline: bad}}
		if _, err := svc.UpdateSection(t.Context(), "rsvp", upd); err != ErrInvalidDeadline {
			t.Fatalf("deadline %q: got %v, want ErrInvalidDeadline", bad, err)
		}
	}

	// Surrounding whitespace is trimmed, then accepted.
	upd := Section{Enabled: true, RSVP: &RSVPPayload{SectionBase: SectionBase{Title: "Confirme"}, Deadline: " 2027-07-07 "}}
	got, err := svc.UpdateSection(t.Context(), "rsvp", upd)
	if err != nil {
		t.Fatalf("trimmed deadline rejected: %v", err)
	}
	if got.RSVP.Deadline != "2027-07-07" {
		t.Fatalf("deadline not trimmed: %q", got.RSVP.Deadline)
	}
}
