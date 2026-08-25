// Package emailtpl renders the couple's transactional emails in the wedding's
// own dress: the brand palette, their wordmark, the engraved-plaque grammar of
// the site. Everything is inlined and table-based, because email clients
// (Gmail, Outlook) still ignore <style> blocks and modern layout.
package emailtpl

import (
	"fmt"
	"html"
	"strings"
)

// Brand colours — the same seven from the couple's identity PDF that
// tokens.css carries on the site.
const (
	cream      = "#efe8d8"
	veil       = "#f3eee1"
	olive      = "#50590d"
	deepOlive  = "#464605"
	terracotta = "#7f3717"
	goldSand   = "#d2be81"
	darkGray   = "#3e3e3e"
	ink        = "#1a1818"
)

// wordmarkURL is the couple's "Jade e João" artwork, served from the same
// public bucket as the site's photographs (email clients cannot read the SPA
// bundle, so the asset must live at a stable public URL).
const wordmarkURL = "https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/brand/logo-vertical.png"

// Serif stack mirroring the site: Arapey is not a web-safe font, so the
// fallbacks carry the same bookish tone in every client.
const bodyFont = "'Arapey', Georgia, 'Times New Roman', serif"

// Row is one labelled line of the summary block (a guest and their answer, a
// gift and its value…).
type Row struct {
	Label string
	Value string
}

// Page describes one email: what happened, in the couple's voice.
type Page struct {
	// Preheader is the grey line inbox lists show beside the subject.
	Preheader string
	// Kicker sits above the headline in small caps ("CONFIRMAÇÃO DE PRESENÇA").
	Kicker   string
	Headline string
	// Intro is one sentence under the headline (optional).
	Intro string
	// Rows render as the engraved list inside the card (optional).
	Rows []Row
	// Quote is a guest's own words, set apart in italics (optional).
	Quote string
	// Footnote closes the card — a tally, a reminder (optional).
	Footnote string
}

// Render returns the full HTML document for one email.
func Render(p Page) string {
	var b strings.Builder

	fmt.Fprintf(&b, `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8">`+
		`<meta name="viewport" content="width=device-width,initial-scale=1">`+
		`<meta name="color-scheme" content="light only"></head>`+
		`<body style="margin:0;padding:0;background-color:%s;">`, veil)

	// Hidden preheader: the preview text, kept out of the visible layout.
	if p.Preheader != "" {
		fmt.Fprintf(&b,
			`<div style="display:none;max-height:0;overflow:hidden;opacity:0;">%s</div>`,
			html.EscapeString(p.Preheader))
	}

	fmt.Fprintf(&b,
		`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" `+
			`style="background-color:%s;padding:32px 16px;"><tr><td align="center">`+
			`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" `+
			`style="max-width:560px;">`, veil)

	// The wordmark, as the invitation's letterhead.
	fmt.Fprintf(&b,
		`<tr><td align="center" style="padding:8px 0 28px;">`+
			`<img src="%s" width="220" alt="Jade e João" `+
			`style="display:block;width:220px;max-width:70%%;height:auto;border:0;"></td></tr>`,
		wordmarkURL)

	// The card: cream paper inside a hairline frame, like the site's rooms.
	fmt.Fprintf(&b,
		`<tr><td style="background-color:%s;border:1px solid %s;padding:36px 32px;">`, cream, goldSand)

	if p.Kicker != "" {
		fmt.Fprintf(&b,
			`<p style="margin:0 0 10px;font-family:%s;font-size:12px;letter-spacing:3px;`+
				`text-transform:uppercase;color:%s;">%s</p>`,
			bodyFont, terracotta, html.EscapeString(p.Kicker))
	}

	fmt.Fprintf(&b,
		`<h1 style="margin:0;font-family:%s;font-size:26px;line-height:1.25;font-weight:400;color:%s;">%s</h1>`,
		bodyFont, olive, html.EscapeString(p.Headline))

	if p.Intro != "" {
		fmt.Fprintf(&b,
			`<p style="margin:16px 0 0;font-family:%s;font-size:16px;line-height:1.6;color:%s;">%s</p>`,
			bodyFont, ink, html.EscapeString(p.Intro))
	}

	// A gold hairline, standing in for the site's leaf divider.
	fmt.Fprintf(&b,
		`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">`+
			`<tr><td style="padding:22px 0;"><div style="height:1px;background-color:%s;"></div></td></tr></table>`,
		goldSand)

	if len(p.Rows) > 0 {
		b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">`)
		for _, row := range p.Rows {
			fmt.Fprintf(&b,
				`<tr><td style="padding:7px 0;font-family:%s;font-size:16px;color:%s;">%s</td>`+
					`<td align="right" style="padding:7px 0;font-family:%s;font-size:16px;color:%s;white-space:nowrap;">%s</td></tr>`,
				bodyFont, ink, html.EscapeString(row.Label),
				bodyFont, darkGray, html.EscapeString(row.Value))
		}
		b.WriteString(`</table>`)
	}

	if p.Quote != "" {
		fmt.Fprintf(&b,
			`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">`+
				`<tr><td style="border-left:3px solid %s;padding:4px 0 4px 16px;">`+
				`<p style="margin:0;font-family:%s;font-size:17px;line-height:1.65;font-style:italic;color:%s;">%s</p>`+
				`</td></tr></table>`,
			goldSand, bodyFont, ink, html.EscapeString(p.Quote))
	}

	if p.Footnote != "" {
		fmt.Fprintf(&b,
			`<p style="margin:24px 0 0;font-family:%s;font-size:14px;line-height:1.6;color:%s;">%s</p>`,
			bodyFont, darkGray, html.EscapeString(p.Footnote))
	}

	b.WriteString(`</td></tr>`)

	// The closing band, in deep olive — the site's footer.
	fmt.Fprintf(&b,
		`<tr><td align="center" style="background-color:%s;padding:20px 24px;">`+
			`<p style="margin:0;font-family:%s;font-size:13px;letter-spacing:2px;`+
			`text-transform:uppercase;color:%s;">7 de agosto de 2027 &middot; Atibaia &ndash; SP</p></td></tr>`,
		deepOlive, bodyFont, goldSand)

	b.WriteString(`</table></td></tr></table></body></html>`)
	return b.String()
}
