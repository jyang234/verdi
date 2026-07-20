// guideclaims_test.go is spec/guide-claims-gate's own gate (ritual-
// integrity#ac-5, ledger L-N4): it decodes the committed
// verdi/docs/guide-claims.yaml (internal/artifact.DecodeGuideClaims,
// itself strict — KnownFields(true), the closed EXISTS/PARTIAL/INVENTED
// enum, atomic-row shape enforced at decode) and proves every EXISTS or
// PARTIAL row's witness set three independent ways, following
// vocabprose_test.go's own witness conventions: (1) the witness NAME
// exists somewhere in this repo's own *_test.go corpus (cmd/, internal/);
// (2) the witness's own declaration carries a `// guide-claim: <row-id>`
// anchor — the vocab:identity marker discipline, extended to name a
// specific row, so a rename or a gutted test becomes a visible lie rather
// than a silent gap (the ADJ-50 lying-gate class); (3) the witness is
// PASS-COUPLED inside `make verify` — proven via scripts/require-pass.sh,
// the same mechanism lint-showcase/showcase-coverage delegate to, driven
// here against a REAL `go test -v` transcript, so a witness that is
// merely present and correctly named but skipped, gated behind an
// unexercised build tag, or never actually invoked cannot satisfy its
// row.
//
// It also proves ac-3's honesty rules: a PARTIAL row without caveat text
// reds, a non-EXISTS row or any DOWNGRADE without a `cite:` reds (an
// independently tested case from the plain non-EXISTS rule, closing the
// red-condition asymmetry the Task-0 design wave's refuters named — a
// gate that only reds on EXISTS-row completeness would make weakening a
// claim the cheapest path to green), and `cite:`'s two-tier check:
// PRESENCE is a decode-time rule with no filesystem dependency at all
// (gated identically in CI and workspace, proven in
// internal/artifact/guideclaims_test.go and again here); RESOLUTION —
// does the cited chronicle/ledger entry genuinely exist — is checked
// WORKSPACE-SIDE ONLY (docs/design/plans/ lives outside this repository,
// a sibling of verdi/), with a loud, disclosed skip when that workspace
// layout is absent (the fidelity_test.go precedent, never a silent pass —
// CLAUDE.md's three-valued honesty).
//
// This file, plus internal/artifact/guideclaims.go's own decode/Validate,
// are wired into `make verify` for free: this package is a test-only
// package (every .go file here is a _test.go file, following
// internal/corpus's and internal/svcfixcanned's own precedent) that `go
// test ./internal/specalign/...` already runs — the Makefile's
// `spec-align` target, and `test`'s CROSS_BINARY_PKGS re-run, both
// already invoke that. No new Makefile target was needed.
//
// DISCLOSED SCOPE (spec/guide-claims-gate ac-4, mirroring
// vocabprose_test.go's own disclosed-scope comment convention): this gate
// proves ROW-TO-WITNESS binding only — every row the manifest DOES carry
// really has a genuine, anchored, passing witness (or a cited
// justification for not having one). It does NOT prove GUIDE-TO-ROW
// completeness: that every capability claim the guide's own Appendix B
// prose makes has a corresponding manifest row here AT ALL. That
// completeness check needs the guide itself in-repo to compare against —
// a later-phase, HARD requirement (Task 18's guide-section<->manifest-row
// SET-EQUALITY check, the mcptools/gatecache precedent) this story does
// not claim to satisfy. A second disclosed residual, inherited from the
// vocabprose bargain: semantic SUFFICIENCY of a witness's own assertions
// (does a witness actually assert something meaningful about the claimed
// capability, not just exist and pass) is not machine-provable — this
// gate proves existence, anchoring, and passing, never meaning.
//
// DISCLOSED DEVIATION (judged-ac2-pass-coupling-is-gate-internal-not-verify-
// coupled, accepted): ac-2 binding 3 ("PASS-coupled in make verify") is
// implemented as a fresh, GATE-INTERNAL `go test -v -count=1 -run ^(names)$
// ./...` transcript checked through scripts/require-pass.sh
// (runGuideClaimWitnessTranscript below), not as coupling to make verify's
// own gate invocations. Two consequences are disclosed, not hidden: (1) the
// inner run's conditions differ from make verify's real test run — notably
// it omits -race, which `make test` uses, so a witness whose behavior
// depended on -race would be proven under different conditions than the gate
// it couples to; and (2) the named witnesses execute more than once per make
// verify — the inner transcript run here, plus this package's own run under
// `go test -race ./...` and the CROSS_BINARY_PKGS -count=1 re-run — an
// intentional but real repeated execution. The property actually proven is
// "the witness passes when the gate itself invokes it"; coupling to make
// verify holds only transitively because this gate runs under spec-align.
// The alternative — coupling the gate to make verify's own Makefile gate
// invocations — would tie this gate to Makefile internals and is deferred.
// The ac's named failure shapes (skip, unexercised build tag, never invoked)
// all still red under this construction.
package specalign

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// guideClaimAnchorPrefix is the marker word this gate's anchors use,
// mirroring vocab:identity's own single-marker convention
// (vocabprose_test.go) but naming a SPECIFIC row rather than merely
// flagging deliberateness.
const guideClaimAnchorPrefix = "guide-claim:"

// guideClaimWitnessDecl is one located top-level Go test function
// declaration: which file declares it (for diagnostics) and its own Go
// doc comment (nil if it carries none) — the anchor check's input.
type guideClaimWitnessDecl struct {
	File string
	Doc  *ast.CommentGroup
}

// guideClaimCorpus maps a top-level Go test function's name to its
// declaration.
type guideClaimCorpus map[string]guideClaimWitnessDecl

// isGoTestFuncName reports whether name has the shape `go test` itself
// selects: the literal "Test" prefix followed by nothing, or by a rune
// that is not a lowercase ASCII letter (so "TestFoo" counts, a
// hypothetical "Testify" continuation would too — matching go test's own
// rule exactly; this repo's real witnesses are all plain "TestXxx"
// names).
func isGoTestFuncName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	rest := name[len("Test"):]
	if rest == "" {
		return true
	}
	c := rest[0]
	return c < 'a' || c > 'z'
}

