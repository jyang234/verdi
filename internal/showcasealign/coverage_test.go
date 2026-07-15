// TestShowcaseCoverage is the capability-coverage inventory (Task 3.2,
// story spec/showcase-drift-gate, docs/design/plans/2026-07-14-public-
// rollout-plan.md's Phase 3): the heart of the showcase drift gate. It
// enumerates every shipped capability along three axes — CLI verbs, MCP
// tools, workbench surfaces — and requires each to carry at least one
// showcase-backed e2e evidence file (ledger L-B): a Playwright spec under
// e2e/tests/ whose text matches /SHOWCASE\./, or a Go e2e test file whose
// text matches /examples\/showcase/ (CLI behavioral paths are Go-driven,
// per the testing rules — Playwright never inspects CLI output).
//
// The committed showcaseCoverage map below is the inventory. Both
// directions are checked: an enumerated capability absent from the map is
// a gap (t.Errorf, "showcase-coverage gap: <capability> has no
// showcase-backed e2e evidence"); a map entry naming a capability that no
// longer exists (renamed verb, retired tool, ...) is equally an error —
// stale inventory rots silently otherwise. Every mapped evidence file must
// exist under the repo root and its bytes must match its marker regexp —
// a mapping is proof, not a wish.
//
// THIS TEST IS EXPECTED TO BE RED AT THIS COMMIT. Task 3.2's own
// instructions require writing the true, honest map — mapping only
// capabilities that ALREADY have genuine showcase-backed evidence — and
// leaving every other capability unmapped so this test names it as a gap.
// Task 3.4 closes those gaps (new Playwright specs and Go e2e tests); this
// file's failure output IS Task 3.4's punch list. Faking a mapping to turn
// this test green would defeat the entire gate — never done here.
//
// Three kinds of gap are recorded in the map below, each real and
// disclosed rather than papered over:
//
//  1. No candidate evidence file exists at all yet (most CLI verbs, all
//     MCP tools, wall-badges, derivation-drawer, disclosures).
//  2. A candidate Playwright spec exists and carries the SHOWCASE. marker,
//     but ONLY via a SHOWCASE-classified fixture whose provisioner prose
//     is still rudimentary (fixtures.ts's own NOTE: income-verification,
//     refi-decline-audit, refi-decline-replay, decline-slot-wall,
//     stale-decline-notices, the draft-boards family) — the controller
//     directive for this task requires leaving these unmapped rather than
//     counting rudimentary-fixture backing as real coverage
//     (board-review-mode, obligation-wall, evidence-slot; wall-receipts
//     transitively, since its only two natural host specs are these same
//     two rudimentary-backed files).
//  3. A candidate Go test file matches the /examples\/showcase/ marker
//     textually (usually in a comment) but its own doc comment discloses
//     that it deliberately does NOT exercise the showcase corpus (a
//     scratch/self-contained fixture instead, to avoid coupling to golden
//     SHAs another task is still rebaking) — mapping it would be a false
//     mapping, not evidence (cli:rollup: cmd/verdi/rollup_test.go; the
//     whole MCP axis: internal/mcpserve/fixture_test.go).
//
// Full reasoning for every mapped and every gapped capability lives in
// .superpowers/sdd/task-3.2-report.md (not duplicated here at length, to
// keep this file's signal — the map itself — legible).
package showcasealign

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/mcpserve"
)

// coverageEvidence is one proof that a capability is exercised against the
// showcase: a Playwright spec whose text contains "SHOWCASE." or a Go e2e
// test whose text contains "examples/showcase" (ledger L-B).
type coverageEvidence struct {
	file   string // repo-relative
	marker string // regexp the file's text must match
}

var playwright = func(f string) coverageEvidence { return coverageEvidence{"e2e/tests/" + f, `SHOWCASE\.`} }
var goE2E = func(f string) coverageEvidence { return coverageEvidence{f, `examples/showcase`} }

