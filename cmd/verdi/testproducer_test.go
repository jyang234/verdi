package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseGoTestProducerRef(c.ref)
			if ok != c.wantOK {
				t.Fatalf("parseGoTestProducerRef(%q) ok = %v, want %v", c.ref, ok, c.wantOK)
			}
			if !ok {
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

	selected, selectDiscl := selectGoTestObligations(candidates, "verify")
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
	emptyJobSelected, emptyJobDiscl := selectGoTestObligations(candidates, "")
	if len(emptyJobSelected) != 0 || len(emptyJobDiscl) != 0 {
		t.Errorf("selectGoTestObligations(candidates, \"\") = (%d selected, %d disclosures), want (0, 0)", len(emptyJobSelected), len(emptyJobDiscl))
	}
}

// --- reader table (contract 4) --------------------------------------------

func TestReadNamedTestOutcomes(t *testing.T) {
	const pkg = "pkg/a"

	ev := func(action, test string) string {
		if test == "" {
			return fmt.Sprintf(`{"Action":%q,"Package":%q}`, action, pkg)
		}
		return fmt.Sprintf(`{"Action":%q,"Package":%q,"Test":%q}`, action, pkg, test)
	}
	out := func(test string) string {
		return fmt.Sprintf(`{"Action":"output","Package":%q,"Test":%q,"Output":"ok\n"}`, pkg, test)
	}

	t.Run("pass", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), ev("run", "TestA"), ev("pass", "TestA"), ev("pass", "")}, "\n")
		got, err := readNamedTestOutcomes(strings.NewReader(stream), pkg)
		if err != nil {
			t.Fatalf("readNamedTestOutcomes: %v", err)
		}
		if got["TestA"] != testOutcomePass {
			t.Errorf("TestA outcome = %q, want pass", got["TestA"])
		}
	})

	t.Run("fail", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), ev("run", "TestA"), ev("fail", "TestA"), ev("fail", "")}, "\n")
		got, err := readNamedTestOutcomes(strings.NewReader(stream), pkg)
		if err != nil {
			t.Fatalf("readNamedTestOutcomes: %v", err)
		}
		if got["TestA"] != testOutcomeFail {
			t.Errorf("TestA outcome = %q, want fail", got["TestA"])
		}
	})

	t.Run("skip", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), ev("run", "TestA"), ev("skip", "TestA"), ev("pass", "")}, "\n")
		got, err := readNamedTestOutcomes(strings.NewReader(stream), pkg)
		if err != nil {
			t.Fatalf("readNamedTestOutcomes: %v", err)
		}
		if got["TestA"] != testOutcomeSkip {
			t.Errorf("TestA outcome = %q, want skip", got["TestA"])
		}
	})

	t.Run("absent", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), ev("run", "TestB"), ev("pass", "TestB"), ev("pass", "")}, "\n")
		got, err := readNamedTestOutcomes(strings.NewReader(stream), pkg)
		if err != nil {
			t.Fatalf("readNamedTestOutcomes: %v", err)
		}
		if _, present := got["TestA"]; present {
			t.Errorf("TestA present in outcomes %+v, want absent (it never ran)", got)
		}
	})

	t.Run("subtests present", func(t *testing.T) {
		stream := strings.Join([]string{
			ev("start", ""), ev("run", "TestA"), ev("run", "TestA/sub"),
			ev("pass", "TestA/sub"), ev("pass", "TestA"), ev("pass", ""),
		}, "\n")
		got, err := readNamedTestOutcomes(strings.NewReader(stream), pkg)
		if err != nil {
			t.Fatalf("readNamedTestOutcomes: %v", err)
		}
		if got["TestA"] != testOutcomePass {
			t.Errorf("TestA outcome = %q, want pass (its own terminal event, not the subtest's)", got["TestA"])
		}
		if got["TestA/sub"] != testOutcomePass {
			t.Errorf("TestA/sub outcome = %q, want pass", got["TestA/sub"])
		}
	})

	t.Run("output interleaving", func(t *testing.T) {
		stream := strings.Join([]string{
			ev("start", ""), ev("run", "TestA"), ev("run", "TestB"),
			out("TestA"), out("TestB"), out("TestA"),
			ev("pass", "TestB"), ev("pass", "TestA"), ev("pass", ""),
		}, "\n")
		got, err := readNamedTestOutcomes(strings.NewReader(stream), pkg)
		if err != nil {
			t.Fatalf("readNamedTestOutcomes: %v", err)
		}
		if got["TestA"] != testOutcomePass || got["TestB"] != testOutcomePass {
			t.Errorf("outcomes = %+v, want both TestA and TestB pass despite interleaved output", got)
		}
	})

	t.Run("duplicate terminal events", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), ev("run", "TestA"), ev("pass", "TestA"), ev("pass", "TestA")}, "\n")
		if _, err := readNamedTestOutcomes(strings.NewReader(stream), pkg); err == nil {
			t.Fatal("readNamedTestOutcomes: want error for duplicate terminal event, got nil")
		}
	})

	t.Run("malformed line", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), "not json at all"}, "\n")
		if _, err := readNamedTestOutcomes(strings.NewReader(stream), pkg); err == nil {
			t.Fatal("readNamedTestOutcomes: want error for malformed line, got nil")
		}
	})

	t.Run("truncated stream", func(t *testing.T) {
		stream := strings.Join([]string{ev("start", ""), ev("run", "TestA")}, "\n")
		if _, err := readNamedTestOutcomes(strings.NewReader(stream), pkg); err == nil {
			t.Fatal("readNamedTestOutcomes: want error for truncated stream, got nil")
		}
	})

	t.Run("unknown field is rejected", func(t *testing.T) {
		stream := fmt.Sprintf(`{"Action":"start","Package":%q,"Surprise":true}`, pkg)
		if _, err := readNamedTestOutcomes(strings.NewReader(stream), pkg); err == nil {
			t.Fatal("readNamedTestOutcomes: want error for an unknown field, got nil (strict decode)")
		}
	})
}

