package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gotestjson"
	"github.com/jyang234/verdi/internal/store"
)

// --- grammar table (contract 1) -----------------------------------------

func TestParseGoTestProducerRef(t *testing.T) {
	cases := []struct {
		name    string
		ref     string
		wantPkg string
		wantTst string
		wantOK  bool
	}{
		{"valid", "go-test:cmd/verdi:TestFoo", "cmd/verdi", "TestFoo", true},
		{"valid nested package", "go-test:internal/evidence:TestBar_Baz", "internal/evidence", "TestBar_Baz", true},
		{"missing test", "go-test:cmd/verdi", "", "", false},
		{"subtest path", "go-test:cmd/verdi:TestFoo/Sub", "", "", false},
		{"empty package", "go-test::TestFoo", "", "", false},
		{"extra colons", "go-test:cmd/verdi:TestFoo:Extra", "", "", false},
		{"wrong scheme", "checker:cmd/verdi:TestFoo", "", "", false},
		{"empty test", "go-test:cmd/verdi:", "", "", false},
		{"empty ref", "", "", "", false},
		// Go's test-function naming rule (cmd/go/internal/load isTest):
		// "Test", or "Test" followed by a rune that is not lower-case.
		{"bare Test", "go-test:a:Test", "a", "Test", true},
		{"digit after Test", "go-test:a:Test1", "a", "Test1", true},
		{"underscore after Test", "go-test:a:Test_x", "a", "Test_x", true},
		{"upper-case Unicode after Test", "go-test:a:TestÜber", "a", "TestÜber", true},
		{"import-path characters", "go-test:x-y/z_w.v~1+2:TestA", "x-y/z_w.v~1+2", "TestA", true},
		{"lower-case after Test", "go-test:a:Testfoo", "", "", false},
		{"regexp metacharacter", "go-test:sample:Test(", "", "", false},
		{"regexp wildcard", "go-test:sample:.*", "", "", false},
		{"example function", "go-test:ex:ExampleFoo", "", "", false},
		{"fuzz target", "go-test:f:FuzzFoo", "", "", false},
		{"benchmark", "go-test:b:BenchmarkFoo", "", "", false},
		{"not an identifier", "go-test:a:TestFoo-Bar", "", "", false},
		{"space in test", "go-test:a:Test Foo", "", "", false},
		{"ellipsis package", "go-test:...:TestFoo", "", "", false},
		{"parent directory", "go-test:../x:TestFoo", "", "", false},
		{"dot-slash package", "go-test:./sample:TestFoo", "", "", false},
		{"dot package", "go-test:.:TestFoo", "", "", false},
		{"trailing slash", "go-test:sample/:TestFoo", "", "", false},
		{"leading slash", "go-test:/sample:TestFoo", "", "", false},
		{"empty segment", "go-test:a//b:TestFoo", "", "", false},
		{"dot segment", "go-test:a/./b:TestFoo", "", "", false},
		{"dot-dot segment", "go-test:a/../b:TestFoo", "", "", false},
		{"ellipsis segment", "go-test:a/...:TestFoo", "", "", false},
		{"leading-dot segment", "go-test:a/.hidden:TestFoo", "", "", false},
		{"trailing-dot segment", "go-test:a/b.:TestFoo", "", "", false},
		{"space in package", "go-test:a b:TestFoo", "", "", false},
		{"at sign in package", "go-test:a@v1:TestFoo", "", "", false},
		{"backslash in package", "go-test:a\\b:TestFoo", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseGoTestProducerRef(c.ref)
			if (err == nil) != c.wantOK {
				t.Fatalf("parseGoTestProducerRef(%q) err = %v, want ok %v", c.ref, err, c.wantOK)
			}
			if err != nil {
				return
			}
			if got.Package != c.wantPkg || got.Test != c.wantTst {
				t.Errorf("parseGoTestProducerRef(%q) = (%q, %q), want (%q, %q)", c.ref, got.Package, got.Test, c.wantPkg, c.wantTst)
			}
		})
	}
}

// --- discovery + selection table (contract 2) ----------------------------

// obligationQualityInput bundles the varying fields obligationMD renders,
// so every selection-table case can describe just what differs.
type obligationQualityInput struct {
	State        string // "elaborated" | "unresolved-design-debt"
	ProducerKind string
	ProducerRef  string
	SourceKind   string
	SourceRef    string
}

