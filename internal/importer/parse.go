// Package importer turns an uploaded spreadsheet export (CSV or XLSX) into
// guest groups and guests. The couple keeps editing their sheet wherever they
// like; the exported file uploaded in the admin is the entire pipeline —
// there is no Google API integration, ever (AD-10).
package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/guests"
)

// Row is one parsed guest line of the file.
type Row struct {
	Line      int // 1-based line/row number in the file, for error messages
	Nome      string
	Grupo     string
	Principal bool
	Categoria *string // storage enum: adult|child|baby|elderly
	Issues    []RowIssue
}

// RowIssue is a non-fatal problem found while parsing one row.
type RowIssue struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ParseError aborts the import with a user-facing PT-BR message (422).
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string { return e.Message }

const expectedHeadersHint = `A planilha precisa ter uma linha de cabeçalho com as colunas: "nome" (obrigatória), "grupo", "principal" e "categoria" (opcionais). Exporte como CSV ou XLSX e envie o arquivo.`

// categoryByNormalized maps PT-BR sheet values to the storage enum.
var categoryByNormalized = map[string]string{
	"adulto": "adult", "adulta": "adult",
	"crianca": "child",
	"bebe":    "baby",
	"idoso":   "elderly", "idosa": "elderly",
}

// truthyPrincipal marks a row as the group's primary guest.
var truthyPrincipal = map[string]bool{
	"sim": true, "s": true, "x": true, "1": true, "true": true, "verdadeiro": true,
}

// ParseFile detects the format (extension + content sniff) and parses rows.
// Header matching is case- and accent-insensitive (Nome, NOME, nome all work).
func ParseFile(filename string, data []byte) ([]Row, error) {
	isZip := bytes.HasPrefix(data, []byte("PK\x03\x04"))
	switch ext := strings.ToLower(lastExt(filename)); ext {
	case ".csv":
		if isZip {
			return nil, &ParseError{Message: "O arquivo tem extensão .csv, mas o conteúdo não é texto. Exporte novamente como CSV ou envie o .xlsx original."}
		}
		return parseCSV(data)
	case ".xlsx":
		if !isZip {
			return nil, &ParseError{Message: "O arquivo tem extensão .xlsx, mas o conteúdo não é uma planilha válida. Exporte novamente como XLSX."}
		}
		return parseXLSX(data)
	default:
		return nil, &ParseError{Message: fmt.Sprintf("Formato %q não suportado. Envie um arquivo .csv ou .xlsx. %s", ext, expectedHeadersHint)}
	}
}

func lastExt(filename string) string {
	if i := strings.LastIndex(filename, "."); i >= 0 {
		return filename[i:]
	}
	return ""
}

func parseCSV(data []byte) ([]Row, error) {
	// Strip the UTF-8 BOM Excel loves to prepend.
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	// Brazilian Excel exports CSV with semicolons; sniff the header line.
	headerLine, _, _ := bytes.Cut(data, []byte("\n"))
	delimiter := ','
	if bytes.Count(headerLine, []byte(";")) > bytes.Count(headerLine, []byte(",")) {
		delimiter = ';'
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delimiter
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("Não foi possível ler o CSV: %v. %s", err, expectedHeadersHint)}
	}
	return rowsFromRecords(records)
}

func parseXLSX(data []byte) ([]Row, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("Não foi possível abrir o XLSX: %v. %s", err, expectedHeadersHint)}
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, &ParseError{Message: "A planilha não tem nenhuma aba. " + expectedHeadersHint}
	}
	records, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("Não foi possível ler a primeira aba: %v", err)}
	}
	return rowsFromRecords(records)
}

// rowsFromRecords maps the header, validates it, and parses every data row.
func rowsFromRecords(records [][]string) ([]Row, error) {
	if len(records) == 0 {
		return nil, &ParseError{Message: "O arquivo está vazio. " + expectedHeadersHint}
	}

	known := map[string]string{"nome": "nome", "grupo": "grupo", "principal": "principal", "categoria": "categoria"}
	columns := map[string]int{}
	var unrecognized []string
	for i, cell := range records[0] {
		header := guests.Normalize(cell)
		if header == "" {
			continue
		}
		if canonical, ok := known[header]; ok {
			if _, dup := columns[canonical]; !dup {
				columns[canonical] = i
			}
			continue
		}
		unrecognized = append(unrecognized, strings.TrimSpace(cell))
	}
	if _, ok := columns["nome"]; !ok || len(unrecognized) > 0 {
		msg := expectedHeadersHint
		if len(unrecognized) > 0 {
			msg = fmt.Sprintf("Colunas não reconhecidas: %s. %s", strings.Join(unrecognized, ", "), expectedHeadersHint)
		}
		return nil, &ParseError{Message: msg}
	}

	cellAt := func(record []string, column string) string {
		idx, ok := columns[column]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	var rows []Row
	for i, record := range records[1:] {
		line := i + 2 // 1-based, after the header
		empty := true
		for _, c := range record {
			if strings.TrimSpace(c) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		row := Row{Line: line, Nome: cellAt(record, "nome"), Grupo: cellAt(record, "grupo")}
		row.Principal = truthyPrincipal[guests.Normalize(cellAt(record, "principal"))]
		if raw := cellAt(record, "categoria"); raw != "" {
			if mapped, ok := categoryByNormalized[guests.Normalize(raw)]; ok {
				row.Categoria = &mapped
			} else {
				row.Issues = append(row.Issues, RowIssue{Line: line,
					Message: fmt.Sprintf("Categoria %q não reconhecida (use adulto, criança, bebê ou idoso); convidado importado sem categoria.", raw)})
			}
		}
		if row.Nome == "" {
			row.Issues = append(row.Issues, RowIssue{Line: line, Message: "Linha sem nome — convidado ignorado."})
		}
		rows = append(rows, row)
	}
	return rows, nil
}
