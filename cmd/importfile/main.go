// Command importfile runs a guest-list import from a local CSV/XLSX file
// through the exact same parse → reconcile → apply pipeline as the admin
// endpoint — useful for the initial seed and for operating without a JWT.
//
//	go run ./cmd/importfile -file lista.csv          # dry-run (nothing written)
//	go run ./cmd/importfile -file lista.csv -apply   # apply + persist report
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jadeejoao/jadeejoao-api/internal/importer"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

func main() {
	file := flag.String("file", "", "path to the .csv or .xlsx guest list")
	apply := flag.Bool("apply", false, "write the reconciled plan (default is a dry-run)")
	replace := flag.Bool("replace", false, "DESTRUCTIVE: empty the guest list first, so the file becomes the whole truth (drops every RSVP answer)")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: importfile -file <lista.csv|lista.xlsx> [-apply]")
		os.Exit(2)
	}
	if err := run(*file, *apply, *replace); err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}
}

func run(path string, apply, replace bool) error {
	platform.LoadDotEnv(".env")
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := platform.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	name := filepath.Base(path)
	repo := importer.NewRepo(pool)

	var report importer.Report
	if apply {
		report, err = importer.NewService(repo).Import(ctx, name, data, importer.Options{Replace: replace})
		if err != nil {
			return err
		}
		fmt.Println("== APLICADO ==")
	} else {
		rows, err := importer.ParseFile(name, data)
		if err != nil {
			return err
		}
		// Mirror what -apply would do: replacing reconciles against nothing,
		// so the rehearsal has to look at an empty snapshot too. Otherwise
		// the operator reads a merge report and approves a wipe.
		var snap importer.Snapshot
		if !replace {
			snap, err = repo.Snapshot(ctx)
			if err != nil {
				return err
			}
		}
		_, report = importer.Reconcile(rows, snap)
		if replace {
			fmt.Println("== DRY-RUN, MODO SUBSTITUIR (nada gravado; use -apply) ==")
			fmt.Println("   com -apply, a lista atual seria APAGADA antes de gravar estes nomes.")
		} else {
			fmt.Println("== DRY-RUN (nada gravado; use -apply) ==")
		}
	}

	fmt.Printf("adicionados: %d | atualizados: %d | não mencionados: %d | conflitos: %d | erros: %d\n",
		len(report.Added), len(report.Updated), len(report.Unmatched), len(report.Conflicts), len(report.Errors))
	for _, n := range report.Added {
		fmt.Println("  + " + n)
	}
	for _, n := range report.Updated {
		fmt.Println("  ~ " + n)
	}
	for _, n := range report.Unmatched {
		fmt.Println("  ? ausente da planilha (mantido): " + n)
	}
	for _, c := range report.Conflicts {
		fmt.Printf("  ! conflito: %s (planilha: %s / banco: %s) — %s\n", c.Name, c.FileGroup, c.DBGroup, c.Reason)
	}
	for _, e := range report.Errors {
		fmt.Printf("  x linha %d: %s\n", e.Line, e.Message)
	}
	return nil
}
