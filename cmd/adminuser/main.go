// Command adminuser provisions the couple's panel accounts.
//
// It exists because Supabase has no notion of "temporary password": an account
// is created holding one, and the obligation to replace it is written into
// app_metadata — which the account holder cannot clear themselves, so the API
// can enforce it (see internal/server/auth.go).
//
//	go run ./cmd/adminuser -email jade@example.com                  # create
//	go run ./cmd/adminuser -email jade@example.com -reset           # forgot it
//	go run ./cmd/adminuser -list                                    # who exists
//
// The password is read from ADMIN_TEMP_PASSWORD, or asked for on the terminal.
// It is never a flag: flags land in shell history and in `ps`.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
	"github.com/jadeejoao/jadeejoao-api/internal/server"
)

func main() {
	email := flag.String("email", "", "email of the panel account")
	reset := flag.Bool("reset", false, "put an existing account back on a temporary password")
	list := flag.Bool("list", false, "list the panel accounts that exist")
	name := flag.String("name", "", "how the panel addresses this person, e.g. Jade")
	greeting := flag.String("greeting", "", "Bem-vindo or Bem-vinda — Portuguese has a gender and a name does not tell you which")
	flag.Parse()

	if err := run(*email, *reset, *list, *name, *greeting); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(email string, reset, list bool, name, greeting string) error {
	platform.LoadDotEnv(".env")
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	auth := platform.NewSupabaseAuth(cfg.SupabaseURL, cfg.SupabaseSecretKey)
	if !auth.Configured() {
		return errors.New("SUPABASE_SECRET_KEY está vazia — sem ela não dá para criar contas")
	}
	ctx := context.Background()

	if list {
		fmt.Println("contas do painel (ADMIN_EMAILS):")
		for _, e := range cfg.AdminEmails {
			user, err := auth.FindUserByEmail(ctx, e)
			switch {
			case errors.Is(err, platform.ErrAuthUserNotFound):
				fmt.Printf("  %-34s ainda não existe\n", e)
			case err != nil:
				return err
			default:
				fmt.Printf("  %-34s ok (%s)\n", e, user.ID)
			}
		}
		return nil
	}

	if email == "" {
		return errors.New("informe -email (ou -list)")
	}
	// The allowlist is what the API checks on every request. Creating an
	// account outside it would produce a login that works and a panel that
	// answers 403 to everything — confusing, and easy to avoid here.
	if !allowed(cfg.AdminEmails, email) {
		return fmt.Errorf("%s não está em ADMIN_EMAILS (%s) — a conta entraria e não conseguiria fazer nada",
			email, strings.Join(cfg.AdminEmails, ", "))
	}

	// Naming somebody is not a password operation and must not ask for one:
	// making the couple type a temporary password to fix a spelling would be
	// the kind of friction that stops it being fixed.
	if name != "" && !reset {
		user, err := auth.FindUserByEmail(ctx, email)
		if err != nil {
			return err
		}
		if greeting == "" {
			return errors.New("informe -greeting (Bem-vindo ou Bem-vinda) junto com -name")
		}
		if err := auth.SetDisplayName(ctx, user.ID, name, greeting); err != nil {
			return err
		}
		fmt.Printf("%s agora é %q, cumprimentado com %q\n", email, name, greeting)
		return nil
	}

	temp, err := readTempPassword()
	if err != nil {
		return err
	}
	if len(temp) < server.MinAdminPasswordLength {
		return fmt.Errorf("a senha temporária precisa ter pelo menos %d caracteres", server.MinAdminPasswordLength)
	}

	user, err := auth.FindUserByEmail(ctx, email)
	switch {
	case errors.Is(err, platform.ErrAuthUserNotFound):
		if reset {
			return fmt.Errorf("%s ainda não existe — rode sem -reset para criar", email)
		}
		created, err := auth.CreateAdminUser(ctx, email, temp)
		if err != nil {
			return err
		}
		fmt.Printf("criada: %s (%s)\n", email, created.ID)
	case err != nil:
		return err
	default:
		if !reset {
			return fmt.Errorf("%s já existe — use -reset para devolver a senha temporária", email)
		}
		if err := auth.ResetAdminPassword(ctx, user.ID, temp); err != nil {
			return err
		}
		fmt.Printf("resetada: %s (%s)\n", email, user.ID)
	}

	fmt.Println("no primeiro acesso o painel vai exigir a troca — até lá, todo o resto responde 403.")
	return nil
}

func allowed(list []string, email string) bool {
	for _, e := range list {
		if strings.EqualFold(strings.TrimSpace(e), strings.TrimSpace(email)) {
			return true
		}
	}
	return false
}

// readTempPassword prefers the environment and falls back to the terminal.
// Never a flag: those end up in shell history and in the process list.
func readTempPassword() (string, error) {
	if v := os.Getenv("ADMIN_TEMP_PASSWORD"); v != "" {
		return v, nil
	}
	fmt.Print("senha temporária: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
