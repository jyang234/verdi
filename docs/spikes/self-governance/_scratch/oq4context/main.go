// Command oq4context is spike/self-governance oq-4 fix round 1's helper
// (review finding F2): it builds a real, canonical `verdi
// context conflict --request <file>` document for an accepted-context
// target, through the exact same exported seams
// cmd/verdi/context_test.go's own contextRequestBytes/
// localOperatorConflictRequestBytes test helpers use — never a
// hand-typed JSON literal — so the request this spike feeds the CLI can
// never silently drift from the real wire grammar.
//
// Usage: go run ./docs/spikes/self-governance/_scratch/oq4context <spec-ref> <out-file>
package main

import (
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: oq4context <spec-ref> <out-file>")
		os.Exit(2)
	}
	spec, out := os.Args[1], os.Args[2]

	req := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   contextcompile.PhaseBuild,
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    spec,
	}
	encoded, err := contextcompile.EncodeRequest(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contextcompile.EncodeRequest:", err)
		os.Exit(2)
	}
	accepted, err := contextcompile.DecodeRequest(encoded)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contextcompile.DecodeRequest:", err)
		os.Exit(2)
	}

	wrapped := policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{Kind: policyconflict.TargetAcceptedContext, AcceptedContext: &accepted},
	}
	data, err := policyconflict.EncodeRequest(wrapped)
	if err != nil {
		fmt.Fprintln(os.Stderr, "policyconflict.EncodeRequest:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(2)
	}
	fmt.Printf("wrote %s (%d bytes) for spec=%s phase=build\n", out, len(data), spec)
}
