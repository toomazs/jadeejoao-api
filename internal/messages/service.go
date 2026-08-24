// Package messages owns the "Recado aos noivos" guestbook. Publicly
// write-only: guests create pending messages, the couple moderates in the
// admin, and no public read endpoint exists in v1 (AD-14).
package messages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
	"github.com/jadeejoao/jadeejoao-api/internal/platform/emailtpl"
)

// ErrNotFound: message id unknown.
var ErrNotFound = errors.New("message not found")

// ErrUnknownGroup: the optional group_id references no existing group.
var ErrUnknownGroup = errors.New("unknown guest group")

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

// Mailer sends one notification email. Implemented by platform.ResendMailer;
// nil disables notifications entirely.
type Mailer interface {
	Send(ctx context.Context, subject, html string) error
}

// Service implements the guestbook.
type Service struct {
	repo             Repo
	mailer           Mailer
	notifyRetryDelay time.Duration
	notifyWG         sync.WaitGroup
}

// NewService wires the messages service. mailer may be nil (notifications off).
func NewService(repo Repo, mailer Mailer) *Service {
	return &Service{repo: repo, mailer: mailer, notifyRetryDelay: 2 * time.Second}
}

// Create records a pending message. Attribution is required — anonymous
// messages are rejected at the schema level (author_name minLength).
func (s *Service) Create(ctx context.Context, groupID *uuid.UUID, authorName, body string) (Message, error) {
	message, err := s.repo.Insert(ctx, groupID, authorName, body)
	if err != nil {
		return Message{}, err
	}
	s.notifyMessage(message)
	return message, nil
}

// notifyMessage emails the couple the guest's words. Fire-and-forget with a
// single retry, exactly like the RSVP notification: it runs detached from the
// request and any failure is only logged — a guest never waits on, or hears
// about, email delivery.
func (s *Service) notifyMessage(message Message) {
	if s.mailer == nil {
		return
	}
	subject, body := buildMessageEmail(message)
	retryDelay := s.notifyRetryDelay
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		send := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return s.mailer.Send(ctx, subject, body)
		}
		err := send()
		// Retry once — but not on permanent (4xx) rejections, where an
		// identical second request cannot succeed.
		if err != nil && !errors.Is(err, platform.ErrPermanentSend) {
			time.Sleep(retryDelay)
			err = send()
		}
		if err != nil {
			slog.Error("message notification failed", "message", message.ID, "error", err)
		}
	}()
}

// DrainNotifications waits for in-flight notification goroutines, bounded by
// ctx — called on graceful shutdown so a just-written message's email is not
// killed silently with the process.
func (s *Service) DrainNotifications(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.notifyWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("shutdown before message notifications drained")
	}
}

// buildMessageEmail renders the guestbook notification in the wedding's dress.
func buildMessageEmail(message Message) (subject, body string) {
	// The author's name reaches the subject line: strip control characters a
	// pasted string could carry.
	cleanAuthor := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, message.AuthorName)

	subject = fmt.Sprintf("Novo recado: %s", cleanAuthor)
	body = emailtpl.Render(emailtpl.Page{
		Preheader: fmt.Sprintf("%s deixou um recado para vocês.", cleanAuthor),
		Kicker:    "Recado aos noivos",
		Headline:  fmt.Sprintf("%s deixou um recado!", message.AuthorName),
		Quote:     message.Body,
		Footnote:  "Guardado com carinho — vocês veem todos os recados no painel.",
	})
	return subject, body
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