// guideClaimCorpusSkipDir reports directories buildGuideClaimCorpus never
// descends into — build/vendor noise, never legitimate witness homes.
func guideClaimCorpusSkipDir(name string) bool {
	switch name {
	case "testdata", "node_modules", ".git":
		return true
	}
	return false
}

// buildGuideClaimCorpus walks every *_test.go file under root/<dir> for
// each dir in dirs and indexes every top-level (receiverless) Go test
// function declaration it finds, by name. A dir that does not exist under
// root is skipped, not an error (a fixture root may legitimately have
// only some of the dirs); a name declared more than once keeps the first
// hit in walk order — ac-2's checks only need "does a corpus entry exist
// and is it anchored", not a referee for accidental name collisions.
func buildGuideClaimCorpus(t *testing.T, root string, dirs []string) guideClaimCorpus {
	t.Helper()
	corpus := guideClaimCorpus{}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		start := filepath.Join(root, dir)
		if info, err := os.Stat(start); err != nil || !info.IsDir() {
			continue
		}
		err := filepath.Walk(start, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if guideClaimCorpusSkipDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			f, perr := parser.ParseFile(fset, path, src, parser.ParseComments)
			if perr != nil {
				return fmt.Errorf("parsing %s: %w", path, perr)
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !isGoTestFuncName(fn.Name.Name) {
					continue
				}
				if _, exists := corpus[fn.Name.Name]; exists {
					continue
				}
				corpus[fn.Name.Name] = guideClaimWitnessDecl{File: path, Doc: fn.Doc}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("buildGuideClaimCorpus: walking %s: %v", start, err)
		}
	}
	return corpus
}

// hasGuideClaimAnchor reports whether doc (a declaration's own Go doc
// comment group — nil if it has none) carries a line naming rowID as a
// "// guide-claim: <row-id>" anchor. Multiple such lines may stack in one
// doc comment (several rows legitimately sharing one witness), each
// checked independently by its own row ID.
func hasGuideClaimAnchor(doc *ast.CommentGroup, rowID string) bool {
	if doc == nil {
		return false
	}
	want := guideClaimAnchorPrefix + " " + rowID
	for _, c := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if text == want {
			return true
		}
	}
	return false
}

// checkWitnessNameInCorpus is ac-2 binding 1: witness.Name must exist
// somewhere in corpus. Returns a finding naming both the row and the
// missing witness if absent, "" if present.
func checkWitnessNameInCorpus(corpus guideClaimCorpus, rowID string, witness artifact.GuideClaimWitness) string {
	if _, ok := corpus[witness.Name]; !ok {
		return fmt.Sprintf("row %s: witness %s not found anywhere in the corpus (no top-level Go test function by that name)", rowID, witness.Name)
	}
	return ""
}

// checkWitnessAnchor is ac-2 binding 2: witness.Name's own declaration
// must carry a "// guide-claim: <rowID>" anchor. Assumes the corpus hit
// already exists (checkWitnessNameInCorpus's job); returns "" harmlessly
// if not, so callers may run both checks unconditionally.
func checkWitnessAnchor(corpus guideClaimCorpus, rowID string, witness artifact.GuideClaimWitness) string {
	decl, ok := corpus[witness.Name]
	if !ok {
		return ""
	}
	if !hasGuideClaimAnchor(decl.Doc, rowID) {
		return fmt.Sprintf("row %s: witness %s exists in the corpus but carries no `// guide-claim: %s` anchor at its own declaration (%s) — a rename or a gutted test must become a visible lie, not a silent gap", rowID, witness.Name, rowID, decl.File)
	}
	return ""
}

// requirePassScriptPath is scripts/require-pass.sh's committed location.
func requirePassScriptPath(root string) string {
	return filepath.Join(root, "scripts", "require-pass.sh")
}

// checkWitnessesPassCoupled is ac-2 binding 3: every name in names must
// have emitted a "--- PASS: <name> (" line in transcript, delegated to
// the REAL scripts/require-pass.sh (the same guard lint-showcase/
// showcase-coverage use) — never a reimplementation, so there is one
// canonical PASS-line predicate in this repository. Returns the guard's
// own stderr (naming the first offending witness) on failure, "" on
// success.
func checkWitnessesPassCoupled(t *testing.T, root string, names []string, transcript string) string {
	t.Helper()
	if len(names) == 0 {
		return ""
	}
	script := requirePassScriptPath(root)
	cmd := exec.Command("bash", script, strings.Join(names, " "))
	cmd.Stdin = strings.NewReader(transcript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return ""
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return stderr.String()
	}
	t.Fatalf("running %s: %v", script, err)
	return ""
}

