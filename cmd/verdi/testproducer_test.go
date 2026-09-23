package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

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

// selectionOutcome is where one obligation file ends up after discovery and
// selection for a job.
type selectionOutcome int

const (
	ignoredEntirely     selectionOutcome = iota // never a candidate, no disclosure
	anotherJobs                                 // a candidate, silently not this job's
	selectedForJob                              // selected for this job
	disclosedAtDiscover                         // disclosed by discovery, skipped
	disclosedAtSelect                           // disclosed by selection, skipped
)

// TestGoTestProducerSelection proves discovery and selection (contract 2),
// one obligation file per row in its own store: only an elaborated,
// test-producer, ci-job obligation for this job whose ref parses is selected;
// unresolved, checker, and human obligations are never candidates; another
// job's obligation is skipped silently (and so is everything when no job
// name was detected); an undecodable file or a copy away from its convention
// path is disclosed at discovery; a malformed ref, a nested-module package,
// and a runtime-kind obligation are disclosed at selection, each naming only
// its own obligation.
func TestGoTestProducerSelection(t *testing.T) {
	testQ := func(ref, job string) obligationQualityInput {
		return obligationQualityInput{State: "elaborated", ProducerKind: "test", ProducerRef: ref, SourceKind: "ci-job", SourceRef: job}
	}
	cases := []struct {
		name       string
		kind       string
		q          obligationQualityInput
		raw        string // non-empty: the file's content verbatim
		file       string // non-empty: file path under .verdi/obligations/ instead of the convention path
		job        string
		want       selectionOutcome
		wantSource string
	}{
		{name: "test producer for this job", kind: "behavioral", q: testQ("go-test:pkg/a:TestA", "verify"), job: "verify", want: selectedForJob},
		{name: "static test producer for this job", kind: "static", q: testQ("go-test:pkg/a:TestA", "verify"), job: "verify", want: selectedForJob},
		{name: "unresolved design debt", kind: "behavioral", q: obligationQualityInput{State: "unresolved-design-debt"}, job: "verify", want: ignoredEntirely},
		{name: "checker producer", kind: "static", q: obligationQualityInput{State: "elaborated", ProducerKind: "checker", ProducerRef: "verify:static", SourceKind: "ci-job", SourceRef: "verify"}, job: "verify", want: ignoredEntirely},
		{name: "authenticated-human producer", kind: "attestation", q: obligationQualityInput{State: "elaborated", ProducerKind: "authenticated-human", ProducerRef: "role:owner", SourceKind: "governed-attestation", SourceRef: "approval:owner"}, job: "verify", want: ignoredEntirely},
		{name: "another job", kind: "behavioral", q: testQ("go-test:pkg/a:TestA", "lint"), job: "verify", want: anotherJobs},
		{name: "no detected job", kind: "behavioral", q: testQ("go-test:pkg/a:TestA", "verify"), job: "", want: anotherJobs},
		{name: "malformed producer ref", kind: "behavioral", q: testQ("go-test:pkg/a", "verify"), job: "verify", want: disclosedAtSelect, wantSource: goTestProducerMalformedRefSource},
		{name: "nested-module package", kind: "behavioral", q: testQ("go-test:nested/x:TestA", "verify"), job: "verify", want: disclosedAtSelect, wantSource: goTestProducerMalformedRefSource},
		{name: "runtime-kind obligation", kind: "runtime", q: testQ("go-test:pkg/a:TestA", "verify"), job: "verify", want: disclosedAtSelect, wantSource: goTestProducerRuntimeKindSource},
		{name: "undecodable file", raw: "not an obligation at all", job: "verify", want: disclosedAtDiscover, wantSource: testProducerObligationUnreadableSource},
		{name: "copy away from its convention path", kind: "behavioral", q: testQ("go-test:pkg/a:TestA", "verify"), file: "story-a/ac-1--behavioral-copy.md", job: "verify", want: disclosedAtDiscover, wantSource: testProducerObligationMisfiledSource},
		{name: "copy under another story's directory", kind: "behavioral", q: testQ("go-test:pkg/a:TestA", "verify"), file: "story-b/ac-1--behavioral.md", job: "verify", want: disclosedAtDiscover, wantSource: testProducerObligationMisfiledSource},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "nested", "go.mod"), []byte("module example.com/nested\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			content := c.raw
			if content == "" {
				content = obligationMD("story-a", "ac-1", c.kind, c.q)
			}
			rel := c.file
			if rel == "" {
				rel = "story-a/ac-1--" + c.kind + ".md"
				if c.kind == "" {
					rel = "story-a/ac-1--behavioral.md"
				}
			}
			path := filepath.Join(root, ".verdi", "obligations", filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			candidates, discoverDiscl, err := discoverTestProducerObligations(root)
			if err != nil {
				t.Fatalf("discoverTestProducerObligations: %v", err)
			}
			selected, selectDiscl := selectGoTestObligations(root, candidates, c.job)

			wantCandidates, wantSelected, wantDiscover, wantSelect := 0, 0, 0, 0
			switch c.want {
			case anotherJobs:
				wantCandidates = 1
			case selectedForJob:
				wantCandidates, wantSelected = 1, 1
			case disclosedAtDiscover:
				wantDiscover = 1
			case disclosedAtSelect:
				wantCandidates, wantSelect = 1, 1
			}
			if len(candidates) != wantCandidates || len(selected) != wantSelected || len(discoverDiscl) != wantDiscover || len(selectDiscl) != wantSelect {
				t.Fatalf("got %d candidates, %d selected, %d discovery and %d selection disclosures; want %d, %d, %d, %d",
					len(candidates), len(selected), len(discoverDiscl), len(selectDiscl), wantCandidates, wantSelected, wantDiscover, wantSelect)
			}
			if c.want == selectedForJob {
				got := selected[0]
				if got.SpecName != "story-a" || got.ACID != "ac-1" || string(got.Kind) != c.kind || got.Package != "pkg/a" || got.Test != "TestA" || got.ProducerRef != c.q.ProducerRef {
					t.Errorf("selected = %+v, want story-a/ac-1/%s pkg/a TestA", got, c.kind)
				}
			}
			for _, d := range append(discoverDiscl, selectDiscl...) {
				r := disclosure.Render(d)
				if !disclosure.IsRendered(r) || !strings.Contains(r, "["+c.wantSource+"]") {
					t.Errorf("disclosure %q, want a rendered %s disclosure", r, c.wantSource)
				}
				if c.want == disclosedAtSelect && !strings.Contains(r, "obligation/story-a--ac-1--"+c.kind) {
					t.Errorf("disclosure %q does not name its own obligation", r)
				}
			}
		})
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

// TestProduceGoTestEvidence_RejectedStreamIsOperational proves a stream the
// shared reader rejects — one carrying a field Go 1.25.5 does not write
// (internal/gotestjson decodes strictly), or one truncated before its
// package's terminal event (03 §Bundle assembly) — is an operational error
// naming the offending event, and that no record is written for anyone.
func TestProduceGoTestEvidence_RejectedStreamIsOperational(t *testing.T) {
	const pkg = fakeModulePath + "/pkg/a"
	cases := []struct {
		name    string
		stream  string
		wantErr string
	}{
		{"unknown field on a test event",
			`{"Action":"start","Package":"` + pkg + `"}` + "\n" +
				`{"Action":"run","Package":"` + pkg + `","Test":"TestA"}` + "\n" +
				`{"Action":"pass","Package":"` + pkg + `","Test":"TestA","Surprise":1}` + "\n" +
				`{"Action":"pass","Package":"` + pkg + `"}` + "\n",
			`event 3: json: unknown field "Surprise"`},
		{"unknown field on a build event",
			`{"ImportPath":"` + pkg + `","Action":"build-output","Output":"# cgo warning\n","Surprise":1}` + "\n" +
				string(testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass})),
			`event 1: json: unknown field "Surprise"`},
		{"truncated before the package's terminal event",
			`{"Action":"start","Package":"` + pkg + `"}` + "\n" +
				`{"Action":"run","Package":"` + pkg + `","Test":"TestA"}` + "\n" +
				`{"Action":"pass","Package":"` + pkg + `","Test":"TestA"}` + "\n",
			"truncated"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeGoMod(t, root)
			writeObligation(t, root, "story-a", "ac-1", "behavioral",
				obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
					State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestA",
					SourceKind: "ci-job", SourceRef: "verify",
				}))
			const commit = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
			runner := &fakeNamedGoTestRunner{output: map[string][]byte{"./pkg/a": []byte(c.stream)}}
			prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", JobName: "verify", Commit: commit}
			var stdout bytes.Buffer
			err := produceGoTestEvidence(context.Background(), root, commit, "verify", runner, prov, &stdout)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("produceGoTestEvidence err = %v, want an error containing %q", err, c.wantErr)
			}
			path := filepath.Join(store.DerivedSpecDir(root, store.RefSlug("spec/story-a")), commit, "verdicts.json")
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("verdicts.json at %s: stat err = %v, want none written", path, statErr)
			}
		})
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

