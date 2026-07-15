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
// Task 3.2 committed this test RED (30 gaps: most CLI verbs, all nine MCP
// tools, seven workbench surfaces — .superpowers/sdd/task-3.2-report.md
// has the full original gap list and, for every candidate file rejected
// along the way, why). Task 3.4 (.superpowers/sdd/task-3.4-report.md)
// closed every one of them with genuine showcase-backed evidence — two new
// Go e2e test files (internal/showcasealign/{mcp_showcase_test.go,
// cli_showcase_test.go}) for the CLI and MCP axes, re-pointed or
// content-upgraded Playwright specs for the seven workbench gaps, and one
// documented, reversible exclusion (cli:feature, PLAN-V1.md ledger
// R4-I-54) — never by faking a mapping or weakening this test's own two
// checks (capability-set equality, and every mapped file's bytes actually
// matching its marker). This test is expected to be GREEN as of Task 3.4;
// a regression here means a real capability lost its showcase backing,
// not a defect in the test.
//
// The three gap kinds Task 3.2 recorded (all now closed; kept here as
// history/context for how each was closed):
//
//  1. No candidate evidence file existed yet (most CLI verbs, all MCP
//     tools, wall-badges, derivation-drawer, disclosures) — closed by a
//     new, genuine test/spec per Task 3.4.
//  2. A candidate Playwright spec existed and carried the SHOWCASE. marker,
//     but ONLY via a SHOWCASE-classified fixture whose provisioner prose
//     was still rudimentary (board-review-mode, obligation-wall,
//     evidence-slot; wall-receipts transitively) — closed by re-pointing
//     at, or upgrading, a genuinely-vetted fixture (task-3.4-report.md
//     records which for each).
//  3. A candidate Go test file matched the /examples\/showcase/ marker
//     textually but its own doc comment disclosed it deliberately does
//     NOT exercise the showcase corpus (cli:rollup:
//     cmd/verdi/rollup_test.go; the whole MCP axis:
//     internal/mcpserve/fixture_test.go) — closed by a NEW test that
//     genuinely does exercise real showcase content instead of relying on
//     the disclaimed scratch fixture.
//
// Full reasoning for every mapped and every gapped (now closed)
// capability lives in .superpowers/sdd/task-3.{2,4}-report.md (not
// duplicated here at length, to keep this file's signal — the map itself
// — legible).
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