// runGuideClaimWitnessTranscript runs a REAL `go test -v -count=1 -run
// <alternation>` pass over the whole module from root and returns its
// combined output — the transcript ac-2 binding 3 checks against.
// -count=1 always: a stale cached PASS would defeat the entire point of
// "PASS-coupled" (the ADJ-68 cache-honesty concern, applied deliberately
// here). The exit code is intentionally ignored: a witness that FAILED
// still did not emit "--- PASS: <name> (", which is exactly what must red
// below — a nonzero go test exit must not abort this test before that
// evaluation runs. -run is anchored per-name (^(a|b|c)$) so a longer
// witness name can never spuriously satisfy a shorter one.
//
// This is the gate-internal PASS-coupling construction whose deviation is
// disclosed at the package doc comment above (judged-ac2-pass-coupling-is-
// gate-internal-not-verify-coupled): it omits -race and re-executes the
// witness set, rather than coupling to make verify's own gate invocations.
func runGuideClaimWitnessTranscript(t *testing.T, root string, names []string) string {
	t.Helper()
	if len(names) == 0 {
		return ""
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	cmd := exec.Command("go", "test", "-v", "-count=1", "-run", pattern, "./...")
	cmd.Dir = root
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// decodeRealGuideClaims reads and strict-decodes verdi/docs/guide-claims.yaml.
func decodeRealGuideClaims(t *testing.T, root string) *artifact.GuideClaimsManifest {
	t.Helper()
	path := filepath.Join(root, "docs", "guide-claims.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	m, err := artifact.DecodeGuideClaims(data)
	if err != nil {
		t.Fatalf("DecodeGuideClaims(%s): %v", path, err)
	}
	return m
}

// evaluateGuideClaimRows runs ac-2's three bindings for every EXISTS/
// PARTIAL row in m and returns every finding (one row's problem per
// entry). INVENTED rows are skipped entirely — ac-2 binds EXISTS/PARTIAL
// rows only.
func evaluateGuideClaimRows(t *testing.T, root string, m *artifact.GuideClaimsManifest, corpus guideClaimCorpus, transcript string) []string {
	t.Helper()
	var findings []string
	for _, row := range m.Rows {
		if row.Status == artifact.GuideClaimInvented {
			continue
		}
		// Backstop only: artifact.GuideClaimsManifest.Validate now
		// fail-closes this at DECODE (judged-ac2-zero-witness-red-untested),
		// so decodeRealGuideClaims never yields a witnessed-status row with
		// an empty witness set. This branch keeps the gate honest for any
		// future caller that hands evaluateGuideClaimRows a manifest it
		// constructed without decoding.
		if len(row.Witnesses) == 0 {
			findings = append(findings, fmt.Sprintf("row %s: status %s requires at least one witness, has none", row.ID, row.Status))
			continue
		}
		var names []string
		for _, w := range row.Witnesses {
			names = append(names, w.Name)
			if f := checkWitnessNameInCorpus(corpus, row.ID, w); f != "" {
				findings = append(findings, f)
				continue
			}
			if f := checkWitnessAnchor(corpus, row.ID, w); f != "" {
				findings = append(findings, f)
			}
		}
		if f := checkWitnessesPassCoupled(t, root, names, transcript); f != "" {
			findings = append(findings, fmt.Sprintf("row %s: %s", row.ID, strings.TrimSpace(f)))
		}
	}
	return findings
}

// TestGuideClaimsManifest_RowToWitnessBinding is spec/guide-claims-gate's
// live gate: decodes the REAL verdi/docs/guide-claims.yaml, builds a REAL
// corpus from this repo's own cmd/ and internal/ trees, runs a REAL `go
// test -v` pass over every EXISTS/PARTIAL row's named witnesses, and
// fails naming every row whose three-way binding does not hold. A clean
// run here doubles as ac-2 case 4's positive proof (every binding
// genuinely satisfied) for every row the real manifest carries, not just
// one synthetic fixture — see also
// TestGuideClaimsWitnessBinding_AllThreeBindingsSatisfied_Clean below for
// an isolated, hermetic version of that same case.
func TestGuideClaimsManifest_RowToWitnessBinding(t *testing.T) {
	root := verdiRepoRoot
	m := decodeRealGuideClaims(t, root)

	corpus := buildGuideClaimCorpus(t, root, []string{"cmd", "internal"})

	seen := map[string]bool{}
	var names []string
	for _, row := range m.Rows {
		if row.Status == artifact.GuideClaimInvented {
			continue
		}
		for _, w := range row.Witnesses {
			if !seen[w.Name] {
				seen[w.Name] = true
				names = append(names, w.Name)
			}
		}
	}
	sort.Strings(names)
	transcript := runGuideClaimWitnessTranscript(t, root, names)

	findings := evaluateGuideClaimRows(t, root, m, corpus, transcript)
	if len(findings) > 0 {
		t.Errorf("guide-claims.yaml row-to-witness binding failed for %d row(s):\n  %s", len(findings), strings.Join(findings, "\n  "))
	}
}

// writeGuideClaimFixtureFile creates root/relPath with content, creating
// parent directories as needed — test infrastructure for the hermetic
// red/green cases below, never itself part of the check under test.
func writeGuideClaimFixtureFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for fixture %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", relPath, err)
	}
}

// TestGuideClaimsWitnessBinding_NameAbsentFromCorpus_Reds is ac-2 case 1.
func TestGuideClaimsWitnessBinding_NameAbsentFromCorpus_Reds(t *testing.T) {
	dir := t.TempDir()
	writeGuideClaimFixtureFile(t, dir, "internal/x/foo_test.go", "package x\n\nfunc TestSomethingElse(t *testing.T) {}\n")

	corpus := buildGuideClaimCorpus(t, dir, []string{"internal"})
	f := checkWitnessNameInCorpus(corpus, "9-fake-row", artifact.GuideClaimWitness{Name: "TestDoesNotExistAnywhere"})
	if f == "" {
		t.Fatal("want a finding naming the missing witness, got none")
	}
	if !strings.Contains(f, "9-fake-row") || !strings.Contains(f, "TestDoesNotExistAnywhere") {
		t.Errorf("finding = %q, want it to name both the row and the missing witness", f)
	}
}

// TestGuideClaimsWitnessBinding_WitnessPresentButUnanchored_Reds is ac-2
// case 2, constructed so the witness is otherwise real (the corpus-name
// check passes) and only the anchor is missing — isolating the
// missing-anchor case specifically (the ADJ-50 lying-gate class this
// obligation names: name existence alone must not be sufficient).
func TestGuideClaimsWitnessBinding_WitnessPresentButUnanchored_Reds(t *testing.T) {
	dir := t.TempDir()
	writeGuideClaimFixtureFile(t, dir, "internal/x/foo_test.go", "package x\n\n// TestRealWitness proves something real, but carries no anchor.\nfunc TestRealWitness(t *testing.T) {}\n")

	corpus := buildGuideClaimCorpus(t, dir, []string{"internal"})
	w := artifact.GuideClaimWitness{Name: "TestRealWitness"}

	if f := checkWitnessNameInCorpus(corpus, "9-fake-row", w); f != "" {
		t.Fatalf("setup invariant broken: name-in-corpus check should already pass here, got finding %q", f)
	}
	f := checkWitnessAnchor(corpus, "9-fake-row", w)
	if f == "" {
		t.Fatal("want a finding naming the missing anchor, got none")
	}
	if !strings.Contains(f, "9-fake-row") || !strings.Contains(f, "TestRealWitness") {
		t.Errorf("finding = %q, want it to name both the row and the witness", f)
	}
}

// TestGuideClaimsWitnessBinding_SkippedOrUnexercised_RedsViaRequirePass
// is ac-2 case 3, constructed so BOTH the corpus-name and anchor checks
// already pass and only PASS-coupling fails — isolating the PASS-
// coupling case specifically. A transcript that never mentions the
// witness at all stands in for all three of ac-2's named shapes
// (skipped, build-tag gated and unexercised, or simply never invoked):
// require-pass.sh cannot see a "--- PASS: <name> (" line in any of them,
// so all three red identically through this one mechanism.
func TestGuideClaimsWitnessBinding_SkippedOrUnexercised_RedsViaRequirePass(t *testing.T) {
	dir := t.TempDir()
	writeGuideClaimFixtureFile(t, dir, "internal/x/foo_test.go", "package x\n\n// guide-claim: 9-fake-row\nfunc TestRealWitness(t *testing.T) {}\n")

	corpus := buildGuideClaimCorpus(t, dir, []string{"internal"})
	w := artifact.GuideClaimWitness{Name: "TestRealWitness"}

	if f := checkWitnessNameInCorpus(corpus, "9-fake-row", w); f != "" {
		t.Fatalf("setup invariant broken (name-in-corpus): %q", f)
	}
	if f := checkWitnessAnchor(corpus, "9-fake-row", w); f != "" {
		t.Fatalf("setup invariant broken (anchor): %q", f)
	}

	transcript := "=== RUN   TestUnrelated\n--- PASS: TestUnrelated (0.00s)\nPASS\nok  \tpkg\t0.1s\n"
	f := checkWitnessesPassCoupled(t, verdiRepoRoot, []string{w.Name}, transcript)
	if f == "" {
		t.Fatal("want a finding (require-pass.sh red) for a witness with no PASS line in the transcript, got none")
	}
	if !strings.Contains(f, w.Name) {
		t.Errorf("finding = %q, want it to name %s", f, w.Name)
	}
}

// TestGuideClaimsWitnessBinding_AllThreeBindingsSatisfied_Clean is ac-2
// case 4: a hermetic, isolated positive case with all three bindings
// genuinely satisfied at once.
func TestGuideClaimsWitnessBinding_AllThreeBindingsSatisfied_Clean(t *testing.T) {
	dir := t.TempDir()
	writeGuideClaimFixtureFile(t, dir, "internal/x/foo_test.go", "package x\n\n// guide-claim: 9-fake-row\nfunc TestRealWitness(t *testing.T) {}\n")

	corpus := buildGuideClaimCorpus(t, dir, []string{"internal"})
	w := artifact.GuideClaimWitness{Name: "TestRealWitness"}

	if f := checkWitnessNameInCorpus(corpus, "9-fake-row", w); f != "" {
		t.Errorf("name-in-corpus: got finding %q, want none", f)
	}
	if f := checkWitnessAnchor(corpus, "9-fake-row", w); f != "" {
		t.Errorf("anchor: got finding %q, want none", f)
	}
	transcript := "=== RUN   TestRealWitness\n--- PASS: TestRealWitness (0.00s)\nPASS\nok  \tpkg\t0.1s\n"
	if f := checkWitnessesPassCoupled(t, verdiRepoRoot, []string{w.Name}, transcript); f != "" {
		t.Errorf("pass-coupling: got finding %q, want none", f)
	}
}

// TestGuideClaimsCite_PartialWithoutCaveat_Reds is ac-3 case 1.
func TestGuideClaimsCite_PartialWithoutCaveat_Reds(t *testing.T) {
	y := "schema: verdi.guideclaims/v1\nrows:\n" +
		"  - id: x\n    section: \"1\"\n    capability: c\n    status: PARTIAL\n    cite: \"docs/x.md#Y\"\n"
	if _, err := artifact.DecodeGuideClaims([]byte(y)); err == nil {
		t.Fatal("want a decode error for a PARTIAL row with no caveat text, got nil")
	}
}

// TestGuideClaimsCite_NonExistsWithoutCite_Reds is ac-3 case 2, for both
// non-EXISTS statuses.
func TestGuideClaimsCite_NonExistsWithoutCite_Reds(t *testing.T) {
	cases := map[string]string{
		"PARTIAL":  "schema: verdi.guideclaims/v1\nrows:\n  - id: x\n    section: \"1\"\n    capability: c\n    status: PARTIAL\n    caveat: \"narrower than it sounds\"\n",
		"INVENTED": "schema: verdi.guideclaims/v1\nrows:\n  - id: x\n    section: \"1\"\n    capability: c\n    status: INVENTED\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := artifact.DecodeGuideClaims([]byte(y)); err == nil {
				t.Fatalf("status %s with no cite: want a decode error, got nil", name)
			}
		})
	}
}