// --- execution guards (contract 3) ----------------------------------------

// TestRealNamedGoTestRunner proves the real runner's error surface: a
// failing test (nonzero go test exit) still returns the toolchain's stream,
// while a context cancelled before or during the run, a missing go binary,
// and a run with no output are errors that wrap why.
func TestRealNamedGoTestRunner(t *testing.T) {
	fixture := func(*testing.T) string { return "testdata/gotestfixture" }
	cancelled := func(t *testing.T) context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	shortDeadline := func(t *testing.T) context.Context {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		t.Cleanup(cancel)
		return ctx
	}
	background := func(*testing.T) context.Context { return context.Background() }
	cases := []struct {
		name    string
		dir     func(t *testing.T) string
		ctx     func(t *testing.T) context.Context
		goBin   string // non-empty: a fake `go` script put alone on PATH ("-" for none)
		wantErr error  // nil means success
		wantOut string
	}{
		{name: "failing test returns its stream", dir: fixture, ctx: background, wantOut: `"Package":"example.com/gotestfixture/sample"`},
		{name: "cancelled before the run", dir: fixture, ctx: cancelled, wantErr: context.Canceled},
		// The fake go writes part of a stream, then blocks until killed: the
		// kill surfaces as an ExitError, never as a usable stream.
		{name: "killed mid-run by its context", dir: fixture, ctx: shortDeadline, goBin: "#!/bin/sh\nprintf '{\"Action\":\"start\",\"Package\":\"example.com/gotestfixture/sample\"}\\n'\nexec /bin/sleep 30\n", wantErr: context.DeadlineExceeded},
		{name: "no go binary", dir: fixture, ctx: background, goBin: "-", wantErr: exec.ErrNotFound},
		{name: "no output outside any module", dir: func(t *testing.T) string { return t.TempDir() }, ctx: background, wantErr: errNoOutput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hermeticGoEnv(t)
			switch c.goBin {
			case "":
			case "-":
				fakeGoOnPath(t, "")
			default:
				fakeGoOnPath(t, c.goBin)
			}
			start := time.Now()
			out, err := realNamedGoTestRunner{}.RunNamedGoTest(c.ctx(t), c.dir(t), "./sample", goTestRunPattern([]string{"TestFail", "TestPass"}))
			// The fake go sleeps 30s: only a context-bound run returns sooner.
			if elapsed := time.Since(start); elapsed > 20*time.Second {
				t.Errorf("RunNamedGoTest returned after %v: the context never stopped the run", elapsed)
			}
			if c.wantErr == nil {
				if err != nil {
					t.Fatalf("RunNamedGoTest: %v", err)
				}
				if !strings.Contains(string(out), c.wantOut) {
					t.Errorf("stream lacks %s:\n%s", c.wantOut, out)
				}
				return
			}
			if out != nil {
				t.Errorf("RunNamedGoTest returned a stream %q with its error", out)
			}
			if c.wantErr == errNoOutput {
				if err == nil || !strings.Contains(err.Error(), "produced no output") || !strings.Contains(err.Error(), "go.mod file not found") {
					t.Fatalf("RunNamedGoTest err = %v, want a no-output error carrying go's stderr", err)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("RunNamedGoTest err = %v, want one wrapping %v", err, c.wantErr)
			}
		})
	}
}