// obligationMD renders one obligation artifact's full markdown content
// (frontmatter + body) for story/ac/forKind, elaborated with q (or,
// state=="unresolved-design-debt", the strict unresolved union).
func obligationMD(story, ac, forKind string, q obligationQualityInput) string {
	var quality string
	if q.State == "unresolved-design-debt" {
		quality = "  state: unresolved-design-debt\n"
	} else {
		quality = "  state: elaborated\n" +
			`  claim: "c"` + "\n" +
			`  falsifier: "f"` + "\n" +
			`  scope: "s"` + "\n" +
			fmt.Sprintf("  producer: { kind: %s, ref: %q }\n", q.ProducerKind, q.ProducerRef) +
			fmt.Sprintf("  authoritative_source: { kind: %s, ref: %q }\n", q.SourceKind, q.SourceRef) +
			"  freshness:\n" +
			"    invalidated_by: [code]\n" +
			`    rule: "r"` + "\n"
	}
	return "---\n" +
		fmt.Sprintf("id: obligation/%s--%s--%s\n", story, ac, forKind) +
		"kind: obligation\n" +
		`title: "T"` + "\n" +
		`owners: ["o"]` + "\n" +
		fmt.Sprintf("for_kind: %s\n", forKind) +
		"quality:\n" + quality +
		"links:\n" +
		fmt.Sprintf("  - { type: verifies, ref: %q }\n", "spec/"+story) +
		"frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }\n" +
		"---\n" +
		"# T\n\nbody\n"
}

func writeObligation(t *testing.T, root, story, ac, forKind, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "obligations", story)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ac+"--"+forKind+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGoTestProducerSelection proves the combined discovery+selection walk
// (contract 2): an unresolved-design-debt obligation is never a candidate;
// a checker-kind or authenticated-human-kind producer is never a
// candidate; a candidate bound to a DIFFERENT CI job is silently excluded
// (no disclosure); a candidate whose producer ref fails the grammar is
// excluded WITH a disclosure naming its obligation; and exactly the one
// well-formed, this-job, test-kind, grammar-valid obligation is selected.
func TestGoTestProducerSelection(t *testing.T) {
	root := t.TempDir()

	writeObligation(t, root, "story-unresolved", "ac-1", "behavioral",
		obligationMD("story-unresolved", "ac-1", "behavioral", obligationQualityInput{State: "unresolved-design-debt"}))

	writeObligation(t, root, "story-checker", "ac-1", "static",
		obligationMD("story-checker", "ac-1", "static", obligationQualityInput{
			State: "elaborated", ProducerKind: "checker", ProducerRef: "verify:static",
			SourceKind: "ci-job", SourceRef: "verify",
		}))

	writeObligation(t, root, "story-human", "ac-1", "attestation",
		obligationMD("story-human", "ac-1", "attestation", obligationQualityInput{
			State: "elaborated", ProducerKind: "authenticated-human", ProducerRef: "role:owner",
			SourceKind: "governed-attestation", SourceRef: "approval:owner",
		}))

	writeObligation(t, root, "story-other-job", "ac-1", "behavioral",
		obligationMD("story-other-job", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/other:TestOther",
			SourceKind: "ci-job", SourceRef: "lint",
		}))

	writeObligation(t, root, "story-malformed", "ac-1", "behavioral",
		obligationMD("story-malformed", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/bad",
			SourceKind: "ci-job", SourceRef: "verify",
		}))

	writeObligation(t, root, "story-valid", "ac-1", "behavioral",
		obligationMD("story-valid", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestA",
			SourceKind: "ci-job", SourceRef: "verify",
		}))

	// A totally broken obligation elsewhere in the store must never break
	// discovery for the other stories.
	brokenDir := filepath.Join(root, ".verdi", "obligations", "story-broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "ac-1--behavioral.md"), []byte("not an obligation at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates, discoverDiscl, err := discoverTestProducerObligations(root)
	if err != nil {
		t.Fatalf("discoverTestProducerObligations: %v", err)
	}
	if len(discoverDiscl) != 1 {
		t.Fatalf("discovery disclosures = %d, want 1 (the broken obligation); got %+v", len(discoverDiscl), discoverDiscl)
	}
	if !disclosure.IsRendered(disclosure.Render(discoverDiscl[0])) {
		t.Errorf("discovery disclosure does not render as a recognized disclosure line")
	}

	// Exactly the three test-kind, ci-job-sourced, elaborated candidates
	// reach selection (other-job, malformed, valid) — checker/human/
	// unresolved never became candidates at all.
	wantSpecs := map[string]bool{"story-other-job": true, "story-malformed": true, "story-valid": true}
	if len(candidates) != len(wantSpecs) {
		t.Fatalf("candidates = %d, want %d; got %+v", len(candidates), len(wantSpecs), candidates)
	}
	for _, c := range candidates {
		if !wantSpecs[c.SpecName] {
			t.Errorf("unexpected candidate spec %q (checker/human/unresolved obligations must never become candidates)", c.SpecName)
		}
	}

	selected, selectDiscl := selectGoTestObligations(root, candidates, "verify")
	if len(selected) != 1 {
		t.Fatalf("selected = %d, want 1; got %+v", len(selected), selected)
	}
	if selected[0].SpecName != "story-valid" || selected[0].Package != "pkg/a" || selected[0].Test != "TestA" {
		t.Errorf("selected[0] = %+v, want story-valid/pkg/a/TestA", selected[0])
	}
	if len(selectDiscl) != 1 {
		t.Fatalf("selection disclosures = %d, want 1 (the malformed producer ref); got %+v", len(selectDiscl), selectDiscl)
	}
	rendered := disclosure.Render(selectDiscl[0])
	if !strings.Contains(rendered, "obligation/story-malformed--ac-1--behavioral") {
		t.Errorf("malformed-ref disclosure = %q, want it to name the obligation", rendered)
	}

	// jobName == "" (not in a named CI job at all) selects nothing, with no
	// disclosures — an elaborated obligation's authoritative_source.ref is
	// never blank, so nothing can ever match an empty job name.
	emptyJobSelected, emptyJobDiscl := selectGoTestObligations(root, candidates, "")
	if len(emptyJobSelected) != 0 || len(emptyJobDiscl) != 0 {
		t.Errorf("selectGoTestObligations(candidates, \"\") = (%d selected, %d disclosures), want (0, 0)", len(emptyJobSelected), len(emptyJobDiscl))
	}
}