// TestGuideClaimsCite_PresenceGatedRegardlessOfChronicleReachability is
// ac-3 case 4's PRESENCE leg: cite: presence is a pure decode-time rule
// (artifact.GuideClaimsManifest.Validate) with no filesystem dependency
// at all, so it reds identically in a CI checkout (no workspace-sibling
// docs/design/plans/ tree at all) as it would anywhere else — simulated
// here by simply never touching the filesystem.
func TestGuideClaimsCite_PresenceGatedRegardlessOfChronicleReachability(t *testing.T) {
	y := "schema: verdi.guideclaims/v1\nrows:\n  - id: x\n    section: \"1\"\n    capability: c\n    status: INVENTED\n"
	if _, err := artifact.DecodeGuideClaims([]byte(y)); err == nil {
		t.Fatal("cite: presence must red even with zero filesystem/chronicle access (a CI checkout has no workspace-sibling docs/ tree at all)")
	}
}

// guideClaimStatusRank orders GuideClaimStatus from strongest to weakest
// claim (EXISTS=0 > PARTIAL=1 > INVENTED=2) — a "downgrade" is a rank
// INCREASE.
func guideClaimStatusRank(s artifact.GuideClaimStatus) int {
	switch s {
	case artifact.GuideClaimExists:
		return 0
	case artifact.GuideClaimPartial:
		return 1
	case artifact.GuideClaimInvented:
		return 2
	default:
		return -1
	}
}

