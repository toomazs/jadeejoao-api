package messages

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

type stubRepo struct{ inserted Message }

func (r *stubRepo) Insert(_ context.Context, groupID *uuid.UUID, authorName, body string) (Message, error) {
	r.inserted = Message{
		ID:         uuid.New(),
		GroupID:    groupID,
		AuthorName: authorName,
		Body:       body,
		CreatedAt:  time.Now(),
	}
	return r.inserted, nil
}
func (r *stubRepo) List(context.Context) ([]Message, error) { return nil, nil }

type fakeMailer struct {
	sent     chan [2]string
	attempts int
	err      error
}

func (m *fakeMailer) Send(_ context.Context, subject, html string) error {
	m.attempts++
	if m.err != nil {
		return m.err
	}
	m.sent <- [2]string{subject, html}
	return nil
}

func TestCreateNotifiesTheCouple(t *testing.T) {
	mailer := &fakeMailer{sent: make(chan [2]string, 1)}
	svc := NewService(&stubRepo{}, mailer)

	if _, err := svc.Create(context.Background(), nil, "Tia Marta", "Que vocês sejam muito felizes!"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	select {
	case msg := <-mailer.sent:
		subject, body := msg[0], msg[1]
		if !strings.Contains(subject, "Tia Marta") {
			t.Fatalf("subject = %q", subject)
		}
		// The guest's own words reach the couple, in the wedding's dress.
		if !strings.Contains(body, "Que vocês sejam muito felizes!") ||
			!strings.Contains(body, "Tia Marta") ||
			!strings.Contains(body, "#50590d") ||
			!strings.Contains(body, "brand/logo-vertical.png") {
			t.Fatalf("body = %q", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification never sent")
	}
}

// A guest's message must be recorded even with no mailer configured — the
// notification is a courtesy, never a requirement.
func TestCreateWorksWithoutMailer(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, nil)

	message, err := svc.Create(context.Background(), nil, "Zé", "Parabéns!")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if message.AuthorName != "Zé" || repo.inserted.Body != "Parabéns!" {
		t.Fatalf("message not recorded: %+v", message)
	}
}

func TestNotifyRetriesOnceThenGivesUp(t *testing.T) {
	mailer := &fakeMailer{sent: make(chan [2]string, 1), err: errors.New("network down")}
	svc := NewService(&stubRepo{}, mailer)
	svc.notifyRetryDelay = time.Millisecond

	if _, err := svc.Create(context.Background(), nil, "Ana", "oi"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	svc.DrainNotifications(context.Background())
	if mailer.attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (first try + one retry)", mailer.attempts)
	}
}

// A 4xx means the identical request cannot succeed — retrying only burns quota.
func TestNotifyDoesNotRetryPermanentRejections(t *testing.T) {
	mailer := &fakeMailer{
		sent: make(chan [2]string, 1),
		err:  errors.New("resend: status 422: " + platform.ErrPermanentSend.Error()),
	}
	mailer.err = errors.Join(platform.ErrPermanentSend, mailer.err)
	svc := NewService(&stubRepo{}, mailer)
	svc.notifyRetryDelay = time.Millisecond

	if _, err := svc.Create(context.Background(), nil, "Ana", "oi"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	svc.DrainNotifications(context.Background())
	if mailer.attempts != 1 {
		t.Fatalf("attempts = %d, want exactly 1", mailer.attempts)
	}
}