// --- emission (contract 5) -------------------------------------------------

// fakeNamedGoTestRunner returns canned output for a package, recording
// every call it received.
type fakeNamedGoTestRunner struct {
	output map[string][]byte
	err    map[string]error
	calls  []struct{ pkg, pattern string }
}

func (f *fakeNamedGoTestRunner) RunNamedGoTest(ctx context.Context, dir, pkgArg, runPattern string) ([]byte, error) {
	f.calls = append(f.calls, struct{ pkg, pattern string }{pkgArg, runPattern})
	if err, ok := f.err[pkgArg]; ok {
		return nil, err
	}
	return f.output[pkgArg], nil
}

// fakeModulePath is the module path writeGoMod declares for a unit-test
// store root; fake streams name packages under it exactly as test2json does
// (the full import path, never the relative package path).
const fakeModulePath = "example.com/m"

// writeGoMod makes root a Go module root declaring fakeModulePath.
func writeGoMod(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+fakeModulePath+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testGoTestJSON renders a canned stream for relPkg, naming it by its full
// import path under fakeModulePath as the real toolchain does.
func testGoTestJSON(relPkg string, results map[string]string) []byte {
	pkg := fakeModulePath + "/" + relPkg
	var b bytes.Buffer
	fmt.Fprintf(&b, `{"Action":"start","Package":%q}`+"\n", pkg)
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)
	overall := gotestjson.ActionPass
	for _, name := range names {
		fmt.Fprintf(&b, `{"Action":"run","Package":%q,"Test":%q}`+"\n", pkg, name)
		fmt.Fprintf(&b, `{"Action":%q,"Package":%q,"Test":%q}`+"\n", results[name], pkg, name)
		if results[name] == gotestjson.ActionFail {
			overall = gotestjson.ActionFail
		}
	}
	fmt.Fprintf(&b, `{"Action":%q,"Package":%q}`+"\n", overall, pkg)
	return b.Bytes()
}

