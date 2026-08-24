// Command mailtest renders the real notification templates to HTML files and,
// given an address, sends them through the configured Resend account — so the
// couple's team can check the emails before a guest ever triggers one.
//
//	go run ./cmd/mailtest                 # only writes the HTML previews
//	go run ./cmd/mailtest alguem@dominio  # also sends them
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
	"github.com/jadeejoao/jadeejoao-api/internal/platform/emailtpl"
)

func main() {
	platform.LoadDotEnv(".env")

	var to string
	if len(os.Args) > 1 {
		to = os.Args[1]
	}

	rsvp := emailtpl.Render(emailtpl.Page{
		Preheader: "Família Tomaz respondeu ao convite.",
		Kicker:    "Confirmação de presença",
		Headline:  "Família Tomaz respondeu ao convite!",
		Intro:     "Veja como ficou a resposta de cada pessoa do grupo:",
		Rows: []emailtpl.Row{
			{Label: "Eduardo Tomaz", Value: "vai ✓"},
			{Label: "Convidado(a) de Eduardo", Value: "vai ✓"},
			{Label: "Tia Marta", Value: "não vai"},
		},
		Footnote: "Resumo do grupo: 2 sim, 1 não.",
	})

	message := emailtpl.Render(emailtpl.Page{
		Preheader: "Eduardo Tomaz deixou um recado para vocês.",
		Kicker:    "Recado aos noivos",
		Headline:  "Eduardo Tomaz deixou um recado!",
		Quote:     "Que a casa de vocês seja sempre cheia — de gente, de gato e de comida boa. Contem comigo pra tudo, sempre. Amo vocês!",
		Footnote:  "O recado entra como pendente — aprove no painel para publicá-lo.",
	})

	previews := []struct{ subject, body, file string }{
		{"[TESTE] Confirmação de presença: Família Tomaz", rsvp, "email-confirmacao.html"},
		{"[TESTE] Novo recado: Eduardo Tomaz", message, "email-recado.html"},
	}

	for _, p := range previews {
		if err := os.WriteFile(p.file, []byte(p.body), 0o644); err != nil {
			fmt.Println("não salvou", p.file+":", err)
		} else {
			fmt.Println("salvo:", p.file)
		}
	}

	if to == "" {
		fmt.Println("(nenhum destinatário informado — só gerei os previews)")
		return
	}

	mailer := platform.NewResendMailer(os.Getenv("RESEND_API_KEY"), os.Getenv("RESEND_FROM"), []string{to})
	for _, p := range previews {
		if err := mailer.Send(context.Background(), p.subject, p.body); err != nil {
			fmt.Printf("FALHOU %q: %v\n", p.subject, err)
			continue
		}
		fmt.Printf("ENVIADO para %s: %s\n", to, p.subject)
	}
}
