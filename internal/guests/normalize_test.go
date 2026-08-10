package guests

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Eduardo Silva", "eduardo silva"},
		{"trim", "  Eduardo Silva  ", "eduardo silva"},
		{"inner whitespace collapsed", "Eduardo   da\tSilva", "eduardo da silva"},
		{"diacritics stripped", "João Antônio Conceição", "joao antonio conceicao"},
		{"cedilla", "François Muniz", "francois muniz"},
		{"uppercase with accents", "MARIA JOSÉ", "maria jose"},
		{"hyphen kept", "Ana-Clara Ribeiro", "ana-clara ribeiro"},
		{"control chars stripped", "ab\x00c\x07d", "abcd"},
		{"tabs still separate words", "Ana\tClara", "ana clara"},
		{"mixed everything", "  ÂNgeLa  CristINA  ", "angela cristina"},
		{"empty", "", ""},
		{"only spaces", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	in := "João Antônio  da Conceição"
	once := Normalize(in)
	if twice := Normalize(once); twice != once {
		t.Fatalf("not idempotent: %q -> %q", once, twice)
	}
}