// fakeGoOnPath is the real runner's exec seam: it puts script, as the only
// `go`, alone on PATH for the rest of t. An empty script puts no go there.
func fakeGoOnPath(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	if script != "" {
		if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

// TestRealNamedGoTestRunner_Argv pins the real runner's command line through
// the fake-`go` seam, a script that records its arguments one per line:
// exactly `go test -json -count=1 -run '^(...)$' <pkgArg>`, nothing more.
// -count=1 is the flag that keeps go test from replaying a cached result
// (SI-228: CI runs `go test -json -count=1` per named package); CI restores
// GOCACHE between runs, so without it an earlier run's result could become
// this job's record. Any added flag fails the pin as well.
func TestRealNamedGoTestRunner_Argv(t *testing.T) {
	const stream = `{"Action":"start","Package":"example.com/gotestfixture/sample"}`
	cases := []struct {
		name    string
		pkgArg  string
		tests   []string
		wantRun string
	}{
		{"one test", "./sample", []string{"TestPass"}, "^(TestPass)$"},
		{"several tests, sorted and anchored", "./sample", []string{"TestPass", "TestFail"}, "^(TestFail|TestPass)$"},
		{"nested package", "./internal/forge", []string{"TestX"}, "^(TestX)$"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hermeticGoEnv(t)
			argvFile := filepath.Join(t.TempDir(), "argv")
			fakeGoOnPath(t, "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > '"+argvFile+"'\nprintf '%s\\n' '"+stream+"'\n")

			out, err := realNamedGoTestRunner{}.RunNamedGoTest(context.Background(), t.TempDir(), c.pkgArg, goTestRunPattern(c.tests))
			if err != nil {
				t.Fatalf("RunNamedGoTest: %v", err)
			}
			if string(out) != stream+"\n" {
				t.Errorf("RunNamedGoTest = %q, want the fake go's stdout %q", out, stream+"\n")
			}
			data, err := os.ReadFile(argvFile)
			if err != nil {
				t.Fatalf("the fake go recorded no arguments: %v", err)
			}
			got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			want := []string{"test", "-json", "-count=1", "-run", c.wantRun, c.pkgArg}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("go argv = %q, want exactly %q", got, want)
			}
		})
	}
}

// errNoOutput marks the runner case whose error is checked by text: go ran,
// exited nonzero, and wrote only to stderr.
var errNoOutput = errors.New("no output")

// TestProduceGoTestEvidence_NilRunner proves a selected obligation with no
// runner configured is an error, never a nil-interface panic.
func TestProduceGoTestEvidence_NilRunner(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	writeObligation(t, root, "story-a", "ac-1", "behavioral",
		obligationMD("story-a", "ac-1", "behavioral", obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: "go-test:pkg/a:TestA",
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	var stdout bytes.Buffer
	err := produceGoTestEvidence(context.Background(), root, "c0ffee", "verify", nil, artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "c0ffee"}, &stdout)
	if err == nil || !strings.Contains(err.Error(), "no test runner") {
		t.Fatalf("produceGoTestEvidence(nil runner) err = %v, want a no-runner error", err)
	}
	// Nothing selected needs no runner.
	if err := produceGoTestEvidence(context.Background(), root, "c0ffee", "lint", nil, artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "c0ffee"}, &stdout); err != nil {
		t.Errorf("produceGoTestEvidence(nil runner, nothing selected) = %v, want nil", err)
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
