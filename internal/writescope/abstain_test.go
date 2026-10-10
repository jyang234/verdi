package writescope_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The ac-1 producer abstains while its reachability is disclosed as
// unproven (ledger SI-367 (1), after SI-354 (3)'s abstain for
// spec/ritual-effect-witness ac-2).

// producerName is the obligation's producer
// (go-test:internal/writescope:TestRegistry_CoversEveryMutatingVerb).
const producerName = "TestRegistry_CoversEveryMutatingVerb"

// reachabilityGap is ac-1's disclosed gap (ledger SI-321): beyond the
// module's current code and the pinned evasion corpus, a verb that reaches
// a mutating function by a path the source-only analysis neither follows
// nor fails closed on passes the witness undeclared, until reachability is
// rebuilt on SSA with a VTA call graph (backlog BL-136).
const reachabilityGap = "ac-1's reachability beyond the module's current code and the pinned evasion corpus: the four classes reach/doc.go names carry a function value to a verb by a path the source-only analysis neither follows nor fails closed on, so a verb reaching a mutating function through one of them would pass undeclared (ledger SI-321; backlog BL-136, the SSA/VTA rebuild)"

// registryOpenGaps is the ac-1 producer's abstain decision (ledger SI-367
// (1)): the disclosed gaps that keep ac-1 unproven. While any is open the
// producer abstains, never passes; BL-136's rebuild closes the one gap,
// and only then may the producer pass. A fixed table, so a function.
func registryOpenGaps() []string {
	return []string{reachabilityGap}
}

// TestRegistryOpenGaps pins the abstain decision: exactly the reachability
// gap is open, and it names SI-321 and BL-136. Emptying the list, which
// would let the producer pass, fails here.
func TestRegistryOpenGaps(t *testing.T) {
	got := registryOpenGaps()
	if len(got) != 1 || got[0] != reachabilityGap {
		t.Fatalf("registryOpenGaps() = %q, want exactly the reachability gap %q", got, reachabilityGap)
	}
	for _, cite := range []string{"ledger SI-321", "backlog BL-136"} {
		if !strings.Contains(got[0], cite) {
			t.Errorf("the reachability gap %q does not name %s", got[0], cite)
		}
	}
}

// TestRegistryProducer_EndsInItsAbstain pins the producer's shape: its last
// statement asks registryOpenGaps and, while a gap is open, ends the test
// with t.Skipf, which the go-test producer reads as abstain
// (cmd/verdi/testproducer.go, verdictForOutcome). Every check before it
// still fails the test, so the guard stays red on a violation; a producer
// that dropped its abstain would pass, and fails here instead.
func TestRegistryProducer_EndsInItsAbstain(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "witness_test.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == producerName && fn.Recv == nil {
			body = fn.Body
		}
	}
	if body == nil || len(body.List) == 0 {
		t.Fatalf("witness_test.go declares no %s with a body", producerName)
	}
	last, ok := body.List[len(body.List)-1].(*ast.IfStmt)
	if !ok {
		t.Fatalf("%s's last statement is %T, want the if that abstains while registryOpenGaps names a gap", producerName, body.List[len(body.List)-1])
	}
	if !calls(last.Init, "registryOpenGaps") && !calls(last.Cond, "registryOpenGaps") {
		t.Errorf("%s's last statement does not ask registryOpenGaps", producerName)
	}
	if !callsMethod(last.Body, "t", "Skipf") {
		t.Errorf("%s's last statement does not end the test with t.Skipf", producerName)
	}
}

// calls reports whether node contains a call to the function name.
func calls(node ast.Node, name string) bool {
	found := false
	if node == nil {
		return false
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		}
		return !found
	})
	return found
}

// callsMethod reports whether node contains a call recv.method(...).
func callsMethod(node ast.Node, recv, method string) bool {
	found := false
	if node == nil {
		return false
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == method {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == recv {
					found = true
				}
			}
		}
		return !found
	})
	return found
}
