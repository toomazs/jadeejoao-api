package pix

import (
	"fmt"
	"strings"
	"testing"
)

// TestBRCodeGoldenBacenExample reproduces the canonical example from the
// BACEN BR Code manual: known-good copia-e-cola, CRC 1D3D.
func TestBRCodeGoldenBacenExample(t *testing.T) {
	got := BRCode(Params{
		Key:          "123e4567-e12b-12d1-a456-426655440000",
		MerchantName: "Fulano de Tal",
		MerchantCity: "BRASILIA",
	})
	want := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"
	if got != want {
		t.Fatalf("golden mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestBRCodeWithAmount(t *testing.T) {
	got := BRCode(Params{
		Key:            "casamento@jadeejoao.com.br",
		MerchantName:   "JADE E JOAO",
		MerchantCity:   "ATIBAIA",
		AmountCentavos: 20000,
	})
	if !strings.Contains(got, "5406200.00") {
		t.Fatalf("amount field 54 missing or wrong: %s", got)
	}
	if !strings.HasPrefix(got, "000201") || !strings.Contains(got, "0014br.gov.bcb.pix") {
		t.Fatalf("malformed payload: %s", got)
	}
	assertCRCSelfConsistent(t, got)
}

func TestBRCodeAmountFormatting(t *testing.T) {
	cases := []struct {
		centavos int64
		want     string
	}{
		{1, "0.01"},
		{99, "0.99"},
		{100, "1.00"},
		{15050, "150.50"},
		{123456789, "1234567.89"},
	}
	for _, tc := range cases {
		if got := formatAmount(tc.centavos); got != tc.want {
			t.Fatalf("formatAmount(%d) = %q, want %q", tc.centavos, got, tc.want)
		}
	}
}

func TestBRCodeTruncatesNameAndCity(t *testing.T) {
	got := BRCode(Params{
		Key:          "k",
		MerchantName: "UM NOME COMPRIDO DEMAIS PARA O CAMPO 59",
		MerchantCity: "UMA CIDADE COMPRIDA DEMAIS",
	})
	if !strings.Contains(got, "5925UM NOME COMPRIDO DEMAIS P") {
		t.Fatalf("name not truncated to 25: %s", got)
	}
	if !strings.Contains(got, "6015UMA CIDADE COMP") {
		t.Fatalf("city not truncated to 15: %s", got)
	}
	assertCRCSelfConsistent(t, got)
}

// assertCRCSelfConsistent recomputes the CRC over everything before the last
// four characters and compares it to the embedded value.
func assertCRCSelfConsistent(t *testing.T, code string) {
	t.Helper()
	if len(code) < 8 {
		t.Fatalf("code too short: %q", code)
	}
	payload, want := code[:len(code)-4], code[len(code)-4:]
	if got := fmt.Sprintf("%04X", crc16(payload)); got != want {
		t.Fatalf("crc mismatch for %q: computed %s, embedded %s", code, got, want)
	}
}
