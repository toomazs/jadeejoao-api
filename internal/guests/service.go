package guests

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound: no guest matches the looked-up name (or group id unknown).
	ErrNotFound = errors.New("guest not found")
	// ErrDeadlinePassed: RSVP submissions arrived after the rsvp deadline.
	ErrDeadlinePassed = errors.New("rsvp deadline passed")
	// ErrInvalidMembers: the submission does not cover exactly the group's members.
	ErrInvalidMembers = errors.New("submission must cover exactly the group members")
	// ErrInvalidAnswer: an attendance value other than yes/no. The transport
	// enum already blocks these; the service re-validates so the domain rule
	// never depends on the schema layer alone.
	ErrInvalidAnswer = errors.New("attendance answer must be yes or no")
)

// Group is a guest group (one invitation).
type Group struct {
	ID    uuid.UUID
	Label string
}

// Member is one guest inside a group, with their current RSVP state.
type Member struct {
	ID        uuid.UUID
	FullName  string
	IsPrimary bool
	Category  *string
	Attending string
}

// AttendanceUpdate sets one member's RSVP answer.
type AttendanceUpdate struct {
	GuestID   uuid.UUID
	Attending string // "yes" | "no"
}

// Repo is the persistence surface for guests. UpdateAttendances must run in a
// single transaction and fail if any update does not match exactly one guest
// of the group.
type Repo interface {
	FindGuestByNormalizedName(ctx context.Context, normalized string) (Member, uuid.UUID, error)
	GetGroup(ctx context.Context, id uuid.UUID) (Group, error)
	ListMembers(ctx context.Context, groupID uuid.UUID) ([]Member, error)
	UpdateAttendances(ctx context.Context, groupID uuid.UUID, updates []AttendanceUpdate) error
	ListAllGroups(ctx context.Context) ([]Group, error)
	ListAllGuests(ctx context.Context) (map[uuid.UUID][]Member, error)
}

// GroupWithMembers is one dashboard row.
type GroupWithMembers struct {
	Group
	Members []Member
}

// Headcounts aggregates RSVP state for planning.
type Headcounts struct {
	Total   int
	Yes     int
	No      int
	Pending int
	// YesByCategory counts confirmed guests per category; guests without a
	// category land under "uncategorized".
	YesByCategory map[string]int
}

// DeadlineSource yields the RSVP deadline (a date string, possibly empty).
// Satisfied by content.Service — cross-module access is service→service.
type DeadlineSource interface {
	RSVPDeadline(ctx context.Context) (string, error)
}

// Service implements lookup and RSVP.
type Service struct {
	repo     Repo
	deadline DeadlineSource
	now      func() time.Time
}

// NewService wires the guests service. now is injectable for tests; pass nil
// for time.Now.
func NewService(repo Repo, deadline DeadlineSource, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, deadline: deadline, now: now}
}

// Lookup resolves a typed full name to its guest group. Exact normalized
// match only — no fuzzy matching, no suggestions, by design (AD-5).
func (s *Service) Lookup(ctx context.Context, fullName string) (Group, []Member, error) {
	normalized := Normalize(fullName)
	if normalized == "" {
		return Group{}, nil, ErrNotFound
	}
	_, groupID, err := s.repo.FindGuestByNormalizedName(ctx, normalized)
	if err != nil {
		return Group{}, nil, err
	}
	return s.groupWithMembers(ctx, groupID)
}

