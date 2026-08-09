// Package messages owns the "Recado aos noivos" guestbook. Publicly
// write-only: guests create pending messages, the couple moderates in the
// admin, and no public read endpoint exists in v1 (AD-14).
package messages

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound: message id unknown.
var ErrNotFound = errors.New("message not found")

// Message is one guestbook entry.
type Message struct {
	ID         uuid.UUID
	GroupID    *uuid.UUID
	AuthorName string
	Body       string
	Status     string // pending | approved | rejected
	CreatedAt  time.Time
}

// Repo is the persistence surface for messages.
type Repo interface {
	Insert(ctx context.Context, groupID *uuid.UUID, authorName, body string) (Message, error)
	List(ctx context.Context, statusFilter string) ([]Message, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (Message, error)
}

// Service implements the guestbook.
type Service struct {
	repo Repo
}

// NewService wires the messages service.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// Create records a pending message. Attribution is required — anonymous
// messages are rejected at the schema level (author_name minLength).
func (s *Service) Create(ctx context.Context, groupID *uuid.UUID, authorName, body string) (Message, error) {
	return s.repo.Insert(ctx, groupID, authorName, body)
}

// List returns messages for the admin, optionally filtered by status.
func (s *Service) List(ctx context.Context, statusFilter string) ([]Message, error) {
	return s.repo.List(ctx, statusFilter)
}

// Moderate transitions a message to approved or rejected (admin only; the
// transport layer enforces the enum).
func (s *Service) Moderate(ctx context.Context, id uuid.UUID, status string) (Message, error) {
	return s.repo.UpdateStatus(ctx, id, status)
}
