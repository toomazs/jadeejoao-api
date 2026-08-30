package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestVerifyPasswordTellsTheReasonsApart pins the distinction that was missing.
//
// Every one of these answers used to come back as "the password is wrong". The
// panel repeated that to the couple, who then went looking for a mistake in a
// password that was right — while the actual fault, a key this server could not
// use, stayed invisible.
func TestVerifyPasswordTellsTheReasonsApart(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{
			name:   "senha realmente errada",
			status: http.StatusBadRequest,
			body:   `{"code":400,"error_code":"invalid_credentials","msg":"Invalid login credentials"}`,
			want:   ErrWrongPassword,
		},
		{
			// What a deploy without SUPABASE_SECRET_KEY gets. Nothing was
			// checked, so nothing about the password is known.
			name:   "chave ausente",
			status: http.StatusUnauthorized,
			body:   `{"message":"No API key found in request"}`,
			want:   ErrAuthMisconfigured,
		},
		{
			name:   "chave recusada",
			status: http.StatusUnauthorized,
			body:   `{"message":"Invalid API key"}`,
			want:   ErrAuthMisconfigured,
		},
		{
			name:   "tentativas demais",
			status: http.StatusTooManyRequests,
			body:   `{"error_code":"over_request_rate_limit"}`,
			want:   ErrAuthRateLimited,
		},
		{
			// Also a 400, and also not the password's fault.
			name:   "e-mail nao confirmado",
			status: http.StatusBadRequest,
			body:   `{"code":400,"error_code":"email_not_confirmed","msg":"Email not confirmed"}`,
			want:   nil, // not ErrWrongPassword; checked below
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			auth := NewSupabaseAuth(server.URL, "sb_secret_de_teste")
			err := auth.VerifyPassword(context.Background(), "alguem@exemplo.com", "seja-o-que-for")
			if err == nil {
				t.Fatal("esperava um erro")
			}
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("esperava %v, veio %v", tc.want, err)
				}
				return
			}
			// The unconfirmed case: whatever it is, it must not accuse the
			// password.
			if errors.Is(err, ErrWrongPassword) {
				t.Fatalf("acusou a senha por um erro que nao e dela: %v", err)
			}
		})
	}
}

// TestVerifyPasswordRefusesWithoutAKey covers the gate do() always had and this
// hand-built request did not.
func TestVerifyPasswordRefusesWithoutAKey(t *testing.T) {
	auth := NewSupabaseAuth("https://exemplo.supabase.co", "")
	err := auth.VerifyPassword(context.Background(), "alguem@exemplo.com", "seja-o-que-for")
	if !errors.Is(err, ErrAuthMisconfigured) {
		t.Fatalf("esperava ErrAuthMisconfigured, veio %v", err)
	}
	if errors.Is(err, ErrWrongPassword) {
		t.Fatal("um servidor sem chave nao pode dizer que a senha esta errada")
	}
}

func TestVerifyPasswordAcceptsTheRightOne(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tudo-certo"}`))
	}))
	defer server.Close()

	auth := NewSupabaseAuth(server.URL, "sb_secret_de_teste")
	if err := auth.VerifyPassword(context.Background(), "alguem@exemplo.com", "a-certa"); err != nil {
		t.Fatalf("esperava sucesso, veio %v", err)
	}
}
