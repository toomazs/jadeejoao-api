package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrWrongPassword: the current password does not match. Told apart from a
	// transport failure so the panel can say so instead of blaming the network.
	ErrWrongPassword = errors.New("current password does not match")
	// ErrAuthUserNotFound: no Supabase Auth user with that email.
	ErrAuthUserNotFound = errors.New("auth user not found")
	// ErrAuthMisconfigured: this server's key was refused, so nothing was
	// checked at all. Emphatically not the same as a wrong password: the fault
	// is here, and telling the owner their password is wrong when the server
	// never got to look at it sends them off changing a password that was
	// right the whole time.
	ErrAuthMisconfigured = errors.New("supabase rejected this server's key")
	// ErrAuthRateLimited: Supabase is refusing sign-in attempts for now.
	ErrAuthRateLimited = errors.New("supabase is rate limiting sign-ins")
)

// SupabaseAuth talks to the project's Auth Admin API with the service key.
//
// It exists so the service key never leaves the server: the panel asks this
// API to change a password, and this API asks Supabase. The alternative —
// letting the browser call Supabase directly — would mean shipping a key that
// can rewrite any account in the project.
type SupabaseAuth struct {
	baseURL   string
	secretKey string
	http      *http.Client
}

// NewSupabaseAuth builds the client. A blank secret key yields a client whose
// every call fails loudly, so a misconfigured deploy is obvious rather than
// silently unable to change passwords.
func NewSupabaseAuth(supabaseURL, secretKey string) *SupabaseAuth {
	return &SupabaseAuth{
		baseURL:   strings.TrimRight(supabaseURL, "/") + "/auth/v1",
		secretKey: secretKey,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured reports whether password changes can work at all.
func (s *SupabaseAuth) Configured() bool { return s.secretKey != "" }

func (s *SupabaseAuth) do(ctx context.Context, method, path string, body any, out any) error {
	if !s.Configured() {
		return errors.New("supabase service key is not configured")
	}
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, payload)
	if err != nil {
		return err
	}
	// Supabase wants the key in both places; sending only one is a 401 that
	// looks like a bad key.
	req.Header.Set("apikey", s.secretKey)
	req.Header.Set("Authorization", "Bearer "+s.secretKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("supabase auth %s %s: %d: %s", method, path, resp.StatusCode, string(raw))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// VerifyPassword checks a password by attempting a token grant with it.
//
// The JWT alone already proves who is calling, so this is not about identity —
// it is about a stolen or borrowed token not being enough to lock the real
// owner out of their own account.
func (s *SupabaseAuth) VerifyPassword(ctx context.Context, email, password string) error {
	// This request is built by hand rather than through do(), so it never met
	// do()'s configuration gate. Without this, a deploy missing the key sends
	// a blank apikey, is refused with 401, and every admin is told their own
	// password is wrong.
	if !s.Configured() {
		return fmt.Errorf("%w: SUPABASE_SECRET_KEY is empty", ErrAuthMisconfigured)
	}
	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/token?grant_type=password", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", s.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Which 400 this is matters, and only the body says. Supabase answers 400
	// for a wrong password, for an unconfirmed address and for a malformed
	// request alike; reading the code is the difference between telling the
	// owner something true and telling them their password is wrong when it
	// is not.
	var failure struct {
		ErrorCode string `json:"error_code"`
		Error     string `json:"error"`
		Message   string `json:"msg"`
	}
	_ = json.Unmarshal(raw, &failure)
	code := failure.ErrorCode
	if code == "" {
		code = failure.Error
	}

	switch {
	// The only two that mean what the panel is about to say out loud.
	case code == "invalid_credentials" || code == "invalid_grant":
		return ErrWrongPassword
	// 401 is never about the password. It is this server's key being absent or
	// refused, and the attempt died before Supabase looked at any credential.
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrAuthMisconfigured, strings.TrimSpace(string(raw)))
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrAuthRateLimited
	default:
		return fmt.Errorf("supabase token grant: %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
}

// SetPassword changes a user's password and clears the first-login obligation
// in one call, so the two can never drift apart: a password that changed while
// the flag stayed on would lock the account out of the panel forever.
func (s *SupabaseAuth) SetPassword(ctx context.Context, userID, newPassword string) error {
	return s.do(ctx, http.MethodPut, "/admin/users/"+userID, map[string]any{
		"password":     newPassword,
		"app_metadata": map[string]any{"must_change_password": false},
	}, nil)
}

// AuthUser is the slice of a Supabase Auth user this API cares about.
type AuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// SetDisplayName records how somebody is addressed in the panel.
//
// In user_metadata rather than app_metadata: this is theirs, not a claim about
// their permissions, and there is no harm in them changing it. The greeting
// rides along because Portuguese has a gender and a name does not tell you
// which one — guessing it from the letters is how software calls people the
// wrong thing.
func (s *SupabaseAuth) SetDisplayName(ctx context.Context, userID, name, greeting string) error {
	return s.do(ctx, http.MethodPut, "/admin/users/"+userID, map[string]any{
		"user_metadata": map[string]any{
			"display_name": name,
			"greeting":     greeting,
		},
	}, nil)
}

// FindUserByEmail looks up an account so the provisioning command can tell
// "create" from "reset".
func (s *SupabaseAuth) FindUserByEmail(ctx context.Context, email string) (AuthUser, error) {
	var page struct {
		Users []AuthUser `json:"users"`
	}
	if err := s.do(ctx, http.MethodGet, "/admin/users?per_page=200", nil, &page); err != nil {
		return AuthUser{}, err
	}
	for _, u := range page.Users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return AuthUser{}, ErrAuthUserNotFound
}

// CreateAdminUser provisions one panel account holding a temporary password,
// already confirmed (nobody is checking a mailbox for this) and already
// obliged to change it on first use.
func (s *SupabaseAuth) CreateAdminUser(ctx context.Context, email, tempPassword string) (AuthUser, error) {
	var created AuthUser
	err := s.do(ctx, http.MethodPost, "/admin/users", map[string]any{
		"email":         email,
		"password":      tempPassword,
		"email_confirm": true,
		"app_metadata":  map[string]any{"must_change_password": true},
	}, &created)
	return created, err
}

// ResetAdminPassword puts an existing account back on a temporary password and
// re-arms the obligation — the recovery path when someone forgets theirs.
func (s *SupabaseAuth) ResetAdminPassword(ctx context.Context, userID, tempPassword string) error {
	return s.do(ctx, http.MethodPut, "/admin/users/"+userID, map[string]any{
		"password":     tempPassword,
		"app_metadata": map[string]any{"must_change_password": true},
	}, nil)
}
