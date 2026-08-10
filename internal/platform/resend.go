package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrPermanentSend marks a 4xx rejection from Resend: retrying the identical
// request cannot succeed (bad key, unverified domain, rejected recipient).
var ErrPermanentSend = errors.New("permanent email rejection")

// ResendMailer sends transactional email through the Resend REST API
// (https://resend.com). Used for RSVP notifications to the couple; the
// feature is off entirely when no API key is configured.
type ResendMailer struct {
	apiKey  string
	from    string
	to      []string
	baseURL string
	client  *http.Client
}

// NewResendMailer builds the mailer. from is the sender identity (Resend's
// sandbox sender works until the couple verifies a domain); to is the
// couple's personal inboxes (RSVP_NOTIFY_EMAILS).
func NewResendMailer(apiKey, from string, to []string) *ResendMailer {
	return &ResendMailer{
		apiKey:  apiKey,
		from:    from,
		to:      to,
		baseURL: "https://api.resend.com",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Send posts one email. Callers treat failures as log-only — notifications
// must never affect guest-facing behavior.
func (m *ResendMailer) Send(ctx context.Context, subject, html string) error {
	payload, err := json.Marshal(map[string]any{
		"from":    m.from,
		"to":      m.to,
		"subject": subject,
		"html":    html,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body says WHY (unverified domain, bad recipient…) — this is a
		// log-only feature, so the error message is the whole diagnosis.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		err := fmt.Errorf("resend: status %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("%w: %w", ErrPermanentSend, err)
		}
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse
	return nil
}
