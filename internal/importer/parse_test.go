package importer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func loadFixtureCSV(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "guests.csv"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertFixtureRows(t *testing.T, rows []Row) {
	t.Helper()
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5", len(rows))
	}
	if rows[0].Nome != "Eduardo Silva" || rows[0].Grupo != "Família Silva" || !rows[0].Principal {
		t.Fatalf("row 0 wrong: %+v", rows[0])
	}
	if rows[0].Categoria == nil || *rows[0].Categoria != "adult" {
		t.Fatalf("row 0 categoria: %+v", rows[0].Categoria)
	}
	if rows[1].Categoria == nil || *rows[1].Categoria != "child" {
		t.Fatalf("criança not mapped: %+v", rows[1].Categoria)
	}
	if rows[2].Grupo != "" || rows[2].Principal {
		t.Fatalf("own-group row wrong: %+v", rows[2])
	}
	// "x" marks principal; "idosa" maps to elderly.
	if !rows[3].Principal || rows[3].Categoria == nil || *rows[3].Categoria != "elderly" {
		t.Fatalf("row 3 wrong: %+v", rows[3])
	}
	if rows[4].Categoria == nil || *rows[4].Categoria != "baby" {
		t.Fatalf("bebê not mapped: %+v", rows[4].Categoria)
	}
}

func TestParseCSVFixture(t *testing.T) {
	rows, err := ParseFile("convidados.csv", loadFixtureCSV(t))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	assertFixtureRows(t, rows)
}

func TestParseCSVWithBOMAndSemicolons(t *testing.T) {
	csv := "\xef\xbb\xbfNOME;GRUPO;PRINCIPAL;CATEGORIA\r\nEduardo Silva;Família Silva;sim;adulto\r\n"
	rows, err := ParseFile("export.csv", []byte(csv))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(rows) != 1 || rows[0].Nome != "Eduardo Silva" || !rows[0].Principal {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

// TestParseXLSXFixture round-trips the same fixture through a real XLSX file
// built with excelize.
func TestParseXLSXFixture(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	lines := strings.Split(strings.TrimSpace(string(loadFixtureCSV(t))), "\n")
	for i, line := range lines {
		cells := strings.Split(line, ",")
		values := make([]any, len(cells))
		for j, c := range cells {
			values[j] = c
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &values); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}

	rows, err := ParseFile("convidados.xlsx", buf.Bytes())
	if err != nil {
		t.Fatalf("ParseFile xlsx: %v", err)
	}
	assertFixtureRows(t, rows)
}

func TestParseRejectsBadFiles(t *testing.T) {
	var parseErr *ParseError

	// Wrong extension.
	if _, err := ParseFile("lista.pdf", []byte("x")); !errors.As(err, &parseErr) {
		t.Fatalf("pdf: %v", err)
	}

	// CSV extension, ZIP content.
	if _, err := ParseFile("lista.csv", []byte("PK\x03\x04zipzip")); !errors.As(err, &parseErr) {
		t.Fatalf("zip-as-csv: %v", err)
	}

	// XLSX extension, text content.
	if _, err := ParseFile("lista.xlsx", []byte("nome,grupo\na,b")); !errors.As(err, &parseErr) {
		t.Fatalf("text-as-xlsx: %v", err)
	}

	// Unrecognized header: PT-BR message listing what was expected.
	_, err := ParseFile("lista.csv", []byte("nome,telefone\nEduardo,119999\n"))
	if !errors.As(err, &parseErr) {
		t.Fatalf("unknown header: %v", err)
	}
	if !strings.Contains(parseErr.Message, "telefone") || !strings.Contains(parseErr.Message, "nome") {
		t.Fatalf("message must list the problem and the contract: %s", parseErr.Message)
	}

	// Missing nome column.
	if _, err := ParseFile("lista.csv", []byte("grupo,principal\nFamília,x\n")); !errors.As(err, &parseErr) {
		t.Fatalf("missing nome: %v", err)
	}
}

func TestParseRowLevelIssues(t *testing.T) {
	csv := "nome,grupo,categoria\nEduardo Silva,Família,marciano\n,Família,adulto\n"
	rows, err := ParseFile("lista.csv", []byte(csv))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	// Unknown category: imported without category, issue reported.
	if rows[0].Categoria != nil || len(rows[0].Issues) != 1 {
		t.Fatalf("bad category handling: %+v", rows[0])
	}
	// Empty nome: issue reported, row kept for the report but unusable.
	if rows[1].Nome != "" || len(rows[1].Issues) != 1 {
		t.Fatalf("empty nome handling: %+v", rows[1])
	}
}
