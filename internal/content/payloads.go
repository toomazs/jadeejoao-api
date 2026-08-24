// Package content owns the editorial sections of the one-page site: a closed
// set of slugs, each holding one typed JSONB payload the couple edits in the
// admin. Payload structs are published as OpenAPI components so the admin SPA
// builds its edit forms from the exact same shapes.
package content

import (
	"encoding/json"
	"fmt"
)

// RenderOrder is the fixed order the public site renders sections in. It is
// also the closed set of valid slugs — there is no create or delete.
var RenderOrder = []string{
	"hero",
	"our_story",
	"big_day",
	"rsvp",
	"getting_there",
	"stay",
	"gifts_intro",
	"dress_code",
	"good_practices",
	"messages_intro",
}

// SlugEnum is the enum tag value shared by every slug field in this API.
const SlugEnum = "hero,our_story,big_day,rsvp,getting_there,stay,gifts_intro,dress_code,good_practices,messages_intro"

// IsValidSlug reports whether slug belongs to the closed section set.
func IsValidSlug(slug string) bool {
	for _, s := range RenderOrder {
		if s == slug {
			return true
		}
	}
	return false
}

// SectionBase carries the fields every section payload shares.
type SectionBase struct {
	Title  string   `json:"title" doc:"Section heading shown on the site."`
	Body   string   `json:"body,omitempty" doc:"Rich text as Markdown."`
	Images []string `json:"images,omitempty" doc:"Public CDN image URLs."`
}

// ProgrammeItem is one timed entry of the ceremony programme.
type ProgrammeItem struct {
	Time  string `json:"time" example:"15:00" doc:"Local time (America/Sao_Paulo), HH:MM."`
	Label string `json:"label" example:"Recepção dos convidados"`
}

// Lodging is one suggested hotel/inn entry of the stay section.
type Lodging struct {
	Name          string `json:"name" example:"Pousada Jardim"`
	Area          string `json:"area,omitempty" example:"Centro"`
	Link          string `json:"link,omitempty" format:"uri"`
	Notes         string `json:"notes,omitempty"`
	ShuttleServed bool   `json:"shuttle_served" doc:"Whether the wedding shuttle (translado) serves this lodging."`
}

// HeroMilestone is one arch of the hero's photo triptych — a moment of the
// couple's story (met, proposal, wedding day), fully admin-editable.
type HeroMilestone struct {
	Label    string `json:"label" example:"O pedido" doc:"Short caption under the arch."`
	Date     string `json:"date,omitempty" format:"date" doc:"Optional date shown above the label."`
	ImageURL string `json:"image_url,omitempty" format:"uri" doc:"Public CDN photo; the site shows brand art while empty."`
}

// HeroPayload is the opening section: names, date, and the milestone triptych.
type HeroPayload struct {
	SectionBase
	CoupleNames   string          `json:"couple_names" example:"Jade & João"`
	EventDatetime string          `json:"event_datetime" format:"date-time" example:"2027-08-07T15:00:00-03:00" doc:"Ceremony start, ISO 8601 with America/Sao_Paulo offset."`
	CityLabel     string          `json:"city_label" example:"Atibaia – SP"`
	HeroImageURL  string          `json:"hero_image_url,omitempty" format:"uri"`
	Milestones    []HeroMilestone `json:"milestones,omitempty" maxItems:"3" doc:"Up to three story arches rendered beside the names."`
}

// PersonPayload presents one of the couple in the story-film chapters: the
// chapter portrait, a short bio, and the Instagram handle the site pulls
// posts from.
type PersonPayload struct {
	Name      string `json:"name" example:"Jade" doc:"Display name for the chapter."`
	Bio       string `json:"bio,omitempty" doc:"Short bio, written by the couple."`
	PhotoURL  string `json:"photo_url,omitempty" format:"uri" doc:"Chapter portrait (public CDN URL)."`
	Instagram string `json:"instagram,omitempty" example:"xadenascimento" doc:"Instagram handle, without the @."`
}

// StoryMoment is one instax frame of the couple's photo timeline: the photo,
// the couple's handwritten-style caption, and an optional freeform date
// ("12 de maio de 2020", "2021").
type StoryMoment struct {
	Label    string `json:"label" example:"Jade e Francisca" doc:"Caption written on the frame."`
	Date     string `json:"date,omitempty" example:"12 de maio de 2020" doc:"Freeform date line under the caption."`
	ImageURL string `json:"image_url" format:"uri" doc:"Public CDN photo."`
}

// OurStoryPayload is the couple's story section: one film chapter per person,
// the instax photo album with the letters they wrote each other, and the
// invitation sentence (body).
type OurStoryPayload struct {
	SectionBase
	Bride           *PersonPayload `json:"bride,omitempty"`
	Groom           *PersonPayload `json:"groom,omitempty"`
	Moments         []StoryMoment  `json:"moments,omitempty" doc:"Chronological photo timeline rendered as instax frames."`
	LetterFromGroom string         `json:"letter_from_groom,omitempty" doc:"João's letter to Jade, shown inside the album."`
	LetterFromBride string         `json:"letter_from_bride,omitempty" doc:"Jade's letter to João, shown inside the album."`
}

// BigDayPayload describes the venue and the timed programme.
type BigDayPayload struct {
	SectionBase
	VenueNotes string          `json:"venue_notes,omitempty" doc:"Venue guidance (outdoor, grass — advise footwear)."`
	Programme  []ProgrammeItem `json:"programme,omitempty"`
}