// findDowngradesWithoutFreshCite compares two manifest snapshots by row ID
// and returns a finding for every row present in both whose status weakened
// (a rank increase) while its cite: is UNCHANGED from the pre-downgrade row
// (ac-3 case 3). "Fresh" is the load-bearing word: a non-EXISTS row already
// requires SOME cite: at decode (artifact.GuideClaimsManifest.Validate), so
// a blanket "downgrade without cite" rule is satisfiable by simply keeping
// the stale citation that justified the prior, stronger status
// (judged-ac3-downgrade-rule-not-live-and-satisfiable-by-stale-cite). The
// downgrade demands a citation specific to the downgrade, so an UNCHANGED
// cite (nr.Cite == or.Cite, empty or not) reds; only a CHANGED cite clears
// it. This is wired against real history by
// TestGuideClaimsDowngrades_AgainstMergeBase below (git-diff vs the
// merge-base with origin/main), not merely available for a future story.
func findDowngradesWithoutFreshCite(oldM, newM *artifact.GuideClaimsManifest) []string {
	oldByID := make(map[string]artifact.GuideClaimRow, len(oldM.Rows))
	for _, r := range oldM.Rows {
		oldByID[r.ID] = r
	}
	var findings []string
	for _, nr := range newM.Rows {
		or, ok := oldByID[nr.ID]
		if !ok {
			continue
		}
		if guideClaimStatusRank(nr.Status) > guideClaimStatusRank(or.Status) && nr.Cite == or.Cite {
			findings = append(findings, fmt.Sprintf("row %s: status downgraded %s -> %s but cite: is unchanged from the pre-downgrade row (%q) — a downgrade demands a citation specific to the downgrade, not the stale one that justified the prior status", nr.ID, or.Status, nr.Status, nr.Cite))
		}
	}
	return findings
}

// TestFindDowngradesWithoutCite is ac-3 case 3: a fixture pair simulating
// an EXISTS row flipping to PARTIAL across two manifest versions, plus
// the negative paths (downgrade WITH a fresh cite; same status; an upgrade)
// and the stale-cite red case that together prove the function isolates
// exactly the downgrade-without-a-fresh-cite condition.
func TestFindDowngradesWithoutCite(t *testing.T) {
	old := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
		{ID: "x", Status: artifact.GuideClaimExists},
	}}
	newNoCite := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
		{ID: "x", Status: artifact.GuideClaimPartial}, // constructed directly (bypassing decode) to isolate this function
	}}

	findings := findDowngradesWithoutFreshCite(old, newNoCite)
	if len(findings) != 1 {
		t.Fatalf("findDowngradesWithoutFreshCite = %v, want exactly 1 finding", findings)
	}
	if !strings.Contains(findings[0], "x") {
		t.Errorf("finding = %q, want it to name row x", findings[0])
	}

	t.Run("downgrade WITH a fresh (changed) cite is not flagged", func(t *testing.T) {
		newWithCite := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "x", Status: artifact.GuideClaimPartial, Cite: "docs/x.md#Y"},
		}}
		if f := findDowngradesWithoutFreshCite(old, newWithCite); len(f) != 0 {
			t.Errorf("want no findings for a downgrade WITH a fresh cite, got %v", f)
		}
	})

	t.Run("downgrade keeping the SAME non-empty cite reds (stale cite)", func(t *testing.T) {
		// judged-ac3-downgrade-rule-not-live-and-satisfiable-by-stale-cite:
		// a PARTIAL->INVENTED downgrade that simply keeps the row's
		// pre-existing cite unchanged must NOT satisfy the gate — the
		// downgrade demands a citation specific to the downgrade, not the
		// stale one that justified the prior status.
		oldPartial := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "x", Status: artifact.GuideClaimPartial, Cite: "docs/c.md#A"},
		}}
		newInvented := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "x", Status: artifact.GuideClaimInvented, Cite: "docs/c.md#A"}, // SAME cite as before
		}}
		if f := findDowngradesWithoutFreshCite(oldPartial, newInvented); len(f) != 1 {
			t.Fatalf("want 1 finding for a downgrade whose cite is unchanged from the pre-downgrade row, got %v", f)
		}
	})

	t.Run("same status is not a downgrade", func(t *testing.T) {
		same := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "x", Status: artifact.GuideClaimExists},
		}}
		if f := findDowngradesWithoutFreshCite(old, same); len(f) != 0 {
			t.Errorf("want no findings for an unchanged status, got %v", f)
		}
	})

	t.Run("upgrade (INVENTED -> EXISTS) is not a downgrade", func(t *testing.T) {
		oldInvented := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "x", Status: artifact.GuideClaimInvented, Cite: "docs/x.md#Y"},
		}}
		upgraded := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "x", Status: artifact.GuideClaimExists},
		}}
		if f := findDowngradesWithoutFreshCite(oldInvented, upgraded); len(f) != 0 {
			t.Errorf("want no findings for an upgrade, got %v", f)
		}
	})
}

// gitMergeBase returns `git merge-base a b` run in dir, and ok=false when
// git has no answer: dir is not a git repo, it is a shallow clone lacking
// the common ancestor, or b is unknown (e.g. origin/main was never fetched).
// It never fails the test — an unavailable merge-base is a disclosed SKIP at
// the call site, not an operational error.
func gitMergeBase(dir, a, b string) (string, bool) {
	out, err := exec.Command("git", "-C", dir, "merge-base", a, b).Output()
	if err != nil {
		return "", false
	}
	base := strings.TrimSpace(string(out))
	if base == "" {
		return "", false
	}
	return base, true
}

