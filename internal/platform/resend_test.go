package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendMailerSend(t *testing.T) {
	var got struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
	}
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/emails" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	m := NewResendMailer("re_test_key", "Casamento <noivos@example.com>", []string{"jade@example.com", "joao@example.com"})
	m.baseURL = srv.URL

	if err := m.Send(context.Background(), "Confirmação", "<p>oi</p>"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if auth != "Bearer re_test_key" {
		t.Fatalf("auth header = %q", auth)
	}
	if got.From == "" || len(got.To) != 2 || got.Subject != "Confirmação" || got.HTML != "<p>oi</p>" {
		t.Fatalf("payload wrong: %+v", got)
	}
}

func TestResendMailer4xxIsPermanentWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"domain not verified"}`))
	}))
	t.Cleanup(srv.Close)

	m := NewResendMailer("k", "f", []string{"t@example.com"})
	m.baseURL = srv.URL
	err := m.Send(context.Background(), "s", "h")
	if !errors.Is(err, ErrPermanentSend) {
		t.Fatalf("4xx must be permanent, got %v", err)
	}
	if !strings.Contains(err.Error(), "domain not verified") {
		t.Fatalf("error must carry the response body: %v", err)
	}
}

func TestResendMailer5xxIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	m := NewResendMailer("k", "f", []string{"t@example.com"})
	m.baseURL = srv.URL
	err := m.Send(context.Background(), "s", "h")
	if err == nil || errors.Is(err, ErrPermanentSend) {
		t.Fatalf("5xx must be a retryable error, got %v", err)
	}
}