// RSVPPayload introduces the RSVP flow and carries the deadline the API
// enforces on submissions.
type RSVPPayload struct {
	SectionBase
	Deadline string `json:"deadline" format:"date" example:"2027-07-07" doc:"Last day (inclusive, America/Sao_Paulo) to submit or change RSVPs. Enforced server-side."`
}

// GettingTherePayload holds address, map, and parking guidance.
type GettingTherePayload struct {
	SectionBase
	Address      string `json:"address" example:"Rua Piraju, 306 – Jardim Paulista, Atibaia – SP, 12947-321"`
	MapEmbedURL  string `json:"map_embed_url,omitempty" format:"uri"`
	ParkingNotes string `json:"parking_notes,omitempty"`
}

// StayPayload lists lodging suggestions and Airbnb neighborhoods.
type StayPayload struct {
	SectionBase
	Lodgings    []Lodging `json:"lodgings,omitempty"`
	AirbnbAreas []string  `json:"airbnb_areas,omitempty" doc:"Neighborhoods worth searching on Airbnb."`
}

// GiftsIntroPayload introduces the gift list section.
type GiftsIntroPayload struct {
	SectionBase
}

// DressCodePayload carries the dress code formula and guidance.
type DressCodePayload struct {
	SectionBase
	Attire string `json:"attire,omitempty" example:"Sofisticado, confortável, vestido longo, esporte fino." doc:"Short dress-code formula."`
}

// GoodPracticesPayload lists the couple's playful house rules.
type GoodPracticesPayload struct {
	SectionBase
	Rules []string `json:"rules,omitempty"`
}

// MessagesIntroPayload introduces the guestbook section.
type MessagesIntroPayload struct {
	SectionBase
}

// Section is one editorial section: its slug, visibility, and exactly one
// payload field set — the one matching the slug. Modeling payloads as one
// optional field per slug keeps every shape a named OpenAPI component and
// makes client codegen trivial (section.hero is defined when slug == "hero").
// Slug is omitempty because the same struct is the admin update body, where
// the path — not the body — decides the slug.
type Section struct {
	Slug    string `json:"slug,omitempty" enum:"hero,our_story,big_day,rsvp,getting_there,stay,gifts_intro,dress_code,good_practices,messages_intro" doc:"Always present in responses; ignored in the section-update request body (the path decides)."`
	Enabled bool   `json:"enabled" doc:"Disabled sections are hidden from the public site (server-side)."`

	Hero          *HeroPayload          `json:"hero,omitempty"`
	OurStory      *OurStoryPayload      `json:"our_story,omitempty"`
	BigDay        *BigDayPayload        `json:"big_day,omitempty"`
	RSVP          *RSVPPayload          `json:"rsvp,omitempty"`
	GettingThere  *GettingTherePayload  `json:"getting_there,omitempty"`
	Stay          *StayPayload          `json:"stay,omitempty"`
	GiftsIntro    *GiftsIntroPayload    `json:"gifts_intro,omitempty"`
	DressCode     *DressCodePayload     `json:"dress_code,omitempty"`
	GoodPractices *GoodPracticesPayload `json:"good_practices,omitempty"`
	MessagesIntro *MessagesIntroPayload `json:"messages_intro,omitempty"`
}

// decodePayload fills the payload field matching slug from raw JSONB.
func (s *Section) decodePayload(slug string, raw []byte) error {
	target := s.payloadPtr(slug)
	if target == nil {
		return fmt.Errorf("unknown section slug %q", slug)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode %s payload: %w", slug, err)
	}
	return nil
}

// payloadPtr allocates and returns the payload field for slug, or nil for an
// unknown slug.
func (s *Section) payloadPtr(slug string) any {
	switch slug {
	case "hero":
		s.Hero = &HeroPayload{}
		return s.Hero
	case "our_story":
		s.OurStory = &OurStoryPayload{}
		return s.OurStory
	case "big_day":
		s.BigDay = &BigDayPayload{}
		return s.BigDay
	case "rsvp":
		s.RSVP = &RSVPPayload{}
		return s.RSVP
	case "getting_there":
		s.GettingThere = &GettingTherePayload{}
		return s.GettingThere
	case "stay":
		s.Stay = &StayPayload{}
		return s.Stay
	case "gifts_intro":
		s.GiftsIntro = &GiftsIntroPayload{}
		return s.GiftsIntro
	case "dress_code":
		s.DressCode = &DressCodePayload{}
		return s.DressCode
	case "good_practices":
		s.GoodPractices = &GoodPracticesPayload{}
		return s.GoodPractices
	case "messages_intro":
		s.MessagesIntro = &MessagesIntroPayload{}
		return s.MessagesIntro
	}
	return nil
}

// payloadValue returns the payload field currently set for slug, without
// allocating. Used to validate and marshal admin updates.
func (s *Section) payloadValue(slug string) any {
	switch slug {
	case "hero":
		if s.Hero != nil {
			return s.Hero
		}
	case "our_story":
		if s.OurStory != nil {
			return s.OurStory
		}
	case "big_day":
		if s.BigDay != nil {
			return s.BigDay
		}
	case "rsvp":
		if s.RSVP != nil {
			return s.RSVP
		}
	case "getting_there":
		if s.GettingThere != nil {
			return s.GettingThere
		}
	case "stay":
		if s.Stay != nil {
			return s.Stay
		}
	case "gifts_intro":
		if s.GiftsIntro != nil {
			return s.GiftsIntro
		}
	case "dress_code":
		if s.DressCode != nil {
			return s.DressCode
		}
	case "good_practices":
		if s.GoodPractices != nil {
			return s.GoodPractices
		}
	case "messages_intro":
		if s.MessagesIntro != nil {
			return s.MessagesIntro
		}
	}
	return nil
}