// gitShowFile returns the bytes of relPath at rev in dir's git history, and
// ok=false when the path does not exist at that rev (git show exits
// non-zero) — the case that matters here is the first commit that introduces
// the manifest, whose merge-base predates it, so there is no prior value to
// diff.
func gitShowFile(dir, rev, relPath string) ([]byte, bool) {
	out, err := exec.Command("git", "-C", dir, "show", rev+":"+relPath).Output()
	if err != nil {
		return nil, false
	}
	return out, true
}

// TestGuideClaimsDowngrades_AgainstMergeBase is ac-3 case 3 WIRED LIVE
// (judged-ac3-downgrade-rule-not-live-and-satisfiable-by-stale-cite): it
// diffs the current docs/guide-claims.yaml against the version committed at
// the merge-base with origin/main and reds any status downgrade whose cite:
// is unchanged (findDowngradesWithoutFreshCite). Before this, the
// purpose-built detector existed but was wired to nothing, so no make verify
// run would ever red a downgrade AS a downgrade.
//
// It SKIPS, loudly and disclosed, when the diff cannot be computed: git
// history is unavailable (not a repo, a shallow clone, or origin/main
// unfetched), or the manifest did not yet exist at the merge-base. The
// latter is THIS branch's own case — guide-claims.yaml's first version has
// no prior committed value to diff — so the gate is honestly inert here and
// goes live for the next branch that touches the manifest once this lands on
// main. A skip is never a silent pass (CLAUDE.md three-valued honesty); the
// spec-align target surfaces it. The current (working-tree) manifest is the
// "new" side, matching every other check in this gate; on a clean checkout
// it equals HEAD.
func TestGuideClaimsDowngrades_AgainstMergeBase(t *testing.T) {
	base, ok := gitMergeBase(verdiRepoRoot, "HEAD", "origin/main")
	if !ok {
		t.Skipf("DISCLOSURE: no merge-base for HEAD..origin/main under %s (not a git repo, a shallow clone, or origin/main unfetched) — the downgrade gate cannot diff against a prior manifest here. This is a SKIP, not a pass.", verdiRepoRoot)
	}
	oldData, ok := gitShowFile(verdiRepoRoot, base, "docs/guide-claims.yaml")
	if !ok {
		t.Skipf("DISCLOSURE: docs/guide-claims.yaml does not exist at the merge-base %s — this is the manifest's first version on this branch, so there is no prior committed value to diff. The downgrade gate goes live once this lands on main and a later branch modifies the manifest. This is a SKIP, not a pass.", base)
	}
	oldM, err := artifact.DecodeGuideClaims(oldData)
	if err != nil {
		t.Fatalf("decoding guide-claims.yaml at merge-base %s: %v", base, err)
	}
	newM := decodeRealGuideClaims(t, verdiRepoRoot)
	if findings := findDowngradesWithoutFreshCite(oldM, newM); len(findings) > 0 {
		t.Errorf("guide-claims.yaml has %d status downgrade(s) without a fresh cite: vs the merge-base %s:\n  %s", len(findings), base, strings.Join(findings, "\n  "))
	}
}

// TestGuideClaimsDowngrades_GitAware proves the git-aware wiring end-to-end
// over a hermetic fixturegit repository (real commits, real git show/
// merge-base), so the mechanism the live gate above depends on is committed-
// tested independent of this repo's own history state.
func TestGuideClaimsDowngrades_GitAware(t *testing.T) {
	const path = "docs/guide-claims.yaml"
	oldYAML := "schema: verdi.guideclaims/v1\nrows:\n  - id: x\n    section: \"1\"\n    capability: c\n    status: PARTIAL\n    caveat: \"narrower than it sounds\"\n    cite: \"docs/c.md#A\"\n    witnesses:\n      - name: TestSomething\n"
	staleYAML := "schema: verdi.guideclaims/v1\nrows:\n  - id: x\n    section: \"1\"\n    capability: c\n    status: INVENTED\n    cite: \"docs/c.md#A\"\n"
	freshYAML := "schema: verdi.guideclaims/v1\nrows:\n  - id: x\n    section: \"1\"\n    capability: c\n    status: INVENTED\n    cite: \"docs/c.md#B\"\n"

	decodePair := func(t *testing.T, repo *fixturegit.Repo, oldRev, newRev string) (oldM, newM *artifact.GuideClaimsManifest) {
		t.Helper()
		oldData, ok := gitShowFile(repo.Dir, oldRev, path)
		if !ok {
			t.Fatalf("gitShowFile(%s): manifest must exist at the old rev", oldRev)
		}
		newData, ok := gitShowFile(repo.Dir, newRev, path)
		if !ok {
			t.Fatalf("gitShowFile(%s): manifest must exist at the new rev", newRev)
		}
		om, err := artifact.DecodeGuideClaims(oldData)
		if err != nil {
			t.Fatalf("decode old: %v", err)
		}
		nm, err := artifact.DecodeGuideClaims(newData)
		if err != nil {
			t.Fatalf("decode new: %v", err)
		}
		return om, nm
	}

	t.Run("downgrade keeping the stale cite reds via the git-diff path", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{path: oldYAML}, Message: "seed manifest"},
			{Files: map[string]string{path: staleYAML}, Message: "downgrade x to INVENTED keeping the same cite"},
		})
		base, ok := gitMergeBase(repo.Dir, repo.Head, repo.Heads[0])
		if !ok || base != repo.Heads[0] {
			t.Fatalf("gitMergeBase = (%q, %v), want (%q, true)", base, ok, repo.Heads[0])
		}
		oldM, newM := decodePair(t, repo, base, repo.Head)
		if f := findDowngradesWithoutFreshCite(oldM, newM); len(f) != 1 {
			t.Fatalf("want 1 downgrade-with-stale-cite finding via the git path, got %v", f)
		}
	})

	t.Run("downgrade with a fresh (changed) cite passes", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{path: oldYAML}, Message: "seed manifest"},
			{Files: map[string]string{path: freshYAML}, Message: "downgrade x with a downgrade-specific cite"},
		})
		oldM, newM := decodePair(t, repo, repo.Heads[0], repo.Head)
		if f := findDowngradesWithoutFreshCite(oldM, newM); len(f) != 0 {
			t.Fatalf("want no findings for a downgrade with a fresh cite, got %v", f)
		}
	})

	t.Run("gitMergeBase reports unavailable outside a git repo", func(t *testing.T) {
		if base, ok := gitMergeBase(t.TempDir(), "HEAD", "origin/main"); ok {
			t.Fatalf("want ok=false for a non-repo dir, got base %q", base)
		}
	})

	t.Run("gitShowFile reports absent when the path predates the rev (first-version case)", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"README.md": "hi\n"}, Message: "no manifest yet"},
			{Files: map[string]string{path: oldYAML}, Message: "add manifest"},
		})
		if _, ok := gitShowFile(repo.Dir, repo.Heads[0], path); ok {
			t.Fatal("want ok=false for a manifest path that does not exist at the older rev — the live gate's first-version skip depends on this")
		}
	})
}