// SubmitRSVP records one yes/no per member of the group. Idempotent:
// resubmission overwrites previous answers. Rejected after the deadline.
func (s *Service) SubmitRSVP(ctx context.Context, groupID uuid.UUID, updates []AttendanceUpdate) (Group, []Member, error) {
	deadline, err := s.deadline.RSVPDeadline(ctx)
	if err != nil {
		return Group{}, nil, fmt.Errorf("load rsvp deadline: %w", err)
	}
	passed, err := deadlinePassed(deadline, s.now())
	if err != nil {
		return Group{}, nil, fmt.Errorf("parse rsvp deadline %q: %w", deadline, err)
	}
	if passed {
		return Group{}, nil, ErrDeadlinePassed
	}

	members, err := s.repo.ListMembers(ctx, groupID)
	if err != nil {
		return Group{}, nil, err
	}
	if len(members) == 0 {
		return Group{}, nil, ErrNotFound
	}
	memberIDs := make(map[uuid.UUID]bool, len(members))
	for _, m := range members {
		memberIDs[m.ID] = true
	}
	seen := make(map[uuid.UUID]bool, len(updates))
	for _, u := range updates {
		if u.Attending != "yes" && u.Attending != "no" {
			return Group{}, nil, ErrInvalidAnswer
		}
		if !memberIDs[u.GuestID] || seen[u.GuestID] {
			return Group{}, nil, ErrInvalidMembers
		}
		seen[u.GuestID] = true
	}
	if len(seen) != len(members) {
		return Group{}, nil, ErrInvalidMembers
	}

	if err := s.repo.UpdateAttendances(ctx, groupID, updates); err != nil {
		return Group{}, nil, err
	}
	return s.groupWithMembers(ctx, groupID)
}

// AdminDashboard returns every group with members plus aggregate headcounts.
func (s *Service) AdminDashboard(ctx context.Context) ([]GroupWithMembers, Headcounts, error) {
	groups, err := s.repo.ListAllGroups(ctx)
	if err != nil {
		return nil, Headcounts{}, err
	}
	membersByGroup, err := s.repo.ListAllGuests(ctx)
	if err != nil {
		return nil, Headcounts{}, err
	}
	counts := Headcounts{YesByCategory: map[string]int{}}
	out := make([]GroupWithMembers, 0, len(groups))
	for _, g := range groups {
		members := membersByGroup[g.ID]
		out = append(out, GroupWithMembers{Group: g, Members: members})
		for _, m := range members {
			counts.Total++
			switch m.Attending {
			case "yes":
				counts.Yes++
				category := "uncategorized"
				if m.Category != nil {
					category = *m.Category
				}
				counts.YesByCategory[category]++
			case "no":
				counts.No++
			default:
				counts.Pending++
			}
		}
	}
	return out, counts, nil
}

// categoryPT and attendingPT translate storage enums for the CSV export the
// couple reads.
var categoryPT = map[string]string{"adult": "adulto", "child": "criança", "baby": "bebê", "elderly": "idoso"}

var attendingPT = map[string]string{"pending": "pendente", "yes": "sim", "no": "não"}

// ExportCSV renders the guest list as a spreadsheet-friendly CSV (PT-BR
// headers and values).
func (s *Service) ExportCSV(ctx context.Context) ([]byte, error) {
	groups, _, err := s.AdminDashboard(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	// UTF-8 BOM so Excel-on-Windows renders "criança"/"não" correctly.
	buf.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"grupo", "nome", "principal", "categoria", "presenca"}); err != nil {
		return nil, err
	}
	for _, g := range groups {
		for _, m := range g.Members {
			principal := ""
			if m.IsPrimary {
				principal = "sim"
			}
			category := ""
			if m.Category != nil {
				category = categoryPT[*m.Category]
			}
			if err := w.Write([]string{g.Label, m.FullName, principal, category, attendingPT[m.Attending]}); err != nil {
				return nil, err
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) groupWithMembers(ctx context.Context, groupID uuid.UUID) (Group, []Member, error) {
	group, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return Group{}, nil, err
	}
	members, err := s.repo.ListMembers(ctx, groupID)
	if err != nil {
		return Group{}, nil, err
	}
	return group, members, nil
}

// saoPaulo is the event timezone; RSVP deadline days end at 23:59:59 there.
var saoPaulo = mustLoadLocation("America/Sao_Paulo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // tzdata is embedded via time/tzdata in cmd/api
	}
	return loc
}

// deadlinePassed reports whether now is past the (inclusive) deadline date in
// America/Sao_Paulo. An empty deadline means submissions are always open.
func deadlinePassed(deadline string, now time.Time) (bool, error) {
	deadline = strings.TrimSpace(deadline)
	if deadline == "" {
		return false, nil
	}
	day, err := time.ParseInLocation("2006-01-02", deadline, saoPaulo)
	if err != nil {
		return false, err
	}
	cutoff := day.AddDate(0, 0, 1) // start of the day after the deadline
	return !now.Before(cutoff), nil
}
