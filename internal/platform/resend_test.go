package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestResendMailerNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	t.Cleanup(srv.Close)

	m := NewResendMailer("k", "f", []string{"t@example.com"})
	m.baseURL = srv.URL
	if err := m.Send(context.Background(), "s", "h"); err == nil {
		t.Fatal("non-2xx must surface an error for logging")
	}
}
