// Package cli is a synthetic CLI dispatcher: if-arms on the verb, a
// switch on args[0], a guarded flag arm, a key handed to a helper, an
// alias that delegates to another verb's dispatcher with a copy of its
// arguments, and two host verbs: one serves the workbench, one the tools.
package cli

import (
	"context"
	"net/http"

	"example.com/synth/app"
	"example.com/synth/tools"
	"example.com/synth/web"
)

// Run dispatches args the way cmd/verdi's run does.
func Run(args []string) int {
	if len(args) == 0 {
		return 2
	}
	verb := args[0]
	if verb == "direct" {
		return code(app.Direct(context.Background()))
	}
	if verb == "alias" || verb == "alias2" {
		return code(app.ReadOnly(context.Background()))
	}
	if verb == "ds" {
		// Delegates to sub's dispatch with a copy of its arguments, so no
		// "ds ..." subcommand is derived: sub's arms are not ds's own.
		return sub(append([]string(nil), args[1:]...))
	}
	if verb == "serve" {
		web.Register(http.NewServeMux())
		return 0
	}
	if verb == "mcp" {
		return code((&tools.Server{}).Method(context.Background(), "write_tool"))
	}
	switch verb {
	case "sub":
		return sub(args[1:])
	case "op":
		return ops(args[1:])
	}
	return 2
}

func sub(args []string) int {
	if len(args) > 0 && args[0] == "--fast" {
		return code(app.ViaClosure(context.Background()))
	}
	switch args[0] {
	case "write":
		return code(app.Direct(context.Background()))
	case "read":
		return code(app.ReadOnly(context.Background()))
	case "":
		return 2
	}
	return 2
}

func ops(args []string) int {
	op := args[0]
	switch op {
	case "a", "b":
	default:
		return 2
	}
	return runOp(op)
}

func runOp(op string) int {
	switch op {
	case "a":
		return code(app.ViaFuncValue(context.Background()))
	case "b":
		return code(app.ReadOnly(context.Background()))
	}
	return 2
}

func code(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

// verbs is the synthetic verb inventory: a phase per verb, zero for a verb
// recognized but not implemented.
var verbs = map[string]int{
	"direct": 1,
	"sub":    2,
	"op":     3,
	"gone":   0,
}

// notAMap is a package-level variable that is not a map literal.
var notAMap = "direct"

// Phase reads both variables so neither is unused.
func Phase(verb string) (int, string) { return verbs[verb], notAMap }
