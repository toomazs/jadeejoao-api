package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrUnknownSlug marks updates targeting a slug outside the closed set.
var ErrUnknownSlug = errors.New("unknown section slug")

// ErrPayloadMismatch marks updates whose payload field does not match the slug.
var ErrPayloadMismatch = errors.New("payload does not match section slug")

// Row is the storage shape of a section, as the repository returns it.
type Row struct {
	Slug      string
	Payload   []byte
	Enabled   bool
	UpdatedAt time.Time
}

// Repo is the narrow persistence surface the service needs. The sqlc-backed
// implementation lives in repo.go; tests inject fakes.
type Repo interface {
	ListSections(ctx context.Context) ([]Row, error)
	UpdateSection(ctx context.Context, slug string, payload []byte, enabled bool) (Row, error)
}

// Service exposes editorial content to the public site and the admin panel.
type Service struct {
	repo Repo
}

// NewService wires the content service over its repository.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// PublicContent returns the enabled sections in the fixed render order, fully
// decoded. This backs the single GET /api/v1/content call the site makes.
func (s *Service) PublicContent(ctx context.Context) ([]Section, error) {
	all, err := s.listDecoded(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Section, 0, len(all))
	for _, sec := range all {
		if sec.Enabled {
			out = append(out, sec)
		}
	}
	return out, nil
}

// AdminContent returns every section (including disabled) in render order.
func (s *Service) AdminContent(ctx context.Context) ([]Section, error) {
	return s.listDecoded(ctx)
}

// UpdateSection validates and persists an admin edit for one slug. The update
// must carry exactly the payload field matching the slug (update-only: the
// slug set is closed, so there is no create or delete).
func (s *Service) UpdateSection(ctx context.Context, slug string, update Section) (Section, error) {
	if !IsValidSlug(slug) {
		return Section{}, ErrUnknownSlug
	}
	payload := update.payloadValue(slug)
	if payload == nil {
		return Section{}, fmt.Errorf("%w: expected field %q", ErrPayloadMismatch, slug)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Section{}, fmt.Errorf("marshal %s payload: %w", slug, err)
	}
	row, err := s.repo.UpdateSection(ctx, slug, raw, update.Enabled)
	if err != nil {
		return Section{}, err
	}
	return decodeRow(row)
}

// RSVPDeadline exposes the rsvp section deadline for the guests service to
// enforce (cross-module access is service→service, never repo→repo).
func (s *Service) RSVPDeadline(ctx context.Context) (string, error) {
	all, err := s.listDecoded(ctx)
	if err != nil {
		return "", err
	}
	for _, sec := range all {
		if sec.Slug == "rsvp" && sec.RSVP != nil {
			return sec.RSVP.Deadline, nil
		}
	}
	return "", errors.New("rsvp section missing")
}

func (s *Service) listDecoded(ctx context.Context) ([]Section, error) {
	rows, err := s.repo.ListSections(ctx)
	if err != nil {
		return nil, err
	}
	bySlug := make(map[string]Row, len(rows))
	for _, r := range rows {
		bySlug[r.Slug] = r
	}
	out := make([]Section, 0, len(RenderOrder))
	for _, slug := range RenderOrder {
		row, ok := bySlug[slug]
		if !ok {
			continue // slug not seeded yet — never invent content
		}
		sec, err := decodeRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, nil
}

func decodeRow(row Row) (Section, error) {
	sec := Section{Slug: row.Slug, Enabled: row.Enabled}
	if err := sec.decodePayload(row.Slug, row.Payload); err != nil {
		return Section{}, err
	}
	return sec, nil
}