// showcaseCoverage is the committed inventory (spec §5). Keys for the CLI
// axis are "cli:<verb>", MCP axis "mcp:<tool>", workbench axis
// "wb:<surface>". Every enumerated capability MUST have an entry; entries
// for unknown capabilities fail the test (both directions checked).
//
// Every entry below was verified by hand (not merely by the marker regexp,
// which a stale or scratch-fixture comment can satisfy without proving
// anything — see the rollup/mcpserve note above) to genuinely drive the
// named capability against real showcase-sourced content:
//
//   - cli:lint  — internal/showcasealign/lintclean_test.go's
//     TestShowcaseLintClean runs the REAL "lint" verb (runBinary(t, store,
//     "lint")) against provisionShowcaseStore, itself built from
//     examples/showcase's own layers.
//   - cli:matrix — cmd/verdi/matrix_test.go's buildCorpusRepo builds
//     examples/showcase's committed zone into a real fixturegit repo and
//     many of its tests call cmdMatrix directly against it
//     (TestCmdMatrix_Golden and neighbors).
//   - cli:sync — cmd/verdi/sync_test.go's buildTestStore copies the REAL
//     stale-decline spec.md out of examples/showcase into its scratch
//     store, and TestRunSync_* call runSync (cmdSync's own entry point)
//     against it.
//
// Every other CLI verb (design, accept, feature, build, align, serve, mcp,
// rollup, close, dex, gc, gate, board, audit) and every MCP tool remain
// UNMAPPED below — real gaps, not oversights; see the report for why each
// candidate file was rejected.
var showcaseCoverage = map[string][]coverageEvidence{
	// --- CLI verbs (verbPhase>0 entries, plus "lint") ---
	"cli:lint":   {goE2E("internal/showcasealign/lintclean_test.go")},
	"cli:matrix": {goE2E("cmd/verdi/matrix_test.go")},
	"cli:sync":   {goE2E("cmd/verdi/sync_test.go")},

	// --- MCP tools: none genuinely showcase-backed yet (Task 3.4 gap) ---

	// --- Workbench surfaces ---
	"wb:board":                {playwright("10-board-projection.spec.ts")},
	"wb:board-scoping-canvas": {playwright("30-board-scoping-canvas.spec.ts")},
	"wb:diagram-editor":       {playwright("37-board-diagram-editor.spec.ts")},
	"wb:diagram-tier":         {playwright("39-diagram-tier.spec.ts")},
	"wb:directory-home":       {playwright("37-directory-home.spec.ts")},
	// payoff-quote-portal (Task 2.1's fully-vetted showcase draft), not the
	// rudimentary DB_* draft-boards family — the same /b/<branch>/ routing
	// capability, proven on genuinely showcase-bar content instead.
	"wb:draft-boards": {playwright("40-showcase-draft.spec.ts")},
	"wb:dex":          {playwright("16-dex-v2.spec.ts")},
	"wb:dex-by-story": {playwright("18-dex-by-story.spec.ts")},
	"wb:presentation": {playwright("06-presentation.spec.ts")},
	"wb:ref-peek":     {playwright("25-board-ref-peek.spec.ts")},
	// wb:board-review-mode, wb:obligation-wall, wb:wall-badges,
	// wb:wall-receipts, wb:evidence-slot, wb:derivation-drawer,
	// wb:disclosures are all real gaps — see the report.
}

// workbenchSurfaces is the one hand-maintained axis (spec §10 mitigation).
var workbenchSurfaces = []string{
	"board", "board-review-mode", "board-scoping-canvas", "obligation-wall",
	"wall-badges", "wall-receipts", "evidence-slot", "diagram-editor",
	"diagram-tier", "derivation-drawer", "directory-home", "draft-boards",
	"dex", "dex-by-story", "disclosures", "presentation", "ref-peek",
}

// cliVerbs parses cmd/verdi/dispatch.go with go/parser and returns every
// verb name whose verbPhase entry is greater than zero — dispatch.go's own
// convention for "a real, dispatched v1 verb" (phase 0 means "recognized
// but explicitly out of v0 scope", PLAN.md §5: waivers, verify-artifact) —
// plus "lint", which dispatch.go's run() special-cases before verbPhase is
// even consulted (its own comment: "No verb's semantics live here" is true
// of every verb except lint, dispatched first).
//
// Every unexpected shape fails the test outright with a clear message
// (never silently missing a verb): this is the robustness the task
// requires of the walk, because a silent miscount here would let a real
// capability regression through the gate undetected.
func cliVerbs(t *testing.T) []string {
	t.Helper()

	path := filepath.Join(verdiRepoRoot, "cmd", "verdi", "dispatch.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("cliVerbs: parsing %s: %v", path, err)
	}

	var lit *ast.CompositeLit
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name.Name != "verbPhase" {
					continue
				}
				if i >= len(vs.Values) {
					t.Fatalf("cliVerbs: verbPhase declared with no value in %s (dispatch.go shape changed)", path)
				}
				cl, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					t.Fatalf("cliVerbs: verbPhase is not a composite literal in %s (dispatch.go shape changed)", path)
				}
				lit = cl
			}
		}
	}
	if lit == nil {
		t.Fatalf("cliVerbs: verbPhase map literal not found in %s (dispatch.go shape changed)", path)
	}

	var verbs []string
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			t.Fatalf("cliVerbs: verbPhase entry %#v is not a key:value pair (dispatch.go shape changed)", elt)
		}

		keyLit, ok := kv.Key.(*ast.BasicLit)
		if !ok || keyLit.Kind != token.STRING {
			t.Fatalf("cliVerbs: verbPhase key %#v is not a string literal (dispatch.go shape changed)", kv.Key)
		}
		key, err := strconv.Unquote(keyLit.Value)
		if err != nil {
			t.Fatalf("cliVerbs: unquoting verbPhase key %s: %v", keyLit.Value, err)
		}

		valLit, ok := kv.Value.(*ast.BasicLit)
		if !ok || valLit.Kind != token.INT {
			t.Fatalf("cliVerbs: verbPhase value for %q (%#v) is not an int literal (dispatch.go shape changed)", key, kv.Value)
		}
		n, err := strconv.Atoi(valLit.Value)
		if err != nil {
			t.Fatalf("cliVerbs: parsing verbPhase value for %q: %v", key, err)
		}

		if n > 0 {
			verbs = append(verbs, key)
		}
	}
	if len(verbs) == 0 {
		t.Fatalf("cliVerbs: found the verbPhase literal but extracted zero phase>0 verbs from %s (dispatch.go shape changed)", path)
	}

	verbs = append(verbs, "lint")
	return verbs
}

