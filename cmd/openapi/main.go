// Command openapi exports the generated OpenAPI 3.1 spec. The committed
// openapi.yaml at the repo root is the cross-repo contract both SPAs codegen
// from; a test fails the build whenever it drifts from the code.
package main

//go:generate go run . -out ../../openapi.yaml

import (
	"flag"
	"fmt"
	"os"

	"github.com/jadeejoao/jadeejoao-api/internal/server"
)

func main() {
	out := flag.String("out", "", "write the spec to this file instead of stdout")
	flag.Parse()

	spec, err := server.NewSpecAPI().OpenAPI().YAML()
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate spec:", err)
		os.Exit(1)
	}
	if *out == "" {
		_, _ = os.Stdout.Write(spec)
		return
	}
	if err := os.WriteFile(*out, spec, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write spec:", err)
		os.Exit(1)
	}
}
