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
	e2eDir := filepath.Join(verdiRepoRoot, "e2e", "tests")
	e2ePresent := true
	if _, err := os.Stat(e2eDir); err != nil {
		e2ePresent = false
		t.Logf("DISCLOSURE: e2e/tests is absent (%v); the workbench axis and every Playwright-backed evidence marker — including cli:serve, whose sole evidence is a Playwright spec — are DISCLOSED-AS-UNPROVEN from this checkout. Every Go-backed (examples/showcase) CLI and MCP evidence file is still fully enforced below: that is every MCP tool and every CLI verb except cli:serve, NOT the CLI axis in full.", err)
	}

	capabilities := map[string]bool{}
	for _, v := range cliVerbs(t) {
		capabilities["cli:"+v] = true
	}
	for _, tool := range mcpTools(t) {
		capabilities["mcp:"+tool] = true
	}
	// The workbench axis is Playwright-only: enumerate it only when its specs
	// are present. Otherwise its map entries would be spuriously flagged as
	// stale "extra" entries below, and its gaps could not be proven either
	// way — so when e2e/tests is absent it is disclosed-unproven (above) and
	// deliberately left OUT of the enumerated set, never silently treated as
	// covered.
	if e2ePresent {
		for _, s := range workbenchSurfaces {
			capabilities["wb:"+s] = true
		}
	}

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

// TestShowcaseCoverage_DetectsGaps is the gate's own negative-path proof: it
// feeds computeCoverageGaps deliberately-broken inventories and asserts it
// reports the RIGHT gap class naming the RIGHT capability. AC-1's behavioral
// claim is that the gate fails naming the exact missing capability when a
// mapping is removed or a capability is added unmapped; TestShowcaseCoverage
// above proves the GREEN direction on the real inventory, and this proves the
// RED direction is real and precise — without it the gate's failure mode would
// be unexercised, itself a silent pass (CLAUDE.md: every function needs a
// negative-path test). Each case isolates one deliberate break so exactly one
// slice fires; the others are asserted empty, proving the signal is precise.
func TestShowcaseCoverage_DetectsGaps(t *testing.T) {
	// A stable known-good Go-backed evidence: this very file always contains
	// the examples/showcase marker (goE2E's regexp literal is defined here), so
	// it is a valid mapping to pair with each deliberate break without itself
	// contributing a mismatch.
	good := goE2E("internal/showcasealign/coverage_test.go")

	tests := []struct {
		name         string
		caps         map[string]bool
		inv          map[string][]coverageEvidence
		e2ePresent   bool
		wantMissing  []string   // exact sorted res.missing
		wantExtra    []string   // exact sorted res.extra
		wantMismatch [][]string // one inner slice per expected mismatch entry (index-aligned, sorted); each substring must appear in that entry
		wantDisclose []string   // each substring must appear in some res.disclosedPlaywright entry
	}{
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

	for _, tc := range tests {
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