// mcpToolDef is one tools/list entry's shape this test needs.
type mcpToolDef struct {
	Name string `json:"name"`
}

// mcpTools drives the real, live server's tools/list over the exact NDJSON
// wire framing a client would (mcpserve.ServeConn), exactly as
// internal/specalign/mcptools_test.go's listMCPTools does — an
// inventory-from-the-wire check, not a unit test of tooldefs.go. It is
// pointed at verdiRepoRoot (this repo's own self-hosted .verdi store, the
// only root mcpserve.NewServer needs to construct); the tools/list
// response itself is process-global and does not depend on which store
// backs it, so this differs from specalign's own copy only in package.
func mcpTools(t *testing.T) []string {
	t.Helper()

	srv := mcpserve.NewServer(verdiRepoRoot)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), strings.NewReader(req), &out, srv); err != nil {
		t.Fatalf("mcpTools: ServeConn(tools/list): %v", err)
	}

	var resp struct {
		Result struct {
			Tools []mcpToolDef `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("mcpTools: decoding tools/list response %q: %v", out.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("mcpTools: tools/list returned a JSON-RPC error: %s", resp.Error.Message)
	}

	var names []string
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	if len(names) == 0 {
		t.Fatalf("mcpTools: tools/list returned zero tools (mcpserve shape changed or server misconstructed)")
	}
	return names
}

// TestShowcaseCoverage is the showcase-coverage gate itself: see this
// file's package-level doc comment for the full rationale. It is expected
// to be RED right now (30 of 43 capabilities unmapped) — Task 3.4's punch
// list, not a defect in this test.
func TestShowcaseCoverage(t *testing.T) {
	e2eDir := filepath.Join(verdiRepoRoot, "e2e", "tests")
	if _, err := os.Stat(e2eDir); err != nil {
		t.Skipf("DISCLOSURE: e2e/tests is entirely absent (%v); cannot verify Playwright-backed showcase coverage from this checkout", err)
	}

	capabilities := map[string]bool{}
	for _, v := range cliVerbs(t) {
		capabilities["cli:"+v] = true
	}
	for _, tool := range mcpTools(t) {
		capabilities["mcp:"+tool] = true
	}
	for _, s := range workbenchSurfaces {
		capabilities["wb:"+s] = true
	}

	var missing []string
	for cap := range capabilities {
		if _, ok := showcaseCoverage[cap]; !ok {
			missing = append(missing, cap)
		}
	}
	sort.Strings(missing)
	for _, cap := range missing {
		t.Errorf("showcase-coverage gap: %s has no showcase-backed e2e evidence", cap)
	}

	var extra []string
	for cap := range showcaseCoverage {
		if !capabilities[cap] {
			extra = append(extra, cap)
		}
	}
	sort.Strings(extra)
	for _, cap := range extra {
		t.Errorf("showcase-coverage gap: %s is mapped in showcaseCoverage but names no enumerated capability (stale or renamed entry)", cap)
	}

	// Every mapping present is checked for real: the file must exist under
	// the repo root and its bytes must match its own marker regexp. A
	// mapping that fails this is worse than an honest gap — it is a false
	// claim of coverage — so it fails loudly here too.
	var mappedCaps []string
	for cap := range showcaseCoverage {
		mappedCaps = append(mappedCaps, cap)
	}
	sort.Strings(mappedCaps)
	for _, cap := range mappedCaps {
		evidences := showcaseCoverage[cap]
		if len(evidences) == 0 {
			t.Errorf("showcase-coverage gap: %s maps to zero evidence entries", cap)
			continue
		}
		for _, ev := range evidences {
			path := filepath.Join(verdiRepoRoot, ev.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: evidence file %s does not exist under the repo root: %v", cap, ev.file, err)
				continue
			}
			re, err := regexp.Compile(ev.marker)
			if err != nil {
				t.Fatalf("%s: evidence marker %q does not compile as a regexp: %v", cap, ev.marker, err)
			}
			if !re.Match(data) {
				t.Errorf("%s: evidence file %s does not match its marker %q", cap, ev.file, ev.marker)
			}
		}
	}
}