// parseCite splits a "<path-from-workspace-root>#<anchor text>" cite:
// value. ok is false if there is no "#" separator at all.
func parseCite(cite string) (relPath, anchor string, ok bool) {
	i := strings.Index(cite, "#")
	if i < 0 {
		return "", "", false
	}
	return cite[:i], cite[i+1:], true
}

// resolveCite reports whether cite names a file that exists under
// workspaceRoot and contains anchor as a literal substring — ac-3's
// RESOLUTION check (does the cited entry genuinely exist), deliberately
// workspace-side only (the chronicle lives outside this repository).
func resolveCite(workspaceRoot, cite string) (bool, error) {
	relPath, anchor, ok := parseCite(cite)
	if !ok {
		return false, fmt.Errorf("cite %q is not shaped <path>#<anchor>", cite)
	}
	data, err := os.ReadFile(filepath.Join(workspaceRoot, filepath.FromSlash(relPath)))
	if err != nil {
		return false, fmt.Errorf("reading cited file for %q: %w", cite, err)
	}
	return strings.Contains(string(data), anchor), nil
}

func TestResolveCite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "chronicle.md"), []byte("...\n### PHASE 1 ARCHIVED (PR #165)\n...\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("resolves a real file+anchor", func(t *testing.T) {
		ok, err := resolveCite(dir, "docs/chronicle.md#PHASE 1 ARCHIVED")
		if err != nil || !ok {
			t.Fatalf("resolveCite = (%v, %v), want (true, nil)", ok, err)
		}
	})
	t.Run("file exists but anchor text absent", func(t *testing.T) {
		ok, err := resolveCite(dir, "docs/chronicle.md#NOT THERE")
		if err != nil {
			t.Fatalf("resolveCite: unexpected error %v", err)
		}
		if ok {
			t.Fatal("want false for an anchor that is not in the file")
		}
	})
	t.Run("file does not exist", func(t *testing.T) {
		if _, err := resolveCite(dir, "docs/nope.md#X"); err == nil {
			t.Fatal("want an error for a nonexistent cited file")
		}
	})
	t.Run("malformed cite (no # separator)", func(t *testing.T) {
		if _, err := resolveCite(dir, "docs/chronicle.md"); err == nil {
			t.Fatal("want an error for a cite with no # anchor separator")
		}
	})
}

// guideClaimsWorkspaceWalkLimit bounds guideClaimsWorkspaceRoot's walk UP
// from the verdi module root toward the workspace root. Five levels is
// deliberately generous but finite: the plain layout is
// verdi-system/verdi/ (the marker one level above the module root) and the
// deepest layout this repo actually develops in is the managed worktree
// verdi-system/verdi-wt/<branch>/ (the marker two levels above), per this
// repo's own gc-verb worktree convention; five leaves headroom for an
// extra nesting while still refusing to walk unboundedly toward the
// filesystem root (where a stray docs/design/plans on some unrelated
// machine could otherwise false-positive).
const guideClaimsWorkspaceWalkLimit = 5

// guideClaimsWorkspaceRoot walks UP from verdiRoot (inclusive) across up to
// guideClaimsWorkspaceWalkLimit parent levels and returns the first
// ancestor that carries the workspace marker docs/design/plans/ — the
// directory the guide's cites (docs/design/plans/..., docs/design/
// concepts/...) resolve against, and the root the transcription-fidelity
// and cite-resolution checks read the guide/chronicle from. ok is false
// when no ancestor within the bound carries the marker: a true bare clone
// of verdi alone, which those checks must SKIP loudly rather than fake.
//
// The marker is docs/design/plans specifically — NOT docs/design/specs,
// which fidelity_test.go's own workspaceDocsDir uses via the one-level-up
// convention. The old convention (verdiRoot/../docs/design/specs) reported
// UNAVAILABLE in the verdi-wt/<branch>/ worktree layout where development
// actually happens, so the cite-resolution check silently SKIPPED on the
// very branch that authored the cites (judged-ac3-resolution-check-skips-
// in-authoring-layout). Walking up until the marker is found makes the
// check RUN there. Starting the walk at verdiRoot itself cannot
// false-positive on the module root: the verdi repo's own docs/ tree
// (docs/spikes, docs/guide-claims.yaml) never contains docs/design/plans.
func guideClaimsWorkspaceRoot(verdiRoot string) (string, bool) {
	dir := filepath.Clean(verdiRoot)
	for level := 0; level <= guideClaimsWorkspaceWalkLimit; level++ {
		marker := filepath.Join(dir, "docs", "design", "plans")
		if info, err := os.Stat(marker); err == nil && info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached the filesystem root; no marker within the bound
		}
		dir = parent
	}
	return "", false
}