// needsPlaywrightDir reports whether this evidence is a Playwright spec under
// e2e/tests/ (as produced by the playwright(...) constructor). Such evidence
// is verifiable only when e2e/tests is present; Go-backed evidence (goE2E:
// repo-relative *.go files marked examples/showcase) needs no Playwright
// specs and is always checkable. TestShowcaseCoverage uses this to scope its
// disclosure PRECISELY: an absent e2e/tests disables ONLY the Playwright-
// dependent marker checks (and the wb axis), never the Go-backed CLI and MCP
// axes — the exact over-broad-skip defect this method exists to fix.
func (e coverageEvidence) needsPlaywrightDir() bool {
	return strings.HasPrefix(e.file, "e2e/tests/")
}

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
// Every other CLI verb (design, accept, build, align, serve, mcp, rollup,
// close, dex, gc, gate, board, audit) and every MCP tool are mapped below
// via Task 3.4's two new e2e test files — see cli_showcase_test.go's and
// mcp_showcase_test.go's own package doc comments for exactly what real
// showcase content and assertion backs each one. "feature" is deliberately
// excluded from the enumerated set (featureVerbExcluded's doc comment,
// PLAN-V1.md ledger R4-I-54) rather than mapped.
var showcaseCoverage = map[string][]coverageEvidence{
	// --- CLI verbs (verbPhase>0 entries, plus "lint") ---
	"cli:lint":   {goE2E("internal/showcasealign/lintclean_test.go")},
	"cli:matrix": {goE2E("cmd/verdi/matrix_test.go")},
	"cli:sync":   {goE2E("cmd/verdi/sync_test.go")},

	// Task 3.4: cli_showcase_test.go drives each of these against a real
	// provisioned examples/showcase store via runBinary (the exact
	// build-then-exec discipline cli:lint/cli:matrix/cli:sync already use)
	// — see that file's own package-level doc comment for exactly which
	// real showcase content and deterministic outcome each verb proves.
	"cli:audit":  {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:dex":    {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:board":  {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:rollup": {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:design": {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:accept": {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:build":  {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:align":  {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:gate":   {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:close":  {goE2E("internal/showcasealign/cli_showcase_test.go")},
	"cli:gc":     {goE2E("internal/showcasealign/cli_showcase_test.go")},

	// cli:serve: `cmd/e2eharness/main.go` launches the real `verdi serve
	// --http <addr>` subprocess every Playwright spec in the suite runs
	// against (never a fake/stub server) — so any SHOWCASE.-marked spec
	// already proves serve's own real startup+request path against the
	// provisioned showcase store. Reuses wb:board's own evidence file
	// rather than duplicating a second, redundant server-startup proof.
	"cli:serve": {playwright("10-board-projection.spec.ts")},
	// cli:mcp: `verdi mcp` is byte-for-byte ServeConn piped over stdio
	// (mcpserve/wire.go's own doc comment: "the shim degenerates to a
	// pipe") — mcp_showcase_test.go drives that exact ServeConn/NewServer
	// pair against a provisioned showcase store, tool by tool.
	"cli:mcp": {goE2E("internal/showcasealign/mcp_showcase_test.go")},

	// --- MCP tools: Task 3.4's mcp_showcase_test.go drives each of the
	// nine live tools (via mcpserve.NewServer + mcpserve.ServeConn, the
	// real wire protocol) against a provisioned examples/showcase store,
	// asserting a genuine showcase-derived result per tool — see that
	// file's own doc comment for the specific real content and assertion
	// behind each one. ---
	"mcp:search_artifacts":   {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:get_artifact":       {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:get_links":          {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:get_matrix":         {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:get_context_bundle": {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:list_annotations":   {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:list_tasks":         {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:get_board":          {goE2E("internal/showcasealign/mcp_showcase_test.go")},
	"mcp:add_annotation":     {goE2E("internal/showcasealign/mcp_showcase_test.go")},

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

	// Task 3.4's seven workbench closures. Per-surface disposition (full
	// reasoning: task-3.4-report.md):
	//
	//   - board-review-mode / obligation-wall / evidence-slot: each
	//     surface's only candidate spec already carried the SHOWCASE.
	//     marker on a SHOWCASE-classified fixture (Task 2.2) — the gap was
	//     the fixture's own rudimentary provisioner prose (fixtures.ts's
	//     own flagged NOTE), never the marker or the assertions. Fixed by
	//     UPGRADING cmd/e2eharness/provision_board.go's reviewSpec/
	//     replaySpec/slotWallSpec body prose to the payoff-quote-portal bar
	//     (production-quality, canon-consistent) — re-pointing at a
	//     different spec was unnecessary since the existing one was
	//     already the right surface on the right fixture.
	//   - wall-receipts: internal/workbench/badges.go's own doc comment
	//     names it "the wall-receipts story (evidence-slot,
	//     case-file-flags)" — no dedicated page of its own. Mapped to the
	//     now-vetted evidence-slot spec, its closest named sibling.
	//   - wall-badges / derivation-drawer: 37-board-wall-badges.spec.ts and
	//     38-derivation-drawer.spec.ts stay EDGE-only on purpose — a
	//     deliberately-authored VL-003/VL-006 lint VIOLATION is not
	//     showcase material by definition, so upgrading their prose would
	//     misrepresent them. Closed instead by a NEW spec
	//     (41-showcase-ladder-badge.spec.ts) proving the SAME wall-badge +
	//     derivation-drawer contract on a genuinely real, committed
	//     showcase fact: borrower-update-mobile's own accepted-deviation
	//     scar computes a real ladder:spec-stale badge on its board, opened
	//     to reveal its real derivation record.
	//   - disclosures: 19-disclosures.spec.ts's content was already
	//     genuine; it simply never spelled `SHOWCASE.` (task-3.2-report.md's
	//     "marker-mechanical, not a content problem"). Fixed by adding
	//     SHOWCASE.FORGE_KIND (examples/showcase/.verdi/verdi.yaml's own
	//     committed `forge: gitlab` value) and routing the disclosure-text
	//     assertion through it instead of a bare "gitlab" literal.
	"wb:board-review-mode": {playwright("15-board-review-mode.spec.ts")},
	"wb:obligation-wall":   {playwright("36-board-obligation-wall.spec.ts")},
	"wb:evidence-slot":     {playwright("38-board-evidence-slot.spec.ts")},
	"wb:wall-receipts":     {playwright("38-board-evidence-slot.spec.ts")},
	"wb:wall-badges":       {playwright("41-showcase-ladder-badge.spec.ts")},
	"wb:derivation-drawer": {playwright("41-showcase-ladder-badge.spec.ts")},
	"wb:disclosures":       {playwright("19-disclosures.spec.ts")},
}

// workbenchSurfaces is the one hand-maintained axis (spec §10 mitigation).
var workbenchSurfaces = []string{
	"board", "board-review-mode", "board-scoping-canvas", "obligation-wall",
	"wall-badges", "wall-receipts", "evidence-slot", "diagram-editor",
	"diagram-tier", "derivation-drawer", "directory-home", "draft-boards",
	"dex", "dex-by-story", "disclosures", "presentation", "ref-peek",
}

// featureVerbExcluded is "feature", REMOVED from the enumerated CLI
// capability set (Task 3.4, PLAN-V1.md ledger R4-I-54): dispatch.go's own
// comment marks it "R4-I-6: deprecation alias for build", and its
// dispatch entry routes through runFeatureStart, which shares runBuildStart
// with `build` — every precondition and side effect — differing only by a
// printed R4-I-6 deprecation notice on stderr (cmd/verdi/feature.go). There
// is no second code path for this verb to showcase-back: cli:build's mapping
// (cli_showcase_test.go) already exercises the one build target both names
// share. Mapping "cli:feature" to the same evidence file as "cli:build"
// would be a technically-satisfiable but hollow entry (the marker regexp
// would match, but the file proves nothing "feature"-specific beyond the
// shared build path) — smallest reversible choice per
// CLAUDE.md's provenance discipline: exclude the alias from the
// enumerated set instead of faking a distinct mapping. Reversible the
// moment `feature` stops being a pure alias (its own removal from
// verbPhase, or a divergent implementation, would need this line
// removed and a real mapping added back).
const featureVerbExcluded = "feature"

// cliVerbs parses cmd/verdi/dispatch.go with go/parser and returns every
// verb name whose verbPhase entry is greater than zero — dispatch.go's own
// convention for "a real, dispatched v1 verb" (phase 0 means "recognized
// but explicitly out of v0 scope", PLAN.md §5: waivers, verify-artifact) —
// plus "lint", which dispatch.go's run() special-cases before verbPhase is
// even consulted (its own comment: "No verb's semantics live here" is true
// of every verb except lint, dispatched first). "feature" is filtered back
// out immediately below — see featureVerbExcluded's doc comment.
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

	filtered := verbs[:0]
	for _, v := range verbs {
		if v == featureVerbExcluded {
			continue
		}
		filtered = append(filtered, v)
	}
	return filtered
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

// coverageResult is what computeCoverageGaps found. The first three slices are
// the gate's three failure classes (the caller turns each into a t.Errorf);
// disclosedPlaywright is the Playwright-backed evidence that could not be
// checked because e2e/tests is absent — disclosed-as-unproven, never a silent
// pass. All four are deterministic (sorted).
type coverageResult struct {
	missing             []string // enumerated capabilities with no inventory entry
	extra               []string // inventory keys naming no enumerated capability (stale/renamed)
	markerMismatches    []string // mapped evidence that is absent, empty, or whose bytes miss its marker
	disclosedPlaywright []string // "<cap> -> <file>" for Playwright evidence not checkable (e2e/tests absent)
}

// computeCoverageGaps is the pure heart of the showcase-coverage gate,
// extracted from TestShowcaseCoverage so its FAILURE modes are directly
// unit-testable (TestShowcaseCoverage_DetectsGaps). A gate whose own failure
// path is never exercised is exactly the silent pass this whole story exists
// to rule out, so the check that names missing/stale/mismatched capabilities
// must itself carry a negative-path test (CLAUDE.md: every function needs
// happy AND negative paths). This function takes no *testing.T and never fails
// a test itself: it reports, and the caller decides which slices are errors
// (all three of missing/extra/markerMismatches) and which is a disclosure.
//
// e2ePresent scopes the disclosure PRECISELY. Go-backed evidence (a repo .go
// file marked examples/showcase) is ALWAYS checked, so a broken cli:/mcp:
// mapping is caught even from an e2e-less checkout. Playwright evidence (a spec
// under e2e/tests/) is checkable only when that dir is present; when it is
// absent such evidence is routed into disclosedPlaywright, and wb: inventory
// keys are exempt from `extra` (the workbench axis is deliberately not
// enumerated in that path). Note this means Playwright-backed evidence is
// disclosed regardless of which axis names it: the workbench axis, and the one
// cross-axis exception cli:serve, are BOTH disclosed-unproven when e2e/tests is
// absent — the CLI axis is therefore not "fully" enforced in that path, only
// its Go-backed verbs are.
func computeCoverageGaps(capabilities map[string]bool, inventory map[string][]coverageEvidence, repoRoot string, e2ePresent bool) coverageResult {
	var res coverageResult

	// Gap direction: every enumerated capability must be mapped.
	for cap := range capabilities {
		if _, ok := inventory[cap]; !ok {
			res.missing = append(res.missing, cap)
		}
	}
	sort.Strings(res.missing)

	// Stale direction: every mapped key must name an enumerated capability. A
	// wb: entry when e2e/tests is absent is disclosed-unproven (its axis was
	// deliberately not enumerated), NOT stale — so exempt it; any cli:/mcp: key
	// that names no enumerated capability is still a hard gap, so a stale or
	// renamed Go-axis mapping cannot hide behind an absent dir.
	for cap := range inventory {
		if capabilities[cap] {
			continue
		}
		if !e2ePresent && strings.HasPrefix(cap, "wb:") {
			continue
		}
		res.extra = append(res.extra, cap)
	}
	sort.Strings(res.extra)

	// Marker direction: every mapping is checked for real — the file must exist
	// under the repo root and its bytes must match its own marker regexp. A
	// mapping that fails this is worse than an honest gap (it is a false claim
	// of coverage), so it is reported too. Go-backed evidence is ALWAYS checked;
	// Playwright evidence (under e2e/tests/) is checked only when that dir is
	// present and otherwise disclosed. cli:serve is the lone cli: mapping backed
	// by a Playwright spec, so it is the single CLI verb disclosed (not checked)
	// in the e2e-absent path — every other CLI verb, and every MCP tool, is
	// Go-backed and still checked here.
	var mapped []string
	for cap := range inventory {
		mapped = append(mapped, cap)
	}
	sort.Strings(mapped)
	for _, cap := range mapped {
		evidences := inventory[cap]
		if len(evidences) == 0 {
			res.markerMismatches = append(res.markerMismatches, cap+": maps to zero evidence entries")
			continue
		}
		for _, ev := range evidences {
			if !e2ePresent && ev.needsPlaywrightDir() {
				res.disclosedPlaywright = append(res.disclosedPlaywright, cap+" -> "+ev.file)
				continue
			}
			path := filepath.Join(repoRoot, ev.file)
			data, err := os.ReadFile(path)
			if err != nil {
				res.markerMismatches = append(res.markerMismatches, cap+": evidence file "+ev.file+" does not exist under the repo root")
				continue
			}
			re, err := regexp.Compile(ev.marker)
			if err != nil {
				res.markerMismatches = append(res.markerMismatches, cap+": evidence marker "+ev.marker+" does not compile as a regexp")
				continue
			}
			if !re.Match(data) {
				res.markerMismatches = append(res.markerMismatches, cap+": evidence file "+ev.file+" does not match its marker "+ev.marker)
			}
		}
	}
	sort.Strings(res.disclosedPlaywright)

	return res
}

// e2eTestsPresent reports whether e2e/tests exists under the repo root — the
// single detector the gate and its red-direction proof both use to decide
// whether the Playwright-backed axes (workbench, and the lone cross-axis
// cli:serve) are checkable or disclosed-as-unproven.
func e2eTestsPresent() bool {
	_, err := os.Stat(filepath.Join(verdiRepoRoot, "e2e", "tests"))
	return err == nil
}

// realCapabilities enumerates every shipped capability along all three axes
// from their REAL sources — cliVerbs' go/parser walk of dispatch.go's verbPhase
// literal, mcpTools' live tools/list against internal/mcpserve, and the
// hand-listed workbenchSurfaces — exactly as the green gate consumes them. The
// workbench axis is Playwright-only, so it is enumerated only when e2e/tests is
// present; otherwise its keys would be spuriously flagged stale and its gaps
// could not be proven either way, so it is disclosed-unproven and left OUT of
// the set rather than silently treated as covered. Shared by TestShowcaseCoverage
// (the green gate) and TestShowcaseCoverage_RealEnumerationDetectsGaps (the red
// proof), so both bind to one enumeration, not two divergent copies.
func realCapabilities(t *testing.T, e2ePresent bool) map[string]bool {
	t.Helper()
	capabilities := map[string]bool{}
	for _, v := range cliVerbs(t) {
		capabilities["cli:"+v] = true
	}
	for _, tool := range mcpTools(t) {
		capabilities["mcp:"+tool] = true
	}
	if e2ePresent {
		for _, s := range workbenchSurfaces {
			capabilities["wb:"+s] = true
		}
	}
	return capabilities
}

// TestShowcaseCoverage is the showcase-coverage gate itself: see this
// file's package-level doc comment for the full rationale. It is GREEN as of
// Task 3.4 (every enumerated capability mapped and every mapped file matching
// its marker); a regression here means a real capability lost its showcase
// backing, not a defect in this test. When e2e/tests is absent it still fully
// enforces every Go-backed (examples/showcase) CLI and MCP evidence file —
// every MCP tool and every CLI verb except cli:serve — and discloses as
// unproven only what genuinely depends on Playwright: the workbench axis plus
// the lone cross-axis Playwright-backed CLI evidence, cli:serve (never a silent
// pass). It deliberately does NOT claim the CLI axis is fully enforced in that
// path: cli:serve's sole evidence is a Playwright spec, so that one verb is
// disclosed-unproven exactly like the workbench axis.
func TestShowcaseCoverage(t *testing.T) {
	// The MCP axis and all-but-one of the CLI axis are backed by Go test files
	// in this repo (marker examples/showcase); the workbench axis and the lone
	// cross-axis exception cli:serve (a Playwright spec — the HTTP server every
	// spec runs against) live under e2e/tests/. So a checkout missing e2e/tests
	// can — and MUST — still fully enforce every Go-backed evidence file (every
	// MCP tool, and every CLI verb except cli:serve); only the genuinely
	// Playwright-dependent checks — the workbench axis and cli:serve — are
	// disclosed-as-unproven, never claimed enforced. Three-valued honesty: a
	// loud t.Log, never a silent pass, and never suppressing a real gap in the
	// Go-backed evidence. The pre-fix behavior — a blanket t.Skip on a missing
	// e2e/tests, taken BEFORE the CLI/MCP axes were even computed — disabled ALL
	// THREE axes, far wider than the disclosure text claimed; that over-broad
	// skip was the defect. The residual precision fix is to stop calling the CLI
	// axis "fully" enforced in this path when cli:serve is disclosed, not checked.
	e2ePresent := e2eTestsPresent()
	if !e2ePresent {
		t.Logf("DISCLOSURE: e2e/tests is absent; the workbench axis and every Playwright-backed evidence marker — including cli:serve, whose sole evidence is a Playwright spec — are DISCLOSED-AS-UNPROVEN from this checkout. Every Go-backed (examples/showcase) CLI and MCP evidence file is still fully enforced below: that is every MCP tool and every CLI verb except cli:serve, NOT the CLI axis in full.")
	}

	// realCapabilities runs the same three-axis enumeration the red-direction
	// proof (TestShowcaseCoverage_RealEnumerationDetectsGaps) drives, so the
	// green gate and the red proof bind to ONE enumeration, not two.
	capabilities := realCapabilities(t, e2ePresent)

	// The pure check does all three directions (gap, stale, marker) and the
	// disclosure scoping; this test only enumerates the real capabilities,
	// turns each returned gap into a t.Errorf, and discloses loudly. Its own
	// failure path is proven separately by TestShowcaseCoverage_DetectsGaps.
	res := computeCoverageGaps(capabilities, showcaseCoverage, verdiRepoRoot, e2ePresent)
	for _, cap := range res.missing {
		t.Errorf("showcase-coverage gap: %s has no showcase-backed e2e evidence", cap)
	}
	for _, cap := range res.extra {
		t.Errorf("showcase-coverage gap: %s is mapped in showcaseCoverage but names no enumerated capability (stale or renamed entry)", cap)
	}
	for _, mm := range res.markerMismatches {
		t.Errorf("showcase-coverage marker mismatch: %s", mm)
	}
	if len(res.disclosedPlaywright) > 0 {
		t.Logf("DISCLOSURE: e2e/tests absent — %d Playwright-backed evidence marker(s) UNPROVEN from this checkout (NOT a pass): %s",
			len(res.disclosedPlaywright), strings.Join(res.disclosedPlaywright, ", "))
	}
}

// detectsGapsCase is one deliberately-broken-inventory scenario for the gate's
// negative-path proof. Hoisted to package scope (with detectsGapsCases below)
// so the RED proof lives in a table a SECOND test —
// TestShowcaseCoverage_DetectsGapsCoversAllClasses — can inspect and re-drive
// at ROW granularity. That closes the blind spot the Makefile coverage-guard
// cannot reach on its own: the make guard proves the driver test RAN (its
// `--- PASS:` line), but not that its table still exercises every gap class, so
// deleting the one row that makes an enforcement branch load-bearing would keep
// the function passing and `make verify` green. The comprehensiveness test
// fails when a class-critical row is deleted; both tests are in the Makefile's
// `required` list so neither can be vacuously removed.
type detectsGapsCase struct {
	name         string
	caps         map[string]bool
	inv          map[string][]coverageEvidence
	e2ePresent   bool
	wantMissing  []string   // exact sorted res.missing
	wantExtra    []string   // exact sorted res.extra
	wantMismatch [][]string // one inner slice per expected mismatch entry (index-aligned, sorted); each substring must appear in that entry
	wantDisclose []string   // each substring must appear in some res.disclosedPlaywright entry
}

// detectsGapsCases is the committed table of negative-path scenarios. It is
// driven by TestShowcaseCoverage_DetectsGaps (asserts each case's exact output)
// and re-driven by TestShowcaseCoverage_DetectsGapsCoversAllClasses (asserts
// the table still covers every gap class plus a case-count floor).
func detectsGapsCases() []detectsGapsCase {
	// A stable known-good Go-backed evidence: this very file always contains
	// the examples/showcase marker (goE2E's regexp literal is defined here), so
	// it is a valid mapping to pair with each deliberate break without itself
	// contributing a mismatch.
	good := goE2E("internal/showcasealign/coverage_test.go")

	return []detectsGapsCase{
		{
			name:        "gap: enumerated capability with no inventory entry is named in missing",
			caps:        map[string]bool{"cli:phantom": true, "mcp:real": true},
			inv:         map[string][]coverageEvidence{"mcp:real": {good}},
			e2ePresent:  true,
			wantMissing: []string{"cli:phantom"},
		},
		{
			name:       "stale: inventory key naming no enumerated capability is named in extra",
			caps:       map[string]bool{"mcp:real": true},
			inv:        map[string][]coverageEvidence{"mcp:real": {good}, "cli:ghostkey": {good}},
			e2ePresent: true,
			wantExtra:  []string{"cli:ghostkey"},
		},
		{
			name:         "marker mismatch: mapped file whose bytes miss the marker is named",
			caps:         map[string]bool{"cli:badmarker": true},
			inv:          map[string][]coverageEvidence{"cli:badmarker": {{file: "go.mod", marker: "no-such-marker-string-in-go-mod"}}},
			e2ePresent:   true,
			wantMismatch: [][]string{{"cli:badmarker", "go.mod"}},
		},
		{
			name:         "marker mismatch: mapped evidence file that does not exist is named",
			caps:         map[string]bool{"cli:nofile": true},
			inv:          map[string][]coverageEvidence{"cli:nofile": {{file: "internal/showcasealign/does-not-exist.go", marker: "examples/showcase"}}},
			e2ePresent:   true,
			wantMismatch: [][]string{{"cli:nofile", "does-not-exist.go"}},
		},
		{
			name:         "marker mismatch: capability mapped to zero evidence entries is named",
			caps:         map[string]bool{"cli:noevidence": true},
			inv:          map[string][]coverageEvidence{"cli:noevidence": {}},
			e2ePresent:   true,
			wantMismatch: [][]string{{"cli:noevidence", "zero evidence"}},
		},
		{
			// The fourth markerMismatch producer, previously unexercised: an
			// evidence file that EXISTS (go.mod) but whose marker is not a valid
			// regexp ("[" fails regexp.Compile). computeCoverageGaps reads the
			// file first, then compiles the marker, so this reaches the
			// "does not compile as a regexp" branch specifically — the one
			// failure class the table did not yet cover. Without this row that
			// branch could be deleted or inverted with every gate signal green.
			name:         "marker mismatch: mapped file with a non-compiling regexp marker is named",
			caps:         map[string]bool{"cli:badregexp": true},
			inv:          map[string][]coverageEvidence{"cli:badregexp": {{file: "go.mod", marker: "["}}},
			e2ePresent:   true,
			wantMismatch: [][]string{{"cli:badregexp", "does not compile as a regexp"}},
		},
		{
			// The A2 claim made executable: a CLI verb backed only by a
			// Playwright spec is disclosed-unproven (NOT enforced) when
			// e2e/tests is absent — exactly like the workbench axis.
			name:         "e2e-absent: cli:serve (Playwright-backed) is DISCLOSED, not enforced",
			caps:         map[string]bool{"cli:serve": true, "cli:real": true},
			inv:          map[string][]coverageEvidence{"cli:serve": {playwright("10-board-projection.spec.ts")}, "cli:real": {good}},
			e2ePresent:   false,
			wantDisclose: []string{"cli:serve"},
		},
		{
			name:         "e2e-absent: wb: key is exempt from stale and its Playwright evidence disclosed",
			caps:         map[string]bool{"cli:real": true},
			inv:          map[string][]coverageEvidence{"cli:real": {good}, "wb:board": {playwright("10-board-projection.spec.ts")}},
			e2ePresent:   false,
			wantDisclose: []string{"wb:board"},
		},
		{
			// The ENFORCEMENT half of the e2e-absent disclosure contract (the
			// two cases above prove only its DISCLOSURE half — that Playwright
			// evidence is disclosed, not checked, when e2e/tests is gone). A
			// BROKEN Go-backed (goE2E) mapping — here a nonexistent .go file —
			// is STILL reported as a markerMismatch with e2ePresent:false, NEVER
			// routed into disclosedPlaywright: Go-backed evidence is ALWAYS
			// checked, so a broken cli:/mcp: mapping bites even from an e2e-less
			// checkout (computeCoverageGaps' own contract, and 3ffb6f8's whole
			// purpose). This is what makes the discriminating condition
			// `if !e2ePresent && ev.needsPlaywrightDir()` load-bearing: mutate it
			// to a bare `if !e2ePresent` (3ffb6f8's over-broad skip) and this
			// case FAILS — the broken Go file would be silently disclosed instead
			// of enforced, exactly the honesty gap the mutation reintroduces.
			name:         "e2e-absent: BROKEN Go-backed mapping is STILL enforced, not disclosed",
			caps:         map[string]bool{"cli:real": true},
			inv:          map[string][]coverageEvidence{"cli:real": {goE2E("internal/showcasealign/does-not-exist.go")}},
			e2ePresent:   false,
			wantMismatch: [][]string{{"cli:real", "does-not-exist.go"}},
		},
		{
			name:       "clean: a valid inventory yields no gaps at all",
			caps:       map[string]bool{"cli:real": true, "mcp:real": true},
			inv:        map[string][]coverageEvidence{"cli:real": {good}, "mcp:real": {good}},
			e2ePresent: true,
		},
	}
}

// TestShowcaseCoverage_DetectsGaps is the gate's own negative-path proof: it
// feeds computeCoverageGaps deliberately-broken inventories (detectsGapsCases)
// and asserts it reports the RIGHT gap class naming the RIGHT capability. AC-1's
// behavioral claim is that the gate fails naming the exact missing capability
// when a mapping is removed or a capability is added unmapped; TestShowcaseCoverage
// proves the GREEN direction on the real inventory, this proves the pure check's
// RED direction is real and precise, and TestShowcaseCoverage_RealEnumerationDetectsGaps
// binds that RED direction to the REAL enumeration (dispatch.go's verbPhase walk
// and the live tools/list). Each case isolates one deliberate break so exactly
// one slice fires; the others are asserted empty, proving the signal is precise.
func TestShowcaseCoverage_DetectsGaps(t *testing.T) {
	for _, tc := range detectsGapsCases() {
		t.Run(tc.name, func(t *testing.T) {
			res := computeCoverageGaps(tc.caps, tc.inv, verdiRepoRoot, tc.e2ePresent)

			if !equalStrings(res.missing, tc.wantMissing) {
				t.Errorf("missing = %v, want %v", res.missing, tc.wantMissing)
			}
			if !equalStrings(res.extra, tc.wantExtra) {
				t.Errorf("extra = %v, want %v", res.extra, tc.wantExtra)
			}
			if len(res.markerMismatches) != len(tc.wantMismatch) {
				t.Fatalf("markerMismatches = %v, want %d entr(y|ies) %v", res.markerMismatches, len(tc.wantMismatch), tc.wantMismatch)
			}
			for i, subs := range tc.wantMismatch {
				for _, sub := range subs {
					if !strings.Contains(res.markerMismatches[i], sub) {
						t.Errorf("markerMismatches[%d] = %q, want it to contain %q", i, res.markerMismatches[i], sub)
					}
				}
			}
			if len(tc.wantDisclose) == 0 {
				if len(res.disclosedPlaywright) != 0 {
					t.Errorf("disclosedPlaywright = %v, want none", res.disclosedPlaywright)
				}
			} else {
				for _, sub := range tc.wantDisclose {
					if !anyContains(res.disclosedPlaywright, sub) {
						t.Errorf("disclosedPlaywright = %v, want an entry containing %q", res.disclosedPlaywright, sub)
					}
				}
			}
		})
	}
}

// equalStrings reports whether two string slices are element-wise equal,
// treating nil and empty as equal (a gap-free result yields a nil slice).
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// anyContains reports whether any element of ss contains sub.
func anyContains(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// TestShowcaseCoverage_DetectsGapsCoversAllClasses defends the RED proof at ROW
// granularity — the layer the Makefile coverage-guard cannot reach. That guard
// verifies TestShowcaseCoverage_DetectsGaps RAN (its `--- PASS:` line), but not
// that its table still exercises every gap class; deleting the single row that
// makes an enforcement branch load-bearing leaves the driver passing and
// `make verify` green while that guarantee silently reverts to prose. This test
// re-drives computeCoverageGaps over the SAME committed table (detectsGapsCases)
// and buckets each case by the gap class it actually PRODUCES — observed output,
// not a declared tag — then requires every class to be observed at least once,
// plus a case-count floor. Deleting any class-critical row makes it FAIL. It is
// a genuine second driver of the red direction (it runs the real check), so it
// is not a tautology; it is added to the Makefile `required` list so it cannot
// itself be vacuously deleted. It asserts a DIFFERENT property than the driver
// (table-level class coverage, which the driver never checks — the driver would
// happily pass a table missing an entire class), so the two are complementary,
// not redundant.
//
// Residual (inherent, disclosed): a name-granular make guard cannot verify
// arbitrary internal assertions, so gutting BOTH this test's body and the
// driver's to empty would still pass the guard. That irreducible core is why
// the coverage-guard finding is narrowed here, not erased: this closes the
// concrete row-deletion attack the finding names, defense-in-depth across two
// independently-required tests.
func TestShowcaseCoverage_DetectsGapsCoversAllClasses(t *testing.T) {
	cases := detectsGapsCases()

	// Floor: any single row deletion drops below this, a backstop against bulk
	// trimming that happens to preserve one-of-each-class. Raise it deliberately
	// when adding cases; a drop is a conscious edit, never a silent one.
	const floor = 10
	if len(cases) < floor {
		t.Fatalf("detectsGapsCases has %d case(s), want >= %d — rows were deleted; the RED proof must exercise every gap class", len(cases), floor)
	}

	var (
		sawMissing            bool // enumerated capability with no mapping
		sawExtra              bool // stale inventory key
		sawMismatchE2EPresent bool // markerMismatch with e2e/tests present
		sawMismatchE2EAbsent  bool // markerMismatch with e2e/tests ABSENT (Go-backed still enforced — the load-bearing row)
		sawDiscloseE2EAbsent  bool // Playwright disclosure with e2e/tests absent
		sawClean              bool // no gaps at all
	)
	for _, tc := range cases {
		res := computeCoverageGaps(tc.caps, tc.inv, verdiRepoRoot, tc.e2ePresent)
		hasMissing := len(res.missing) > 0
		hasExtra := len(res.extra) > 0
		hasMismatch := len(res.markerMismatches) > 0
		hasDisclose := len(res.disclosedPlaywright) > 0
		if hasMissing {
			sawMissing = true
		}
		if hasExtra {
			sawExtra = true
		}
		if hasMismatch && tc.e2ePresent {
			sawMismatchE2EPresent = true
		}
		if hasMismatch && !tc.e2ePresent {
			sawMismatchE2EAbsent = true
		}
		if hasDisclose && !tc.e2ePresent {
			sawDiscloseE2EAbsent = true
		}
		if !hasMissing && !hasExtra && !hasMismatch && !hasDisclose {
			sawClean = true
		}
	}

	for _, req := range []struct {
		ok   bool
		what string
	}{
		{sawMissing, "a case producing a MISSING gap (enumerated capability with no mapping)"},
		{sawExtra, "a case producing an EXTRA gap (stale/renamed inventory key)"},
		{sawMismatchE2EPresent, "a case producing a markerMismatch with e2e/tests present"},
		{sawMismatchE2EAbsent, "a case producing a markerMismatch with e2e/tests ABSENT (Go-backed evidence still enforced — the load-bearing row)"},
		{sawDiscloseE2EAbsent, "a case producing a Playwright DISCLOSURE with e2e/tests absent"},
		{sawClean, "a fully-clean case (no gaps at all)"},
	} {
		if !req.ok {
			t.Errorf("detectsGapsCases no longer includes %s — a class-critical row was deleted; the RED proof is incomplete", req.what)
		}
	}
}

// inventoryWithout returns a copy of the real showcaseCoverage inventory with
// one capability key removed — a deliberately-incomplete inventory for the
// real-enumeration red-direction proof below.
func inventoryWithout(cap string) map[string][]coverageEvidence {
	inv := make(map[string][]coverageEvidence, len(showcaseCoverage))
	for k, v := range showcaseCoverage {
		if k == cap {
			continue
		}
		inv[k] = v
	}
	return inv
}

// assertExactlyMissing asserts res names exactly the one expected missing
// capability and reports no extra/markerMismatch noise — the signal is precise:
// removing one real mapping (or adding one unmapped capability) surfaces exactly
// that capability, nothing else. The remaining real inventory is marker-clean
// (proven green by TestShowcaseCoverage), so extra and markerMismatches must be
// empty here regardless of whether e2e/tests is present.
func assertExactlyMissing(t *testing.T, res coverageResult, want string) {
	t.Helper()
	if !equalStrings(res.missing, []string{want}) {
		t.Errorf("missing = %v, want exactly [%s]", res.missing, want)
	}
	if len(res.extra) != 0 {
		t.Errorf("extra = %v, want none (a removed/added mapping must not create stale keys)", res.extra)
	}
	if len(res.markerMismatches) != 0 {
		t.Errorf("markerMismatches = %v, want none (the remaining real inventory must still match its markers)", res.markerMismatches)
	}
}

// TestShowcaseCoverage_RealEnumerationDetectsGaps closes AC-1's behavioral clause
// on the REAL enumeration, not a fabricated caps map: it drives computeCoverageGaps
// with the actual cliVerbs go/parser walk of dispatch.go's verbPhase and the
// actual live mcpTools tools/list — the exact enumeration TestShowcaseCoverage
// feeds the green gate — against a DELIBERATELY-INCOMPLETE inventory, and asserts
// the specific REAL capability name surfaces as a named gap. TestShowcaseCoverage
// proves the real enumeration is fully covered (green); TestShowcaseCoverage_DetectsGaps
// proves the pure check's red direction on synthetic inputs; this binds the two —
// a real capability from the real walk flowing into a named gap — so the
// enumeration→check seam itself carries a red-direction proof ("when a real
// capability's mapping is removed OR a new capability is added without one").
func TestShowcaseCoverage_RealEnumerationDetectsGaps(t *testing.T) {
	e2ePresent := e2eTestsPresent()
	caps := realCapabilities(t, e2ePresent)

	// Key the proof on real, Go-backed capabilities (checked in every checkout,
	// e2e/tests present or not). The guard makes a rename fail this proof LOUDLY
	// — the capability it names no longer exists — rather than silently exercise
	// nothing.
	for _, must := range []string{"cli:build", "mcp:get_artifact"} {
		if !caps[must] {
			t.Fatalf("real enumeration does not contain %q (renamed/removed capability?); this red-direction proof must key on a real capability", must)
		}
	}

	t.Run("real CLI capability whose mapping is removed is named (dispatch.go verbPhase walk)", func(t *testing.T) {
		const removed = "cli:build"
		res := computeCoverageGaps(caps, inventoryWithout(removed), verdiRepoRoot, e2ePresent)
		assertExactlyMissing(t, res, removed)
	})

	t.Run("real MCP capability whose mapping is removed is named (live tools/list)", func(t *testing.T) {
		const removed = "mcp:get_artifact"
		res := computeCoverageGaps(caps, inventoryWithout(removed), verdiRepoRoot, e2ePresent)
		assertExactlyMissing(t, res, removed)
	})

	t.Run("a newly-shipped capability added to the real enumeration without a mapping is named", func(t *testing.T) {
		// AC-1's "a new capability is added without one", modeled on the REAL
		// enumeration: layer one synthetic just-shipped capability on top of the
		// real caps, keep the complete real inventory, and assert it is the ONLY
		// thing named — proving the real inventory covers the entire real
		// enumeration today AND that one more unmapped capability is caught.
		const added = "cli:__newly_shipped_verb__"
		augmented := map[string]bool{added: true}
		for k, v := range caps {
			augmented[k] = v
		}
		res := computeCoverageGaps(augmented, showcaseCoverage, verdiRepoRoot, e2ePresent)
		assertExactlyMissing(t, res, added)
	})
}