// TestProduceGoTestEvidence_WritesPerObligationRecords proves the full
// wiring (discovery -> selection -> execution -> emission): one selected
// obligation per package/test, records land at the owning spec's own
// derived/<slug>/<commit>/verdicts.json (merged alongside a pre-existing
// coarse record via mergeEvidenceByProducer), and carry the obligation's
// own kind/AC id, the exact producer ref, and the passed-in provenance.
func TestProduceGoTestEvidence_WritesPerObligationRecords(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)

	writeObligation(t, root, "story-a", "ac-1", "behavioral",
		obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestPass",
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	writeObligation(t, root, "story-a", "ac-2", "behavioral",
		obligationMD("story-a", "ac-2", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestFail",
			SourceKind: "ci-job", SourceRef: "verify",
		}))

	const commit = "cccccccccccccccccccccccccccccccccccccccc"

	// A pre-existing coarse self-hosted record for spec/story-a's own
	// verdicts.json, which must survive the merge untouched (it has a
	// different Producer).
	existingDir := filepath.Join(store.DerivedSpecDir(root, store.RefSlug("spec/story-a")), commit)
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"behavioral","verdict":"pass","witness":"w","producer":"verdi-verify-behavioral","provenance":{"source":"ci","pipeline":"913","commit":"` + commit + `"},"digest":"sha256:` + strings.Repeat("a", 64) + `"}]`
	if err := os.WriteFile(filepath.Join(existingDir, "verdicts.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeNamedGoTestRunner{output: map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestPass": gotestjson.ActionPass, "TestFail": gotestjson.ActionFail}),
	}}

	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "7", JobName: "verify", Commit: commit}

	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout); err != nil {
		t.Fatalf("produceGoTestEvidence: %v", err)
	}

	if len(runner.calls) != 1 || runner.calls[0].pkg != "./pkg/a" {
		t.Fatalf("runner.calls = %+v, want exactly one call for pkg/a", runner.calls)
	}
	if runner.calls[0].pattern != "^(TestFail|TestPass)$" {
		t.Errorf("run pattern = %q, want sorted alternation ^(TestFail|TestPass)$", runner.calls[0].pattern)
	}

	got := readVerdicts(t, root, "spec/story-a", commit)
	if len(got) != 3 {
		t.Fatalf("verdicts.json has %d records, want 3 (1 pre-existing coarse + 2 per-test); got %+v", len(got), got)
	}

	byProducer := map[string]artifact.Evidence{}
	for _, r := range got {
		byProducer[r.Producer] = r
	}
	if r, ok := byProducer["verdi-verify-behavioral"]; !ok || r.Verdict != artifact.VerdictPass {
		t.Errorf("pre-existing coarse record not preserved: %+v", byProducer)
	}
	passRec, ok := byProducer["go-test:pkg/a:TestPass"]
	if !ok {
		t.Fatalf("no record for go-test:pkg/a:TestPass; got %+v", got)
	}
	if passRec.Verdict != artifact.VerdictPass || passRec.Kind != artifact.EvidenceBehavioral || len(passRec.EvidenceFor) != 1 || passRec.EvidenceFor[0] != "ac-1" {
		t.Errorf("TestPass record = %+v, want verdict pass, kind behavioral, evidence_for [ac-1]", passRec)
	}
	if passRec.Provenance.JobName != "verify" || passRec.Provenance.Source != artifact.SourceCI {
		t.Errorf("TestPass record provenance = %+v, want the passed-in prov", passRec.Provenance)
	}
	failRec, ok := byProducer["go-test:pkg/a:TestFail"]
	if !ok {
		t.Fatalf("no record for go-test:pkg/a:TestFail; got %+v", got)
	}
	if failRec.Verdict != artifact.VerdictFail || len(failRec.EvidenceFor) != 1 || failRec.EvidenceFor[0] != "ac-2" {
		t.Errorf("TestFail record = %+v, want verdict fail, evidence_for [ac-2]", failRec)
	}
}

// TestProduceGoTestEvidence_AbsentTestDisclosesNoRecord proves a selected
// obligation whose named test never runs emits no record, only a
// disclosure naming the obligation.
func TestProduceGoTestEvidence_AbsentTestDisclosesNoRecord(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	writeObligation(t, root, "story-a", "ac-1", "behavioral",
		obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestGone",
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	const commit = "dddddddddddddddddddddddddddddddddddddddd"
	runner := &fakeNamedGoTestRunner{output: map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestOther": gotestjson.ActionPass}),
	}}
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", JobName: "verify", Commit: commit}

	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout); err != nil {
		t.Fatalf("produceGoTestEvidence: %v", err)
	}
	if !strings.Contains(stdout.String(), "obligation/story-a--ac-1--behavioral") {
		t.Errorf("stdout = %q, want a disclosure naming the obligation", stdout.String())
	}
	if !disclosure.IsRendered(strings.TrimRight(stdout.String(), "\n")) {
		t.Errorf("stdout = %q, want a recognized disclosure line", stdout.String())
	}
	path := filepath.Join(store.DerivedSpecDir(root, store.RefSlug("spec/story-a")), commit, "verdicts.json")
	if _, err := os.Stat(path); err == nil {
		t.Errorf("verdicts.json was written at %s, want none (the named test never ran)", path)
	}
}

// TestProduceGoTestEvidence_RunnerErrorIsOperational proves a broken
// runner invocation (no output at all) is returned as an error, never
// swallowed as a disclosure.
func TestProduceGoTestEvidence_RunnerErrorIsOperational(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	writeObligation(t, root, "story-a", "ac-1", "behavioral",
		obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestA",
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	const commit = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	runner := &fakeNamedGoTestRunner{err: map[string]error{"./pkg/a": fmt.Errorf("boom")}}
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", JobName: "verify", Commit: commit}
	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout); err == nil {
		t.Fatal("produceGoTestEvidence: want error when the runner fails, got nil")
	}
}

// readVerdicts (selfevidence_test.go) already reads and strict-decodes
// derived/<slug>/<commit>/verdicts.json for a specRef — reused here rather
// than redefined (CLAUDE.md: never copy-paste within the same package).

// --- end to end: the produced record satisfies its obligation ------------

// TestProduceGoTestEvidence_EndToEndMatchesObligation proves a produced
// record actually satisfies its obligation through evidence.MatchObligation
// (matched), and that a failing test's record reads violated, never
// matched (contract's own honesty requirement: negative evidence is never
// hidden).
func TestProduceGoTestEvidence_EndToEndMatchesObligation(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	const commit = "ffffffffffffffffffffffffffffffffffffffff"

	writeObligation(t, root, "story-a", "ac-1", "behavioral",
		obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestPass",
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	writeObligation(t, root, "story-a", "ac-2", "behavioral",
		obligationMD("story-a", "ac-2", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestFail",
			SourceKind: "ci-job", SourceRef: "verify",
		}))

	runner := &fakeNamedGoTestRunner{output: map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestPass": gotestjson.ActionPass, "TestFail": gotestjson.ActionFail}),
	}}
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", JobName: "verify", Commit: commit}
	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout); err != nil {
		t.Fatalf("produceGoTestEvidence: %v", err)
	}

	got := readVerdicts(t, root, "spec/story-a", commit)
	byProducer := map[string]artifact.Evidence{}
	for _, r := range got {
		byProducer[r.Producer] = r
	}
	passRec := byProducer["go-test:pkg/a:TestPass"]
	failRec := byProducer["go-test:pkg/a:TestFail"]

	ctx := context.Background()
	passResult, err := evidence.AssessObligation(ctx, evidence.ObligationAssessmentInput{
		StoreRoot: root, SpecName: "story-a", ACID: "ac-1", Kind: artifact.EvidenceBehavioral,
		Record: &passRec, EvaluationCommit: commit,
	})
	if err != nil {
		t.Fatalf("AssessObligation(ac-1): %v", err)
	}
	if passResult.MatchState != evidence.ObligationMatched {
		t.Errorf("ac-1 match state = %q, want matched; reason=%q", passResult.MatchState, passResult.Reason)
	}

	failResult, err := evidence.AssessObligation(ctx, evidence.ObligationAssessmentInput{
		StoreRoot: root, SpecName: "story-a", ACID: "ac-2", Kind: artifact.EvidenceBehavioral,
		Record: &failRec, EvaluationCommit: commit,
	})
	if err != nil {
		t.Fatalf("AssessObligation(ac-2): %v", err)
	}
	if failResult.MatchState != evidence.ObligationViolatedWithWitness {
		t.Errorf("ac-2 match state = %q, want violated-with-witness (a failing test must never read matched)", failResult.MatchState)
	}
	if failResult.Violating == nil || failResult.Violating.Producer != "go-test:pkg/a:TestFail" {
		t.Errorf("ac-2 violating witness = %+v, want the TestFail record", failResult.Violating)
	}
}

// TestGoTestRunPattern proves the -run expression is sorted, anchored at
// both ends, and regexp-quotes every name (defence in depth behind the
// grammar), so no name can widen or break another's match.
func TestGoTestRunPattern(t *testing.T) {
	cases := []struct {
		name  string
		tests []string
		want  string
	}{
		{"sorted and anchored", []string{"TestB", "TestA"}, "^(TestA|TestB)$"},
		{"single name", []string{"TestA"}, "^(TestA)$"},
		{"metacharacters quoted", []string{"Test(", "Test.*"}, `^(Test\(|Test\.\*)$`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := goTestRunPattern(c.tests); got != c.want {
				t.Errorf("goTestRunPattern(%q) = %q, want %q", c.tests, got, c.want)
			}
		})
	}
}

// TestSelectGoTestObligations_NestedModule proves a package path that
// crosses into a nested module (a go.mod below the module root, at the
// package or any directory above it) is disclosed as malformed on its own
// obligation and never selected, while a sibling in the root module is.
func TestSelectGoTestObligations_NestedModule(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	for _, dir := range []string{"nested", "deep/er/mod"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "go.mod"), []byte("module example.com/other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cand := func(id, ref string) testProducerCandidate {
		return testProducerCandidate{SpecName: "s", ACID: id, Kind: artifact.EvidenceBehavioral, ProducerRef: ref, JobRef: "verify", ObligationID: "obligation/s--" + id + "--behavioral"}
	}
	candidates := []testProducerCandidate{
		cand("ac-1", "go-test:nested:TestA"),
		cand("ac-2", "go-test:nested/inner:TestA"),
		cand("ac-3", "go-test:deep/er/mod/pkg:TestA"),
		cand("ac-4", "go-test:deep/er:TestA"),
	}
	selected, discl := selectGoTestObligations(root, candidates, "verify")
	if len(selected) != 1 || selected[0].ACID != "ac-4" {
		t.Fatalf("selected = %+v, want only ac-4 (deep/er is in the root module)", selected)
	}
	if len(discl) != 3 {
		t.Fatalf("disclosures = %+v, want 3", discl)
	}
	for i, d := range discl {
		r := disclosure.Render(d)
		if !strings.Contains(r, goTestProducerMalformedRefSource) || !strings.Contains(r, "nested module") || !strings.Contains(r, candidates[i].ObligationID) {
			t.Errorf("disclosure %d = %q, want a malformed-ref disclosure naming %s and the nested module", i, r, candidates[i].ObligationID)
		}
	}
}

// TestGoModulePath proves the module path is read strictly from root/go.mod
// (the Go Modules Reference module directive, single-line, quoted, or block
// form), that a root with no go.mod is reported as not a module root with no
// error, and that a go.mod declaring zero, two, or a malformed module path is
// an error rather than a guess.
func TestGoModulePath(t *testing.T) {
	cases := []struct {
		name        string
		gomod       *string
		wantPath    string
		wantPresent bool
		wantErr     bool
	}{
		{"single line", goModText("module example.com/m\n\ngo 1.25\n"), "example.com/m", true, false},
		{"trailing comment", goModText("module example.com/m // the module\n"), "example.com/m", true, false},
		{"interpreted quote", goModText("module \"example.com/q\"\n"), "example.com/q", true, false},
		{"raw quote", goModText("module `example.com/r`\n"), "example.com/r", true, false},
		{"block form", goModText("module (\n\texample.com/b\n)\n"), "example.com/b", true, false},
		{"after a comment line", goModText("// module example.com/nope\nmodule example.com/yes\n"), "example.com/yes", true, false},
		{"no go.mod", nil, "", false, false},
		{"no module directive", goModText("go 1.25\n"), "", true, true},
		{"two module directives", goModText("module a.com/x\nmodule b.com/y\n"), "", true, true},
		{"extra token", goModText("module a.com/x b.com/y\n"), "", true, true},
		{"bare module keyword", goModText("module\n"), "", true, true},
		{"unterminated block", goModText("module (\n\ta.com/x\n"), "", true, true},
		{"malformed quote", goModText("module \"a.com/x\n"), "", true, true},
		{"empty quoted path", goModText("module \"\"\n"), "", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if c.gomod != nil {
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(*c.gomod), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, present, err := goModulePath(root)
			if (err != nil) != c.wantErr {
				t.Fatalf("goModulePath err = %v, wantErr %v", err, c.wantErr)
			}
			if present != c.wantPresent {
				t.Errorf("present = %v, want %v", present, c.wantPresent)
			}
			if !c.wantErr && got != c.wantPath {
				t.Errorf("module path = %q, want %q", got, c.wantPath)
			}
		})
	}
}

func goModText(s string) *string { return &s }

// TestProduceGoTestEvidence_NoGoModDisclosesWithoutExec proves a store root
// that is not a Go module root (no go.mod) runs nothing and emits no record:
// each selected obligation is disclosed by name as not run, and the step is
// not an operational error for every other story.
func TestProduceGoTestEvidence_NoGoModDisclosesWithoutExec(t *testing.T) {
	root := t.TempDir()
	writeObligation(t, root, "story-a", "ac-1", "behavioral",
		obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestA",
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	const commit = "1212121212121212121212121212121212121212"
	runner := &fakeNamedGoTestRunner{}
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", JobName: "verify", Commit: commit}
	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout); err != nil {
		t.Fatalf("produceGoTestEvidence: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner.calls = %+v, want none (no module root to run in)", runner.calls)
	}
	if !strings.Contains(stdout.String(), "obligation/story-a--ac-1--behavioral") || !strings.Contains(stdout.String(), "go.mod") {
		t.Errorf("stdout = %q, want a disclosure naming the obligation and the missing go.mod", stdout.String())
	}
	if recs := readVerdicts(t, root, "spec/story-a", commit); len(recs) != 0 {
		t.Errorf("records = %+v, want none", recs)
	}
}

// --- emission guards (contract 5) ------------------------------------------

// TestVerdictForOutcome proves the one mapping from a named test's own
// terminal action to a verdict: pass is pass, fail is fail, and a skipped
// test abstains — never pass. Anything else is an error, never a verdict.
func TestVerdictForOutcome(t *testing.T) {
	cases := []struct {
		action  string
		want    artifact.EvidenceVerdict
		wantErr bool
	}{
		{gotestjson.ActionPass, artifact.VerdictPass, false},
		{gotestjson.ActionFail, artifact.VerdictFail, false},
		{gotestjson.ActionSkip, artifact.VerdictAbstain, false},
		{gotestjson.ActionBench, "", true},
		{gotestjson.ActionOutput, "", true},
		{"", "", true},
	}
	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			got, err := verdictForOutcome(c.action)
			if (err != nil) != c.wantErr || got != c.want {
				t.Errorf("verdictForOutcome(%q) = (%q, %v), want (%q, err %v)", c.action, got, err, c.want, c.wantErr)
			}
		})
	}
}

// TestBuildGoTestRecords proves each record carries its own obligation's
// kind, acceptance criterion, exact producer ref, and exactly the provenance
// passed in (source, pipeline, job, job_name, commit — never re-stamped);
// that the outcome is looked up by the exact top-level name only (never a
// prefix or a subtest); and that a package that did not build or load yields
// a disclosure, not a record.
func TestBuildGoTestRecords(t *testing.T) {
	sel := func(ac, kind, test string) selectedGoTestObligation {
		return selectedGoTestObligation{
			testProducerCandidate: testProducerCandidate{SpecName: "story-a", ACID: ac, Kind: artifact.EvidenceKind(kind), ProducerRef: "go-test:pkg/a:" + test, JobRef: "verify", ObligationID: "obligation/story-a--" + ac + "--" + kind},
			Package:               "pkg/a",
			Test:                  test,
		}
	}
	built := func(tests map[string]string) gotestjson.Result {
		return gotestjson.Result{Package: fakeModulePath + "/pkg/a", Loaded: true, Outcome: gotestjson.ActionPass, Tests: tests}
	}
	localProv := artifact.EvidenceProvenance{Source: artifact.SourceLocal, Pipeline: "913", Job: "7", JobName: "verify", Commit: "c0ffee"}
	ciProv := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "914", Job: "2", JobName: "verify", Commit: "c0ffee"}

	cases := []struct {
		name        string
		sel         selectedGoTestObligation
		res         *gotestjson.Result // nil: no run recorded for the package
		prov        artifact.EvidenceProvenance
		wantVerdict artifact.EvidenceVerdict // "": no record
		wantWhy     string                   // disclosure text when no record
		wantErr     bool
	}{
		{"static pass under local provenance", sel("ac-1", "static", "TestA"), ptrResult(built(map[string]string{"TestA": gotestjson.ActionPass})), localProv, artifact.VerdictPass, "", false},
		{"behavioral fail under ci provenance", sel("ac-2", "behavioral", "TestA"), ptrResult(built(map[string]string{"TestA": gotestjson.ActionFail})), ciProv, artifact.VerdictFail, "", false},
		{"skip abstains", sel("ac-3", "behavioral", "TestA"), ptrResult(built(map[string]string{"TestA": gotestjson.ActionSkip})), ciProv, artifact.VerdictAbstain, "", false},
		{"exact name, never a same-prefix test", sel("ac-4", "behavioral", "TestA"), ptrResult(built(map[string]string{"TestAB": gotestjson.ActionPass})), ciProv, "", "did not run (no terminal event)", false},
		{"a subtest is not its parent", sel("ac-5", "behavioral", "TestA"), ptrResult(built(map[string]string{"TestA/sub": gotestjson.ActionPass})), ciProv, "", "did not run (no terminal event)", false},
		{"package failed to build", sel("ac-6", "behavioral", "TestA"), &gotestjson.Result{Package: fakeModulePath + "/pkg/a", Loaded: true, Outcome: gotestjson.ActionFail, BuildFailed: true, FailedBuild: "x"}, ciProv, "", "it failed to build", false},
		{"package could not be loaded", sel("ac-7", "behavioral", "TestA"), &gotestjson.Result{Package: "./pkg/a", Outcome: gotestjson.ActionFail}, ciProv, "", "the go command could not load it", false},
		{"a benchmark outcome is no verdict", sel("ac-8", "behavioral", "TestA"), ptrResult(built(map[string]string{"TestA": gotestjson.ActionBench})), ciProv, "", "", true},
		{"no run recorded for the package", sel("ac-9", "behavioral", "TestA"), nil, ciProv, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			results := map[string]gotestjson.Result{}
			if c.res != nil {
				results["pkg/a"] = *c.res
			}
			bySpec, discl, err := buildGoTestRecords([]selectedGoTestObligation{c.sel}, results, c.prov)
			if (err != nil) != c.wantErr {
				t.Fatalf("buildGoTestRecords err = %v, wantErr %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			recs := bySpec["spec/story-a"]
			if c.wantVerdict == "" {
				if len(recs) != 0 || len(discl) != 1 {
					t.Fatalf("records %+v, disclosures %+v; want no record and one disclosure", recs, discl)
				}
				if r := disclosure.Render(discl[0]); !strings.Contains(r, c.sel.ObligationID) || !strings.Contains(r, c.wantWhy) {
					t.Errorf("disclosure %q, want it to name %s and say %q", r, c.sel.ObligationID, c.wantWhy)
				}
				return
			}
			if len(recs) != 1 || len(discl) != 0 {
				t.Fatalf("records %+v, disclosures %+v; want one record", recs, discl)
			}
			rec := recs[0]
			if rec.Verdict != c.wantVerdict || rec.Kind != c.sel.Kind || rec.Producer != c.sel.ProducerRef ||
				len(rec.EvidenceFor) != 1 || rec.EvidenceFor[0] != c.sel.ACID || rec.Schema != "verdi.evidence/v1" {
				t.Errorf("record = %+v, want verdict %s kind %s producer %s evidence_for [%s]", rec, c.wantVerdict, c.sel.Kind, c.sel.ProducerRef, c.sel.ACID)
			}
			if rec.Provenance != c.prov {
				t.Errorf("provenance = %+v, want exactly %+v", rec.Provenance, c.prov)
			}
			if want, err := namedTestDigest(rec); err != nil || rec.Digest != want {
				t.Errorf("digest = %q, want namedTestDigest %q (err %v)", rec.Digest, want, err)
			}
		})
	}
}

func ptrResult(r gotestjson.Result) *gotestjson.Result { return &r }

// TestNamedTestDigest proves the digest content-addresses every declared
// fact a record asserts — its verdict included — so a pass and a fail for
// the same obligation never share a digest, and equal inputs always do.
func TestNamedTestDigest(t *testing.T) {
	base := artifact.Evidence{Kind: artifact.EvidenceBehavioral, Producer: "go-test:pkg/a:TestA", EvidenceFor: []string{"ac-1"}, Verdict: artifact.VerdictPass}
	baseDigest, err := namedTestDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := namedTestDigest(base); again != baseDigest {
		t.Fatalf("digest not stable: %q vs %q", again, baseDigest)
	}
	cases := []struct {
		name   string
		change func(*artifact.Evidence)
	}{
		{"verdict", func(e *artifact.Evidence) { e.Verdict = artifact.VerdictFail }},
		{"abstain verdict", func(e *artifact.Evidence) { e.Verdict = artifact.VerdictAbstain }},
		{"kind", func(e *artifact.Evidence) { e.Kind = artifact.EvidenceStatic }},
		{"producer", func(e *artifact.Evidence) { e.Producer = "go-test:pkg/a:TestB" }},
		{"evidence_for", func(e *artifact.Evidence) { e.EvidenceFor = []string{"ac-2"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			changed := base
			changed.EvidenceFor = append([]string(nil), base.EvidenceFor...)
			c.change(&changed)
			got, err := namedTestDigest(changed)
			if err != nil {
				t.Fatal(err)
			}
			if got == baseDigest {
				t.Errorf("changing %s left the digest %q unchanged", c.name, got)
			}
		})
	}
}

// --- wiring into sync --produce --------------------------------------------

// TestRunSync_Produce_GoTestEmitter proves runProduce calls the per-test
// producer with the CI job's declared name (CIInfo.JobName, SI-229 — here
// deliberately different from CIInfo.Job, the ordering id), with the same
// provenance the coarse bundle gets (source ci only in a genuine CI run,
// source local under --force-local), and that a producer failure is the
// verb's operational exit 2, never swallowed.
func TestRunSync_Produce_GoTestEmitter(t *testing.T) {
	cases := []struct {
		name       string
		inCI       bool
		forceLocal bool
		runnerErr  bool
		wantExit   int
		wantSource artifact.ProvenanceSource
	}{
		{"ci run", true, false, false, 0, artifact.SourceCI},
		{"force-local run", false, true, false, 0, artifact.SourceLocal},
		{"producer failure", true, false, true, 2, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.inCI {
				t.Setenv("CI", "true")
			} else {
				t.Setenv("CI", "")
				t.Setenv("GITHUB_ACTIONS", "")
			}
			root, deps := buildProduceDeps(t) // CIInfo{Pipeline: "913", Job: "7", JobName: "verify"}
			writeGoMod(t, root)
			for _, o := range []struct{ ac, kind, test string }{{"ac-1", "static", "TestA"}, {"ac-2", "behavioral", "TestB"}} {
				writeObligation(t, root, "story-w", o.ac, o.kind, obligationMD("story-w", o.ac, o.kind, obligationQualityInput{
					State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:" + o.test,
					SourceKind: "ci-job", SourceRef: "verify",
				}))
			}
			runner := &fakeNamedGoTestRunner{output: map[string][]byte{
				"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass, "TestB": gotestjson.ActionFail}),
			}}
			if c.runnerErr {
				runner.err = map[string]error{"./pkg/a": errors.New("boom")}
			}
			deps.NamedGoTest = runner

			code := runSync(context.Background(), root, testRef, testCommit, false, true, c.forceLocal, deps)
			if code != c.wantExit {
				t.Fatalf("runSync(--produce) exit = %d, want %d; stderr=%s", code, c.wantExit, deps.Stderr.(*bytes.Buffer).String())
			}
			if len(runner.calls) != 1 || runner.calls[0].pkg != "./pkg/a" || runner.calls[0].pattern != "^(TestA|TestB)$" {
				t.Fatalf("runner calls = %+v, want one run of ./pkg/a for ^(TestA|TestB)$", runner.calls)
			}
			if c.wantExit != 0 {
				if !strings.Contains(deps.Stderr.(*bytes.Buffer).String(), "boom") {
					t.Errorf("stderr = %q, want the producer's error", deps.Stderr.(*bytes.Buffer).String())
				}
				return
			}
			wantProv := artifact.EvidenceProvenance{Source: c.wantSource, Pipeline: "913", Job: "7", JobName: "verify", Commit: testCommit}
			got := map[string]artifact.Evidence{}
			for _, r := range readVerdicts(t, root, "spec/story-w", testCommit) {
				got[r.EvidenceFor[0]] = r
			}
			for ac, want := range map[string]struct {
				kind    artifact.EvidenceKind
				verdict artifact.EvidenceVerdict
			}{"ac-1": {artifact.EvidenceStatic, artifact.VerdictPass}, "ac-2": {artifact.EvidenceBehavioral, artifact.VerdictFail}} {
				r, ok := got[ac]
				if !ok {
					t.Errorf("%s: no per-test record; got %+v", ac, got)
					continue
				}
				if r.Kind != want.kind || r.Verdict != want.verdict || r.Provenance != wantProv {
					t.Errorf("%s: record = %+v, want kind %s verdict %s provenance %+v", ac, r, want.kind, want.verdict, wantProv)
				}
			}
		})
	}
}
