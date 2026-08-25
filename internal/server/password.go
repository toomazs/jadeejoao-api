package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

// MinAdminPasswordLength is short enough to be typed on a phone at a wedding
// venue and long enough that the temporary one cannot simply be reshuffled.
const MinAdminPasswordLength = 10

// AdminPasswordChanger is the slice of Supabase's Auth Admin API this needs.
// An interface so the handler can be tested without a project.
type AdminPasswordChanger interface {
	VerifyPassword(ctx context.Context, email, password string) error
	SetPassword(ctx context.Context, userID, newPassword string) error
}

// PasswordChangeInput is the first-login handshake.
type PasswordChangeInput struct {
	Body struct {
		CurrentPassword string `json:"current_password" minLength:"1" maxLength:"200" doc:"A senha que você usou para entrar."`
		NewPassword     string `json:"new_password" minLength:"10" maxLength:"200" doc:"A nova senha, com pelo menos 10 caracteres."`
	}
}

// PasswordChangeOutput confirms the change; the panel then re-authenticates.
type PasswordChangeOutput struct {
	Body struct {
		Changed bool `json:"changed" doc:"Sempre true; entre novamente com a senha nova."`
	}
}

// registerPasswordChange mounts the one admin operation an account may reach
// while it still carries its temporary password (see adminAuthMiddleware).
//
// The change goes through this API rather than straight from the panel to
// Supabase, for two reasons: the service key stays on the server, and the
// password and the first-login flag are cleared in the same call — a password
// that changed while the flag stayed on would lock the account out forever.
func registerPasswordChange(api huma.API, auth AdminPasswordChanger) {
	huma.Register(api, huma.Operation{
		OperationID: PasswordChangeOperationID,
		Method:      http.MethodPost,
		Path:        "/password",
		Summary:     "Change the admin password",
		Description: "Replaces the signed-in admin's password and clears the first-login obligation. The only admin operation reachable while that obligation stands; everything else answers 403 until it is done.",
		Tags:        []string{"admin"},
	}, func(ctx context.Context, in *PasswordChangeInput) (*PasswordChangeOutput, error) {
		claims, ok := AdminFromContext(ctx)
		if !ok || claims.UserID == "" {
			return nil, huma.Error401Unauthorized("Sessão inválida. Entre novamente.")
		}
		if auth == nil {
			return nil, huma.Error503ServiceUnavailable("Troca de senha indisponível agora. Fale com quem cuida do site.")
		}

		next := in.Body.NewPassword
		if utf8.RuneCountInString(next) < MinAdminPasswordLength {
			return nil, huma.Error422UnprocessableEntity("A senha nova precisa ter pelo menos 10 caracteres.")
		}
		if strings.TrimSpace(next) != next {
			return nil, huma.Error422UnprocessableEntity("A senha não pode começar nem terminar com espaço.")
		}
		if next == in.Body.CurrentPassword {
			return nil, huma.Error422UnprocessableEntity("A senha nova precisa ser diferente da atual.")
		}

		// Identity is already proven by the JWT; this proves it is the owner
		// at the keyboard, so a borrowed token cannot lock them out.
		if err := auth.VerifyPassword(ctx, claims.Email, in.Body.CurrentPassword); err != nil {
			if errors.Is(err, platform.ErrWrongPassword) {
				return nil, huma.Error422UnprocessableEntity("A senha atual não confere.")
			}
			slog.ErrorContext(ctx, "password verification failed", "error", err)
			return nil, huma.Error500InternalServerError("Não conseguimos trocar a senha agora. Tente novamente em instantes.")
		}
		if err := auth.SetPassword(ctx, claims.UserID, next); err != nil {
			slog.ErrorContext(ctx, "password change failed", "error", err)
			return nil, huma.Error500InternalServerError("Não conseguimos trocar a senha agora. Tente novamente em instantes.")
		}

		slog.InfoContext(ctx, "admin password changed", "email", claims.Email)
		out := &PasswordChangeOutput{}
		out.Body.Changed = true
		return out, nil
	})
}
