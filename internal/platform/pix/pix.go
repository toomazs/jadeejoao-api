// Package pix generates static PIX BR Code payloads (copia-e-cola) in the
// EMV-QRCPS TLV format defined by the BACEN BR Code manual. No payment
// gateway: the couple's PIX key and the amount are embedded in the string and
// the SPAs render the QR image client-side.
package pix

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Params describes one static BR Code.
type Params struct {
	Key            string // the couple's PIX key (from env)
	MerchantName   string // field 59, truncated to 25 chars
	MerchantCity   string // field 60, truncated to 15 chars
	AmountCentavos int64  // field 54; 0 omits the amount
	TxID           string // field 62-05; defaults to "***" (static BR Code)
}

// BRCode renders the copia-e-cola payload, CRC included. Pure function.
// Merchant name/city are sanitized to plain ASCII (accents stripped) and
// truncated on runes — a byte-sliced multi-byte character would corrupt the
// payload in ways some bank apps reject. Non-positive amounts omit field 54.
func BRCode(p Params) string {
	if p.TxID == "" {
		p.TxID = "***"
	}
	var b strings.Builder
	b.WriteString(tlv("00", "01")) // payload format indicator
	b.WriteString(tlv("26",        // merchant account information
		tlv("00", "br.gov.bcb.pix")+tlv("01", p.Key)))
	b.WriteString(tlv("52", "0000")) // merchant category code (none)
	b.WriteString(tlv("53", "986"))  // currency: BRL
	if p.AmountCentavos > 0 {
		b.WriteString(tlv("54", formatAmount(p.AmountCentavos)))
	}
	b.WriteString(tlv("58", "BR"))
	b.WriteString(tlv("59", truncate(sanitizeASCII(p.MerchantName), 25)))
	b.WriteString(tlv("60", truncate(sanitizeASCII(p.MerchantCity), 15)))
	b.WriteString(tlv("62", tlv("05", p.TxID))) // additional data: txid
	b.WriteString("6304")                       // CRC tag+length, value included in the checksum
	payload := b.String()
	return payload + fmt.Sprintf("%04X", crc16(payload))
}

// sanitizeASCII strips diacritics and drops any remaining non-ASCII rune, so
// env-supplied merchant fields always yield EMV-safe single-byte characters.
func sanitizeASCII(s string) string {
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) || r > unicode.MaxASCII {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// tlv renders one id-length-value element. Lengths are two decimal digits.
func tlv(id, value string) string {
	return fmt.Sprintf("%s%02d%s", id, len(value), value)
}

// formatAmount renders centavos as the decimal string field 54 expects.
func formatAmount(centavos int64) string {
	return fmt.Sprintf("%d.%02d", centavos/100, centavos%100)
}

// truncate cuts on runes, never mid-character. After sanitizeASCII the input
// is single-byte, but the guard keeps the function safe on its own.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}

// crc16 is CRC-16/CCITT-FALSE (poly 0x1021, init 0xFFFF), per the BR Code
// manual, computed over the payload including the trailing "6304".
func crc16(s string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8
		for bit := 0; bit < 8; bit++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