// --- emission (contract 5) -------------------------------------------------

// fakeNamedGoTestRunner returns canned output for a package, recording
// every call it received.
type fakeNamedGoTestRunner struct {
	output map[string][]byte
	err    map[string]error
	calls  []struct{ pkg, pattern string }
}

func (f *fakeNamedGoTestRunner) RunNamedGoTest(ctx context.Context, dir, pkg, runPattern string) ([]byte, error) {
	f.calls = append(f.calls, struct{ pkg, pattern string }{pkg, runPattern})
	if err, ok := f.err[pkg]; ok {
		return nil, err
	}
	return f.output[pkg], nil
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
func testGoTestJSON(relPkg string, results map[string]testOutcome) []byte {
	pkg := fakeModulePath + "/" + relPkg
	var b bytes.Buffer
	fmt.Fprintf(&b, `{"Action":"start","Package":%q}`+"\n", pkg)
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)
	overall := "pass"
	for _, name := range names {
		fmt.Fprintf(&b, `{"Action":"run","Package":%q,"Test":%q}`+"\n", pkg, name)
		fmt.Fprintf(&b, `{"Action":%q,"Package":%q,"Test":%q}`+"\n", results[name], pkg, name)
		if results[name] == testOutcomeFail {
			overall = "fail"
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
		"pkg/a": testGoTestJSON("pkg/a", map[string]testOutcome{"TestPass": testOutcomePass, "TestFail": testOutcomeFail}),
	}}

	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "7", JobName: "verify", Commit: commit}

	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout); err != nil {
		t.Fatalf("produceGoTestEvidence: %v", err)
	}

	if len(runner.calls) != 1 || runner.calls[0].pkg != "pkg/a" {
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
		"pkg/a": testGoTestJSON("pkg/a", map[string]testOutcome{"TestOther": testOutcomePass}),
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
	runner := &fakeNamedGoTestRunner{err: map[string]error{"pkg/a": fmt.Errorf("boom")}}
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
		"pkg/a": testGoTestJSON("pkg/a", map[string]testOutcome{"TestPass": testOutcomePass, "TestFail": testOutcomeFail}),
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

// TestGoTestRunPattern proves the -run alternation is sorted and anchored.
func TestGoTestRunPattern(t *testing.T) {
	got := goTestRunPattern([]string{"TestB", "TestA"})
	if got != "^(TestA|TestB)$" {
		t.Errorf("goTestRunPattern = %q, want ^(TestA|TestB)$", got)
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
