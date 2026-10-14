// Package messages owns the "Recado aos noivos" guestbook.
//
// Guests write, and what they write goes up on the site for everyone (AD-14).
// It was write-only at first, on the reasoning that a wedding guestbook is for
// the couple. In practice a wall of other people's good wishes is most of why
// anyone writes one at all, so the couple asked for it to be read back.
//
// Nothing stands between writing and appearing. The couple deletes what should
// not be there, which means a bad message is visible until they look — the
// trade they chose, and the reason every new message already emails them.
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
	CreatedAt  time.Time
}

// Repo is the persistence surface for messages.
type Repo interface {
	Insert(ctx context.Context, groupID *uuid.UUID, authorName, body string) (Message, error)
	List(ctx context.Context) ([]Message, error)
	ListPublic(ctx context.Context, limit int32) ([]Message, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// PublicWallSize caps what the site is given.
//
// The wall draws every message it receives and scrolls them past, so an
// unbounded list would quietly become a heavier page every month. Sixty is more
// than anyone reads in one pass and still comfortably small.
const PublicWallSize int32 = 60

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

// List returns every message, newest first. There is no filter: the couple
// reads all of them.
func (s *Service) List(ctx context.Context) ([]Message, error) {
	return s.repo.List(ctx)
}

// ListPublic returns the newest messages for the wall on the site.
func (s *Service) ListPublic(ctx context.Context) ([]Message, error) {
	return s.repo.ListPublic(ctx, PublicWallSize)
}

// Delete removes a message for good. The couple's only control over the wall,
// and the reason it can be public at all.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
