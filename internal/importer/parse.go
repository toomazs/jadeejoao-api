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
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"github.com/jadeejoao/jadeejoao-api/internal/guests"
)

// Row is one parsed guest line of the file.
type Row struct {
	Line int // 1-based line/row number in the file, for error messages
	Nome string
	// Grupo is the INVITATION group — the people who share one invitation and
	// one primary who answers for all of them. Empty means the guest is their
	// own invitation. It is deliberately not the sheet's "grupo" column; see
	// Circulo.
	Grupo     string
	Principal bool
	Categoria *string // storage enum: adult|teen|child|baby|elderly
	Genero    *string // storage enum: female|male
	Lado      *string // storage enum: bride|groom|both
	// Circulo is the couple's own social bucket (Amigos, Família, Trabalho).
	// It groups nothing: making it the invitation group would put one primary
	// in charge of answering for fifty friends at once.
	Circulo string
	Papel   string // Madrinha, Padrinho, Dama de honra, Celebrante…
	Nota    string // free text, usually kinship ("Marido Renata Gonçalves")
	Issues  []RowIssue
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

const expectedHeadersHint = `A planilha precisa ter uma linha de cabeçalho com a coluna "nome" (obrigatória). Também são lidas, se existirem: "convite" (quem divide o mesmo convite), "principal", "categoria", "gênero", "jj" (de quem é o convidado), "grupo" (amigos, família, trabalho), "lista" (padrinho, madrinha…) e "observação". Exporte como CSV ou XLSX e envie o arquivo.`

// categoryByNormalized maps PT-BR sheet values to the storage enum.
var categoryByNormalized = map[string]string{
	"adulto": "adult", "adulta": "adult",
	"adolescente": "teen",
	"crianca":     "child",
	"bebe":        "baby",
	"idoso":       "elderly", "idosa": "elderly",
}

// genderByNormalized maps the sheet's "gênero" column to the storage enum.
var genderByNormalized = map[string]string{
	"feminino": "female", "f": "female", "mulher": "female",
	"masculino": "male", "m": "male", "homem": "male",
}

// sideByNormalized maps the sheet's "jj" column — whose guest this is.
var sideByNormalized = map[string]string{
	"jade": "bride", "noiva": "bride",
	"joao": "groom", "noivo": "groom",
	"jj": "both", "ambos": "both", "os dois": "both",
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

	// Brazilian Excel's plain "CSV" export is Windows-1252 (ANSI), not UTF-8.
	// Without transcoding, every accented name would be silently stored as
	// mojibake with a corrupt lookup key. CP-1252 decodes any byte, so this
	// only runs when the content is not already valid UTF-8.
	if !utf8.Valid(data) {
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(data)
		if err != nil {
			return nil, &ParseError{Message: "O arquivo não está em um formato de texto reconhecido. Exporte como \"CSV UTF-8\" e envie novamente."}
		}
		data = decoded
	}

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

	// "presenca" is recognized so the API's own CSV export round-trips
	// (export → edit in Excel → re-import); its values are IGNORED — the
	// importer never writes attendance (AD-10).
	known := map[string]string{
		"nome": "nome",
		// The invitation group. Named "convite" rather than "grupo" because
		// the couple's sheet already uses "grupo" for social circles, and
		// reading one as the other would hand a single guest the power to
		// answer for everyone else in their circle.
		"convite": "convite", "familia": "convite",
		"principal": "principal",
		"categoria": "categoria",
		"genero":    "genero", "sexo": "genero",
		"jj": "lado", "lado": "lado", "de quem": "lado",
		"grupo": "circulo", "circulo": "circulo",
		"lista": "papel", "papel": "papel",
		"observacao": "nota", "obs": "nota",
		// Ignored on purpose (AD-10: the importer never writes attendance),
		// but recognized so the couple's own sheet and the API's CSV export
		// both round-trip instead of being rejected.
		"presenca": "presenca", "confirmacao": "presenca",
	}
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

		row := Row{
			Line:    line,
			Nome:    cellAt(record, "nome"),
			Grupo:   cellAt(record, "convite"),
			Circulo: cellAt(record, "circulo"),
			Papel:   cellAt(record, "papel"),
			Nota:    cellAt(record, "nota"),
		}
		row.Principal = truthyPrincipal[guests.Normalize(cellAt(record, "principal"))]
		if raw := cellAt(record, "categoria"); raw != "" {
			if mapped, ok := categoryByNormalized[guests.Normalize(raw)]; ok {
				row.Categoria = &mapped
			} else {
				row.Issues = append(row.Issues, RowIssue{Line: line,
					Message: fmt.Sprintf("Categoria %q não reconhecida (use adulto, adolescente, criança, bebê ou idoso); convidado importado sem categoria.", raw)})
			}
		}
		if raw := cellAt(record, "genero"); raw != "" {
			if mapped, ok := genderByNormalized[guests.Normalize(raw)]; ok {
				row.Genero = &mapped
			} else {
				row.Issues = append(row.Issues, RowIssue{Line: line,
					Message: fmt.Sprintf("Gênero %q não reconhecido (use feminino ou masculino); convidado importado sem gênero.", raw)})
			}
		}
		if raw := cellAt(record, "lado"); raw != "" {
			if mapped, ok := sideByNormalized[guests.Normalize(raw)]; ok {
				row.Lado = &mapped
			} else {
				row.Issues = append(row.Issues, RowIssue{Line: line,
					Message: fmt.Sprintf("Valor %q na coluna JJ não reconhecido (use Jade, João ou JJ); convidado importado sem essa marcação.", raw)})
			}
		}
		if row.Nome == "" {
			row.Issues = append(row.Issues, RowIssue{Line: line, Message: "Linha sem nome — convidado ignorado."})
		}
		rows = append(rows, row)
	}
	return rows, nil
}
