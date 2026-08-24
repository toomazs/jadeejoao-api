package pix

import (
	"strings"
	"testing"
)

func TestQRSVGEncodesTheBRCode(t *testing.T) {
	const brCode = "00020126500014br.gov.bcb.pix0128jadenascimento.c@outlook.com52040000530398654061" +
		"50.005802BR5911JADE E JOAO6007ATIBAIA62070503***6304725E"

	svg, err := QRSVG(brCode)
	if err != nil {
		t.Fatalf("QRSVG: %v", err)
	}
	if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
		t.Fatalf("not an svg document: %.60s…", svg)
	}
	// currentColor lets the site paint the symbol in the couple's ink.
	if !strings.Contains(svg, `fill="currentColor"`) {
		t.Error("symbol should inherit the surrounding colour")
	}
	// A square viewBox with a real module count, and actual dark modules.
	if !strings.Contains(svg, "viewBox=\"0 0 ") || !strings.Contains(svg, "<path") {
		t.Errorf("missing viewBox or modules: %.120s…", svg)
	}
	if strings.Count(svg, "M") < 100 {
		t.Error("suspiciously few dark modules for a 136-character payload")
	}
}

func TestQRSVGRejectsPayloadsItCannotEncode(t *testing.T) {
	// Well past the 2953-byte ceiling of a version-40 symbol.
	if _, err := QRSVG(strings.Repeat("x", 5000)); err == nil {
		t.Fatal("expected an error for an unencodable payload")
	}
}
