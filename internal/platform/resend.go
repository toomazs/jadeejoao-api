package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
		return fmt.Errorf("resend: unexpected status %d", resp.StatusCode)
	}
	return nil
}