// TestGuideClaimsWorkspaceRoot proves the walk-up over SYNTHETIC fixtures
// this test fully controls — never against the live environment, because
// where this suite is invoked from (worktree vs. plain checkout vs. bare
// clone) is a fact about the runner, not about the walk's own logic (the
// builder's own friction note 4; the trap the prior
// TestGuideClaimsChronicleAvailable conceded it could not escape).
func TestGuideClaimsWorkspaceRoot(t *testing.T) {
	t.Run("finds the workspace root two levels up (the verdi-wt/<branch> worktree layout)", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, "docs", "design", "plans"), 0o755); err != nil {
			t.Fatal(err)
		}
		verdiDir := filepath.Join(ws, "verdi-wt", "feature-x")
		if err := os.MkdirAll(verdiDir, 0o755); err != nil {
			t.Fatal(err)
		}
		got, ok := guideClaimsWorkspaceRoot(verdiDir)
		if !ok {
			t.Fatal("want ok=true for a verdi module two levels below a docs/design/plans workspace root")
		}
		if got != ws {
			t.Errorf("workspace root = %q, want %q", got, ws)
		}
	})
	t.Run("finds the workspace root one level up (the plain verdi-system/verdi layout)", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, "docs", "design", "plans"), 0o755); err != nil {
			t.Fatal(err)
		}
		verdiDir := filepath.Join(ws, "verdi")
		if err := os.MkdirAll(verdiDir, 0o755); err != nil {
			t.Fatal(err)
		}
		got, ok := guideClaimsWorkspaceRoot(verdiDir)
		if !ok || got != ws {
			t.Fatalf("guideClaimsWorkspaceRoot = (%q, %v), want (%q, true)", got, ok, ws)
		}
	})
	t.Run("marker exactly at the walk bound is found (inclusive)", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, "docs", "design", "plans"), 0o755); err != nil {
			t.Fatal(err)
		}
		// verdiDir sits exactly guideClaimsWorkspaceWalkLimit (5) levels
		// below ws: ws/a/b/c/d/verdi -> checks at levels 0..5 reach ws.
		verdiDir := filepath.Join(ws, "a", "b", "c", "d", "verdi")
		if err := os.MkdirAll(verdiDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if got, ok := guideClaimsWorkspaceRoot(verdiDir); !ok || got != ws {
			t.Fatalf("guideClaimsWorkspaceRoot = (%q, %v), want (%q, true) — the bound must be inclusive of %d levels", got, ok, ws, guideClaimsWorkspaceWalkLimit)
		}
	})
	t.Run("marker beyond the walk bound is not found (bound respected)", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, "docs", "design", "plans"), 0o755); err != nil {
			t.Fatal(err)
		}
		// One level deeper than the bound reaches: ws/a/b/c/d/e/verdi.
		// The walk stops at ws/a (level 5) without ever reaching ws, so
		// this stays hermetic — it never ascends into the real temp-dir
		// ancestors above ws.
		verdiDir := filepath.Join(ws, "a", "b", "c", "d", "e", "verdi")
		if err := os.MkdirAll(verdiDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if got, ok := guideClaimsWorkspaceRoot(verdiDir); ok {
			t.Fatalf("want ok=false for a marker beyond %d levels, got root %q", guideClaimsWorkspaceWalkLimit, got)
		}
	})
	t.Run("bare verdi-only layout reports not-found", func(t *testing.T) {
		// A temp dir with no marker in it; the bounded walk ascends only
		// into real temp-dir ancestors (/var/folders/...), which never
		// carry docs/design/plans — the exact bare-verdi-checkout CI shape.
		bare := t.TempDir()
		if got, ok := guideClaimsWorkspaceRoot(bare); ok {
			t.Fatalf("want ok=false for a rootless temp dir with no workspace marker, got root %q", got)
		}
	})
}

// TestGuideClaimsCite_ResolutionWorkspaceSideOnly is ac-3 case 4's
// RESOLUTION leg: every non-EXISTS row's cite: in the REAL
// verdi/docs/guide-claims.yaml must resolve to a real file+anchor under
// the workspace root, found by walking UP from the module root to the
// docs/design/plans marker (guideClaimsWorkspaceRoot) — so this check
// RUNS in the verdi-wt/<branch> worktree layout where the cites are
// authored, not just in the plain one-level-up layout
// (judged-ac3-resolution-check-skips-in-authoring-layout). A true bare
// clone of verdi alone (no workspace marker within the bound) SKIPS
// loudly, disclosed, never a silent pass (CLAUDE.md's three-valued
// honesty). The skip, when it fires, is surfaced at the make verify
// surface by the spec-align target (which captures `go test -v` and
// prints every `--- SKIP:` notice), so a skip is never invisible there.
func TestGuideClaimsCite_ResolutionWorkspaceSideOnly(t *testing.T) {
	workspaceRoot, ok := guideClaimsWorkspaceRoot(verdiRepoRoot)
	if !ok {
		t.Skipf("DISCLOSURE: no workspace marker docs/design/plans found within %d levels above %s — this looks like a checkout of verdi alone, not the full verdi-system workspace. guide-claims.yaml cite: RESOLUTION cannot be verified in this layout. This is a SKIP, not a pass: a green run here is NOT proof every cite: resolves.", guideClaimsWorkspaceWalkLimit, verdiRepoRoot)
	}

	m := decodeRealGuideClaims(t, verdiRepoRoot)
	for _, row := range m.Rows {
		if row.Cite == "" {
			continue
		}
		t.Run(row.ID, func(t *testing.T) {
			ok, err := resolveCite(workspaceRoot, row.Cite)
			if err != nil {
				t.Fatalf("row %s: cite %q did not resolve: %v", row.ID, row.Cite, err)
			}
			if !ok {
				t.Fatalf("row %s: cite %q: file found but the anchor text is not present in it", row.ID, row.Cite)
			}
		})
	}
}
