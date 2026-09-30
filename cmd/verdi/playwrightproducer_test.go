package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
)

// playwrightTestCommit is the one commit every production in this file runs at.
const playwrightTestCommit = "7070707070707070707070707070707070707070"

// playwrightTestProv is the provenance of the verify job at playwrightTestCommit.
func playwrightTestProv() artifact.EvidenceProvenance {
	return artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "1", JobName: "verify", Commit: playwrightTestCommit}
}

// playwrightCaptureRootDir is the rootDir every captured report names: the
// capture's own fixture-spec directory, after capture.sh's path
// normalization.
const playwrightCaptureRootDir = "/verdi/internal/playwrightjson/testdata/capture/specs"

// playwrightReportFixture returns a report captured from a real run of the
// pinned Playwright (internal/playwrightjson/testdata/reports, reused, never
// copied), its rootDir re-pointed at root's e2e/tests/: the directory the
// harness config's reports are relative to. Nothing else changes.
func playwrightReportFixture(t *testing.T, name, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "playwrightjson", "testdata", "reports", name+".json"))
	if err != nil {
		t.Fatalf("reading captured report %s: %v", name, err)
	}
	return playwrightRepointReport(t, raw, root)
}

func playwrightRepointReport(t *testing.T, raw []byte, root string) []byte {
	t.Helper()
	dir, err := json.Marshal(filepath.ToSlash(filepath.Join(root, "e2e", "tests")))
	if err != nil {
		t.Fatal(err)
	}
	// Each JSON string that starts with the capture directory now starts with
	// root's e2e/tests/ (the closing quote of dir is dropped to keep the tail).
	return bytes.ReplaceAll(raw, []byte(`"`+playwrightCaptureRootDir), dir[:len(dir)-1])
}

// playwrightMutatedReport is a hand-derived malformed variant of a captured
// report: edit applies to its generic JSON, and it is re-pointed at root.
func playwrightMutatedReport(t *testing.T, name, root string, edit func(report map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "playwrightjson", "testdata", "reports", name+".json"))
	if err != nil {
		t.Fatalf("reading captured report %s: %v", name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var report map[string]any
	if err := dec.Decode(&report); err != nil {
		t.Fatal(err)
	}
	edit(report)
	out, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return playwrightRepointReport(t, out, root)
}

// playwrightKeepFiles reduces a captured report to the files keep, with stats
// that count only their tests: the report of a run of those files alone.
func playwrightKeepFiles(t *testing.T, r map[string]any, keep ...string) {
	t.Helper()
	counts := map[string]int{"expected": 0, "unexpected": 0, "flaky": 0, "skipped": 0}
	var count func(suite map[string]any)
	count = func(suite map[string]any) {
		for _, sp := range suite["specs"].([]any) {
			for _, tc := range sp.(map[string]any)["tests"].([]any) {
				counts[tc.(map[string]any)["status"].(string)]++
			}
		}
		if children, ok := suite["suites"].([]any); ok {
			for _, c := range children {
				count(c.(map[string]any))
			}
		}
	}
	var kept []any
	for _, s := range r["suites"].([]any) {
		for _, k := range keep {
			if s.(map[string]any)["file"] == k {
				kept = append(kept, s)
				count(s.(map[string]any))
			}
		}
	}
	r["suites"] = kept
	stats := r["stats"].(map[string]any)
	for k, v := range counts {
		stats[k] = v
	}
}

// playwrightDigest is a well-formed digest for a hand-written record.
func playwrightDigest(c byte) string { return "sha256:" + strings.Repeat(string(c), 64) }

// playwrightSpecFiles creates each named file under root/e2e/tests/.
func playwrightSpecFiles(t *testing.T, root string, names ...string) {
	t.Helper()
	dir := filepath.Join(root, "e2e", "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("// fixture spec\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// playwrightRunCall is one call the fake runner received.
type playwrightRunCall struct {
	root  string
	specs []string
}

// playwrightFakeRunner writes report (when non-nil) where the producer asked,
// or returns err, recording every call.
type playwrightFakeRunner struct {
	report []byte
	err    error
	calls  []playwrightRunCall
}

func (f *playwrightFakeRunner) RunPlaywright(ctx context.Context, root string, specs []string, reportPath string) error {
	f.calls = append(f.calls, playwrightRunCall{root: root, specs: append([]string(nil), specs...)})
	if f.err != nil {
		return f.err
	}
	if f.report != nil {
		return os.WriteFile(reportPath, f.report, 0o644)
	}
	return nil
}

// playwrightProduce runs one production of the verify job at
// playwrightTestCommit and returns what it printed.
func playwrightProduce(t *testing.T, root string, runner playwrightRunner) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	err := playwrightProduceEvidence(context.Background(), root, playwrightTestCommit, "verify", runner, playwrightTestProv(), &stdout)
	return stdout.String(), err
}

// --- the grammar (design §7 item 1; SI-292, SI-303) --------------------------

// TestParsePlaywrightProducerRef proves the grammar: the file is
// e2e/tests/<name>.spec.ts with <name> in the harness selector's character
// set, the ref splits at the FIRST ":" after the prefix so the title path
// keeps any later ":", and the title path is non-empty, unpadded, and free
// of every line break SI-303 names.
func TestParsePlaywrightProducerRef(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		ref       string
		wantFile  string
		wantTitle string
		wantErr   string // non-empty: the ref is refused, with this in the reason
	}{
		{name: "a top-level test", ref: "playwright:e2e/tests/00-home.spec.ts:home clicks through to a spec page", wantFile: "e2e/tests/00-home.spec.ts", wantTitle: "home clicks through to a spec page"},
		{name: "describe titles then the test title", ref: "playwright:e2e/tests/a.spec.ts:outer › inner › leaf", wantFile: "e2e/tests/a.spec.ts", wantTitle: "outer › inner › leaf"},
		{name: "a title path containing ':'", ref: "playwright:e2e/tests/a.spec.ts:titles: a colon › case: a colon", wantFile: "e2e/tests/a.spec.ts", wantTitle: "titles: a colon › case: a colon"},
		{name: "a title path that is all colons after the first", ref: "playwright:e2e/tests/a.spec.ts:::", wantFile: "e2e/tests/a.spec.ts", wantTitle: "::"},
		{name: "every allowed name character", ref: "playwright:e2e/tests/A.b_c-9.spec.ts:t", wantFile: "e2e/tests/A.b_c-9.spec.ts", wantTitle: "t"},
		{name: "a tab inside the title", ref: "playwright:e2e/tests/a.spec.ts:a\tb", wantFile: "e2e/tests/a.spec.ts", wantTitle: "a\tb"},

		{name: "another scheme", ref: "go-test:pkg/a:TestA", wantErr: `not "playwright:"`},
		{name: "the scheme in another case", ref: "Playwright:e2e/tests/a.spec.ts:t", wantErr: `not "playwright:"`},
		{name: "no title segment", ref: "playwright:e2e/tests/a.spec.ts", wantErr: "no title path"},
		{name: "no file", ref: "playwright::t", wantErr: "not under e2e/tests/"},
		{name: "a file outside e2e/tests/", ref: "playwright:e2e/tests-v1/a.spec.ts:t", wantErr: "not under e2e/tests/"},
		{name: "a file in e2e/", ref: "playwright:e2e/a.spec.ts:t", wantErr: "not under e2e/tests/"},
		{name: "a relative tests/ file", ref: "playwright:tests/a.spec.ts:t", wantErr: "not under e2e/tests/"},
		{name: "a subdirectory of e2e/tests/", ref: "playwright:e2e/tests/sub/a.spec.ts:t", wantErr: "file name"},
		{name: "a traversal", ref: "playwright:e2e/tests/../tests/a.spec.ts:t", wantErr: "file name"},
		{name: "not a .spec.ts file", ref: "playwright:e2e/tests/a.ts:t", wantErr: ".spec.ts"},
		{name: "a .spec.js file", ref: "playwright:e2e/tests/a.spec.js:t", wantErr: ".spec.ts"},
		{name: "a ':' in the file", ref: "playwright:e2e/tests/a:b.spec.ts:t", wantErr: ".spec.ts"},
		{name: "an empty name", ref: "playwright:e2e/tests/.spec.ts:t", wantErr: "file name"},
		{name: "a leading dot", ref: "playwright:e2e/tests/.a.spec.ts:t", wantErr: "file name"},
		{name: "a leading underscore", ref: "playwright:e2e/tests/_a.spec.ts:t", wantErr: "file name"},
		{name: "a leading hyphen", ref: "playwright:e2e/tests/-a.spec.ts:t", wantErr: "file name"},
		{name: "a non-ASCII letter", ref: "playwright:e2e/tests/é.spec.ts:t", wantErr: "file name"},
		{name: "a space in the name", ref: "playwright:e2e/tests/a b.spec.ts:t", wantErr: "file name"},
		{name: "an empty title path", ref: "playwright:e2e/tests/a.spec.ts:", wantErr: "empty"},
		{name: "a leading space", ref: "playwright:e2e/tests/a.spec.ts: t", wantErr: "whitespace"},
		{name: "a trailing space", ref: "playwright:e2e/tests/a.spec.ts:t ", wantErr: "whitespace"},
		{name: "a leading no-break space", ref: "playwright:e2e/tests/a.spec.ts:\u00a0t", wantErr: "whitespace"},
		{name: "a trailing tab", ref: "playwright:e2e/tests/a.spec.ts:t\t", wantErr: "whitespace"},
		{name: "a line feed", ref: "playwright:e2e/tests/a.spec.ts:a\nb", wantErr: "line break"},
		{name: "a carriage return", ref: "playwright:e2e/tests/a.spec.ts:a\rb", wantErr: "line break"},
		{name: "a next line (U+0085)", ref: "playwright:e2e/tests/a.spec.ts:a\u0085b", wantErr: "line break"},
		{name: "a line separator (U+2028)", ref: "playwright:e2e/tests/a.spec.ts:a\u2028b", wantErr: "line break"},
		{name: "a paragraph separator (U+2029)", ref: "playwright:e2e/tests/a.spec.ts:a\u2029b", wantErr: "line break"},
		{name: "invalid UTF-8", ref: "playwright:e2e/tests/a.spec.ts:a\xffb", wantErr: "UTF-8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := playwrightParseProducerRef(c.ref)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("playwrightParseProducerRef(%q) = %+v, want a refusal containing %q", c.ref, got, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("playwrightParseProducerRef(%q) err = %q, want it to contain %q", c.ref, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("playwrightParseProducerRef(%q) = %v, want it accepted", c.ref, err)
			}
			if got.File != c.wantFile || got.TitlePath != c.wantTitle {
				t.Errorf("playwrightParseProducerRef(%q) = %+v, want file %q title path %q", c.ref, got, c.wantFile, c.wantTitle)
			}
		})
	}
}

// --- selection (design §7 item 3) --------------------------------------------

// TestPlaywrightProducerSelection proves discovery and selection, one
// obligation per row in its own store: a Playwright ref authoritative for
// the running job is selected; another job's, or any ref when no job was
// detected, is skipped silently; a go-test ref is the Go-test producer's and
// is skipped silently; a malformed Playwright ref and a runtime-kind
// obligation are disclosed on their own obligation and never selected.
func TestPlaywrightProducerSelection(t *testing.T) {
	t.Parallel()
	const ref = "playwright:e2e/tests/a.spec.ts:suite › case: one"
	cases := []struct {
		name       string
		kind       string
		ref        string
		producer   string // "" means test
		job        string
		selected   bool
		wantSource string // non-empty: one disclosure from this source
	}{
		{name: "a Playwright ref for this job", kind: "behavioral", ref: ref, job: "verify", selected: true},
		{name: "a static Playwright obligation for this job", kind: "static", ref: ref, job: "verify", selected: true},
		{name: "another job's", kind: "behavioral", ref: ref, job: "lint"},
		{name: "no detected job", kind: "behavioral", ref: ref, job: ""},
		{name: "a go-test ref for this job", kind: "behavioral", ref: "go-test:pkg/a:TestA", job: "verify"},
		{name: "a checker producer naming a Playwright ref", kind: "static", ref: ref, producer: "checker", job: "verify"},
		{name: "a malformed Playwright ref", kind: "behavioral", ref: "playwright:e2e/tests/a.spec.ts", job: "verify", wantSource: playwrightProducerMalformedRefSource},
		{name: "a runtime-kind obligation", kind: "runtime", ref: ref, job: "verify", wantSource: playwrightProducerRuntimeKindSource},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			producer := c.producer
			if producer == "" {
				producer = "test"
			}
			writeObligation(t, root, "story-a", "ac-1", c.kind, obligationMD("story-a", "ac-1", c.kind, obligationQualityInput{
				State: "elaborated", ProducerKind: producer, ProducerRef: c.ref, SourceKind: "ci-job", SourceRef: "verify",
			}))
			candidates, _, err := discoverTestProducerObligations(root)
			if err != nil {
				t.Fatal(err)
			}
			jobName := c.job
			selected, discl := playwrightSelectObligations(candidates, jobName)
			if c.selected != (len(selected) == 1) || len(selected) > 1 {
				t.Fatalf("selected = %+v, want selected=%v", selected, c.selected)
			}
			if c.selected {
				s := selected[0]
				if s.File != "e2e/tests/a.spec.ts" || s.TitlePath != "suite › case: one" || s.SpecName != "story-a" || s.ACID != "ac-1" || string(s.Kind) != c.kind || s.ProducerRef != c.ref {
					t.Errorf("selected = %+v", s)
				}
			}
			if c.wantSource == "" {
				if len(discl) != 0 {
					t.Fatalf("disclosures = %+v, want none", discl)
				}
				return
			}
			if len(discl) != 1 {
				t.Fatalf("disclosures = %+v, want one from %s", discl, c.wantSource)
			}
			r := disclosure.Render(discl[0])
			if !disclosure.IsRendered(r) || !strings.Contains(r, "["+c.wantSource+"]") || !strings.Contains(r, "obligation/story-a--ac-1--"+c.kind) {
				t.Errorf("disclosure %q, want a rendered %s disclosure naming its own obligation", r, c.wantSource)
			}
		})
	}
}

// --- outcomes (design §5 and §7 item 2; SI-294) ------------------------------

// playwrightOutcomeCase is one obligation named in an outcome test and what
// its production must be: a record with verdict, or no record with a
// disclosure containing absent.
type playwrightOutcomeCase struct {
	ac      string
	kind    string
	ref     string
	verdict artifact.EvidenceVerdict
	absent  string
}

// playwrightAssertOutcomes checks each case against the records produced for
// spec/story-p at playwrightTestCommit and the disclosures printed.
func playwrightAssertOutcomes(t *testing.T, root, stdout string, cases []playwrightOutcomeCase) {
	t.Helper()
	records := map[string]artifact.Evidence{}
	for _, r := range readVerdicts(t, root, "spec/story-p", playwrightTestCommit) {
		if len(r.EvidenceFor) != 1 {
			t.Fatalf("record %+v attests %d criteria, want exactly its obligation's one", r, len(r.EvidenceFor))
		}
		if _, dup := records[r.EvidenceFor[0]]; dup {
			t.Fatalf("two records for %s", r.EvidenceFor[0])
		}
		records[r.EvidenceFor[0]] = r
	}
	for _, c := range cases {
		id := "obligation/story-p--" + c.ac + "--" + c.kind
		rec, ok := records[c.ac]
		if c.absent != "" {
			if ok {
				t.Errorf("%s (%s): record %+v, want none", c.ac, c.ref, rec)
			}
			line := playwrightDisclosureFor(stdout, id)
			if !strings.Contains(line, "["+playwrightProducerAbsentSource+"]") || !strings.Contains(line, c.absent) {
				t.Errorf("%s (%s): disclosure %q, want a %s disclosure containing %q", c.ac, c.ref, line, playwrightProducerAbsentSource, c.absent)
			}
			continue
		}
		if !ok {
			t.Errorf("%s (%s): no record; stdout:\n%s", c.ac, c.ref, stdout)
			continue
		}
		if rec.Verdict != c.verdict {
			t.Errorf("%s (%s): verdict %s, want %s (witness %q)", c.ac, c.ref, rec.Verdict, c.verdict, rec.Witness)
		}
		if string(rec.Kind) != c.kind || rec.Producer != c.ref || rec.Provenance != playwrightTestProv() || rec.Schema != "verdi.evidence/v1" {
			t.Errorf("%s: record %+v, want kind %s, producer %q, and the run's provenance", c.ac, rec, c.kind, c.ref)
		}
		parsed, err := playwrightParseProducerRef(c.ref)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(rec.Witness, parsed.File) || !strings.Contains(rec.Witness, parsed.TitlePath) {
			t.Errorf("%s: witness %q names neither the file nor the title path", c.ac, rec.Witness)
		}
		undigested := rec
		undigested.Digest = ""
		if want, err := namedTestDigest(undigested); err != nil || rec.Digest != want {
			t.Errorf("%s: digest %q, want %q (%v)", c.ac, rec.Digest, want, err)
		}
		if line := playwrightDisclosureFor(stdout, id); line != "" {
			t.Errorf("%s: a record and a disclosure %q", c.ac, line)
		}
	}
	for ac := range records {
		found := false
		for _, c := range cases {
			found = found || c.ac == ac
		}
		if !found {
			t.Errorf("an unexpected record for %s", ac)
		}
	}
}

// playwrightDisclosureFor returns the Playwright producer's printed disclosure
// line scoped to id.
func playwrightDisclosureFor(stdout, id string) string {
	for _, line := range strings.Split(stdout, "\n") {
		if disclosure.IsRendered(line) && strings.Contains(line, " [sync:playwright-producer-") && strings.Contains(line, "] "+id+": ") {
			return line
		}
	}
	return ""
}

// playwrightWriteCases writes each case's obligation for spec story-p,
// authoritative for the verify job.
func playwrightWriteCases(t *testing.T, root string, cases []playwrightOutcomeCase) {
	t.Helper()
	for _, c := range cases {
		writeTestProducerObligation(t, root, "story-p", c.ac, c.kind, c.ref, "verify")
	}
}

// TestProducePlaywrightEvidence_Outcomes proves the §5 mapping over the report
// of a real run of two fixture files (outcomes.json): pass only when the named
// test ran once and passed; fail when it failed, timed out, or needed another
// attempt (the flaky test that failed then passed is a fail); abstain when it
// was skipped, declaratively, at run time, or after an earlier failure in a
// serial group; a ":" in a title and an anonymous describe name nothing extra;
// a same title path in another file is that file's own test; and no record,
// with a disclosure, for a file that does not exist (never passed to the run)
// and for a title path the report does not carry (a renamed test, or a
// describe's title alone). Each record carries its obligation's own kind and
// criterion, its exact producer ref, the run's provenance, and a digest.
func TestProducePlaywrightEvidence_Outcomes(t *testing.T) {
	t.Parallel()
	const out = "playwright:e2e/tests/outcomes.spec.ts:"
	cases := []playwrightOutcomeCase{
		{ac: "ac-1", kind: "behavioral", ref: out + "outcomes › passes", verdict: artifact.VerdictPass},
		{ac: "ac-2", kind: "static", ref: out + "passes at the top level", verdict: artifact.VerdictPass},
		{ac: "ac-3", kind: "behavioral", ref: "playwright:e2e/tests/other.spec.ts:outcomes › passes", verdict: artifact.VerdictPass},
		{ac: "ac-4", kind: "behavioral", ref: out + "outcomes › fails", verdict: artifact.VerdictFail},
		{ac: "ac-5", kind: "behavioral", ref: out + "outcomes › times out", verdict: artifact.VerdictFail},
		{ac: "ac-6", kind: "behavioral", ref: out + "outcomes › is skipped declaratively", verdict: artifact.VerdictAbstain},
		{ac: "ac-7", kind: "behavioral", ref: out + "outcomes › skips itself at run time", verdict: artifact.VerdictAbstain},
		{ac: "ac-8", kind: "behavioral", ref: out + "serial group › first fails", verdict: artifact.VerdictFail},
		{ac: "ac-9", kind: "behavioral", ref: out + "serial group › second is skipped after the failure", verdict: artifact.VerdictAbstain},
		{ac: "ac-10", kind: "behavioral", ref: out + "retried once › fails first then passes", verdict: artifact.VerdictFail},
		{ac: "ac-11", kind: "behavioral", ref: out + "retried once › fails on every attempt", verdict: artifact.VerdictFail},
		{ac: "ac-12", kind: "behavioral", ref: out + "titles: a colon in the describe › case: a colon in the title", verdict: artifact.VerdictPass},
		{ac: "ac-13", kind: "behavioral", ref: out + "outcomes › passes inside an anonymous describe", verdict: artifact.VerdictPass},
		{ac: "ac-14", kind: "behavioral", ref: out + "outcomes › nested describe › passes in a nested describe", verdict: artifact.VerdictPass},
		{ac: "ac-15", kind: "behavioral", ref: out + "expected failures › fails as test.fail() expects", verdict: artifact.VerdictFail},
		{ac: "ac-16", kind: "behavioral", ref: out + "expected failures › passes despite test.fail()", verdict: artifact.VerdictPass},
		{ac: "ac-17", kind: "behavioral", ref: out + "outcomes › renamed away", absent: "is not in the run's report"},
		{ac: "ac-18", kind: "behavioral", ref: out + "outcomes", absent: "is not in the run's report"},
		{ac: "ac-19", kind: "behavioral", ref: out + "outcomes › passes inside an anonymous describe › extra", absent: "is not in the run's report"},
		{ac: "ac-20", kind: "behavioral", ref: "playwright:e2e/tests/missing.spec.ts:outcomes › passes", absent: "does not exist"},
	}
	root := t.TempDir()
	playwrightSpecFiles(t, root, "outcomes.spec.ts", "other.spec.ts")
	playwrightWriteCases(t, root, cases)
	runner := &playwrightFakeRunner{report: playwrightReportFixture(t, "outcomes", root)}

	stdout, err := playwrightProduce(t, root, runner)
	if err != nil {
		t.Fatalf("playwrightProduceEvidence: %v", err)
	}
	want := []playwrightRunCall{{root: root, specs: []string{"other.spec.ts", "outcomes.spec.ts"}}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("runner calls = %+v, want exactly one run of the two present files %+v", runner.calls, want)
	}
	playwrightAssertOutcomes(t, root, stdout, cases)
}

// TestProducePlaywrightEvidence_InterruptedRun proves the mapping over a real
// run interrupted by SIGINT (sigint.json, no run-level error): the test that
// was running is a fail (interrupted), the test before it a pass, and the test
// the run never reached, present in the report with no attempt, gets no
// record and a disclosure that it did not run.
func TestProducePlaywrightEvidence_InterruptedRun(t *testing.T) {
	t.Parallel()
	const file = "playwright:e2e/tests/sigint.spec.ts:"
	cases := []playwrightOutcomeCase{
		{ac: "ac-1", kind: "behavioral", ref: file + "interrupted run › passes before the interrupt", verdict: artifact.VerdictPass},
		{ac: "ac-2", kind: "behavioral", ref: file + "interrupted run › is running when the run is interrupted", verdict: artifact.VerdictFail},
		{ac: "ac-3", kind: "behavioral", ref: file + "interrupted run › never starts", absent: "did not run"},
	}
	root := t.TempDir()
	playwrightSpecFiles(t, root, "sigint.spec.ts")
	playwrightWriteCases(t, root, cases)
	stdout, err := playwrightProduce(t, root, &playwrightFakeRunner{report: playwrightReportFixture(t, "sigint", root)})
	if err != nil {
		t.Fatalf("playwrightProduceEvidence: %v", err)
	}
	playwrightAssertOutcomes(t, root, stdout, cases)
}

// TestProducePlaywrightEvidence_DuplicateElsewhereIsNotOperational proves the
// operational duplicate is only the NAMED title path shared in the named
// file: a duplicate of another title path in that file ("outcomes › fails",
// a hand-derived variant, since Playwright itself refuses one at load time)
// leaves the named test's record intact.
func TestProducePlaywrightEvidence_DuplicateElsewhereIsNotOperational(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	playwrightSpecFiles(t, root, "outcomes.spec.ts", "other.spec.ts")
	cases := []playwrightOutcomeCase{{ac: "ac-1", kind: "behavioral", ref: "playwright:e2e/tests/outcomes.spec.ts:outcomes › passes", verdict: artifact.VerdictPass}}
	playwrightWriteCases(t, root, cases)
	report := playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) {
		playwrightKeepFiles(t, r, "outcomes.spec.ts")
		for _, s := range r["suites"].([]any) {
			for _, d := range s.(map[string]any)["suites"].([]any) {
				describe := d.(map[string]any)
				if describe["title"] != "outcomes" {
					continue
				}
				specs := describe["specs"].([]any)
				for _, sp := range specs {
					if sp.(map[string]any)["title"] == "fails" {
						describe["specs"] = append(specs, sp)
					}
				}
			}
		}
		stats := r["stats"].(map[string]any)
		stats["unexpected"] = stats["unexpected"].(int) + 1
	})
	stdout, err := playwrightProduce(t, root, &playwrightFakeRunner{report: report})
	if err != nil {
		t.Fatalf("playwrightProduceEvidence: %v", err)
	}
	playwrightAssertOutcomes(t, root, stdout, cases)
}

// playwrightDuplicateOtherSpec appends a second copy of other.spec.ts's one
// passing spec to its describe and counts it in the stats, so the variant
// differs from the captured report only by the duplicate.
func playwrightDuplicateOtherSpec(t *testing.T, r map[string]any) {
	t.Helper()
	for _, s := range r["suites"].([]any) {
		suite := s.(map[string]any)
		if suite["file"] != "other.spec.ts" {
			continue
		}
		describe := suite["suites"].([]any)[0].(map[string]any)
		specs := describe["specs"].([]any)
		describe["specs"] = append(specs, specs[0])
	}
	stats := r["stats"].(map[string]any)
	n, err := stats["expected"].(json.Number).Int64()
	if err != nil {
		t.Fatal(err)
	}
	stats["expected"] = n + 1
}

// --- operational errors (design §5 and §7 item 2) ----------------------------

// TestProducePlaywrightEvidence_OperationalErrorsWriteNothing proves every
// operational case of §5 is an error that writes no record for the run and
// leaves an existing verdicts.json byte-identical: a runner failure; a
// missing, truncated, or malformed report; a run-level error (a failed global
// setup, a global timeout, and Playwright's own load-time refusal of a
// duplicate title, each from a real run); two tests in the named file sharing
// the named title path (a hand-derived variant, including a collision through
// a title that itself contains " › "); a run that is not the producer's (more
// than one project — a real run whose one test appears once per project and
// must never be read as a duplicate — another rootDir, an unnamed file,
// retries, or workers); and a missing runner.
func TestProducePlaywrightEvidence_OperationalErrorsWriteNothing(t *testing.T) {
	t.Parallel()
	const (
		otherPasses    = "playwright:e2e/tests/other.spec.ts:outcomes › passes"
		outcomesPasses = "playwright:e2e/tests/outcomes.spec.ts:outcomes › passes"
	)
	both := []string{otherPasses, outcomesPasses}
	cases := []struct {
		name    string
		refs    []string // obligations ac-1, ac-2, ... in order
		runner  func(t *testing.T, root string) playwrightRunner
		wantErr string
	}{
		{"the runner fails", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{err: errors.New("make e2e-setup: exit status 2")}
		}, "make e2e-setup: exit status 2"},
		{"no report", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{}
		}, "wrote no report"},
		{"a truncated report", both, func(t *testing.T, root string) playwrightRunner {
			full := playwrightReportFixture(t, "outcomes", root)
			return &playwrightFakeRunner{report: full[:len(full)/2]}
		}, "unexpected EOF"},
		{"an empty report", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: []byte{}}
		}, "empty"},
		{"a malformed report", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: []byte("Error: Process from config.webServer was not able to start.\n")}
		}, "invalid character"},
		{"a failed global setup", []string{otherPasses}, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightReportFixture(t, "setup-fails", root)}
		}, "capture fixture: global setup failed"},
		{"a global timeout", []string{"playwright:e2e/tests/global-timeout.spec.ts:global timeout › passes before the stop"}, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightReportFixture(t, "global-timeout", root)}
		}, "Timed out waiting 3s for the test suite to run"},
		{"Playwright refuses a duplicate title", []string{"playwright:e2e/tests/duplicate.spec.ts:dup › same title"}, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightReportFixture(t, "duplicate", root)}
		}, `duplicate test title "dup › same title"`},
		{"two tests share the named title path", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) { playwrightDuplicateOtherSpec(t, r) })}
		}, `2 tests in e2e/tests/other.spec.ts share the title path "outcomes › passes"`},
		{"a title containing ' › ' collides with a describe", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) {
				for _, s := range r["suites"].([]any) {
					suite := s.(map[string]any)
					if suite["file"] != "other.spec.ts" {
						continue
					}
					describe := suite["suites"].([]any)[0].(map[string]any)
					spec := map[string]any{}
					for k, v := range describe["specs"].([]any)[0].(map[string]any) {
						spec[k] = v
					}
					spec["title"] = "outcomes › passes"
					suite["specs"] = append(suite["specs"].([]any), spec)
				}
				stats := r["stats"].(map[string]any)
				n, _ := stats["expected"].(json.Number).Int64()
				stats["expected"] = n + 1
			})}
		}, `2 tests in e2e/tests/other.spec.ts share the title path "outcomes › passes"`},
		{"two projects", []string{otherPasses}, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightReportFixture(t, "two-projects", root)}
		}, "2 projects"},
		{"another rootDir", both, func(t *testing.T, root string) playwrightRunner {
			raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "playwrightjson", "testdata", "reports", "outcomes.json"))
			if err != nil {
				t.Fatal(err)
			}
			return &playwrightFakeRunner{report: raw}
		}, "is not the harness's test directory"},
		{"a file the producer did not name", []string{otherPasses}, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightReportFixture(t, "outcomes", root)}
		}, "outcomes.spec.ts, which the producer did not name"},
		{"retries", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) {
				r["config"].(map[string]any)["projects"].([]any)[0].(map[string]any)["retries"] = 1
			})}
		}, "retries 1"},
		{"workers", both, func(t *testing.T, root string) playwrightRunner {
			return &playwrightFakeRunner{report: playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) {
				r["config"].(map[string]any)["workers"] = 2
			})}
		}, "2 workers"},
		{"no runner", both, func(t *testing.T, root string) playwrightRunner { return nil }, "no Playwright runner"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			playwrightSpecFiles(t, root, "other.spec.ts", "outcomes.spec.ts", "global-timeout.spec.ts", "duplicate.spec.ts")
			// Each obligation has an earlier record at this commit, beside
			// another producer's.
			earlier := []artifact.Evidence{
				{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-1"}, Kind: artifact.EvidenceStatic, Verdict: artifact.VerdictPass, Witness: "coarse", Producer: "verify:static", Provenance: playwrightTestProv(), Digest: playwrightDigest('c')},
			}
			for i, ref := range c.refs {
				ac := "ac-" + string(rune('1'+i))
				writeTestProducerObligation(t, root, "story-p", ac, "behavioral", ref, "verify")
				earlier = append(earlier, artifact.Evidence{Schema: "verdi.evidence/v1", EvidenceFor: []string{ac}, Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictPass, Witness: "earlier", Producer: ref, Provenance: playwrightTestProv(), Digest: playwrightDigest('e')})
			}
			if _, err := writeManagedEvidence(root, playwrightTestCommit, nil, map[string][]artifact.Evidence{"spec/story-p": earlier}); err != nil {
				t.Fatal(err)
			}
			before := snapshotDerived(t, root)

			_, err := playwrightProduce(t, root, c.runner(t, root))
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("playwrightProduceEvidence err = %v, want an operational error containing %q", err, c.wantErr)
			}
			if after := snapshotDerived(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("an operational error changed the derived tree:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

// --- repeat production (SI-238) -----------------------------------------------

// TestProducePlaywrightEvidence_RerunWithdrawsUnrecorded proves SI-238 covers
// Playwright producers: a second production at the same commit whose named
// file no longer exists withdraws each earlier record of its producers and
// says so, while another producer's record and a go-test record in the same
// file stay byte for byte.
func TestProducePlaywrightEvidence_RerunWithdrawsUnrecorded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	playwrightSpecFiles(t, root, "outcomes.spec.ts", "other.spec.ts")
	cases := []playwrightOutcomeCase{
		{ac: "ac-1", kind: "behavioral", ref: "playwright:e2e/tests/outcomes.spec.ts:outcomes › passes", verdict: artifact.VerdictPass},
		{ac: "ac-2", kind: "behavioral", ref: "playwright:e2e/tests/outcomes.spec.ts:outcomes › fails", verdict: artifact.VerdictFail},
		{ac: "ac-3", kind: "behavioral", ref: "playwright:e2e/tests/other.spec.ts:outcomes › passes", verdict: artifact.VerdictPass},
	}
	playwrightWriteCases(t, root, cases)
	unmanaged := []artifact.Evidence{
		{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-4"}, Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictPass, Witness: "go", Producer: "go-test:pkg/a:TestA", Provenance: playwrightTestProv(), Digest: playwrightDigest('a')},
		{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-1"}, Kind: artifact.EvidenceStatic, Verdict: artifact.VerdictPass, Witness: "coarse", Producer: "verify:static", Provenance: playwrightTestProv(), Digest: playwrightDigest('c')},
	}
	if _, err := writeManagedEvidence(root, playwrightTestCommit, nil, map[string][]artifact.Evidence{"spec/story-p": unmanaged}); err != nil {
		t.Fatal(err)
	}
	if _, err := playwrightProduce(t, root, &playwrightFakeRunner{report: playwrightReportFixture(t, "outcomes", root)}); err != nil {
		t.Fatalf("attempt 1: %v", err)
	}
	if got := len(readVerdicts(t, root, "spec/story-p", playwrightTestCommit)); got != 5 {
		t.Fatalf("attempt 1 left %d records, want the 2 unmanaged plus 3 produced", got)
	}

	// Attempt 2: outcomes.spec.ts is gone; other.spec.ts still runs.
	if err := os.Remove(filepath.Join(root, "e2e", "tests", "outcomes.spec.ts")); err != nil {
		t.Fatal(err)
	}
	report := playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) { playwrightKeepFiles(t, r, "other.spec.ts") })
	runner := &playwrightFakeRunner{report: report}
	stdout, err := playwrightProduce(t, root, runner)
	if err != nil {
		t.Fatalf("attempt 2: %v", err)
	}
	if want := []playwrightRunCall{{root: root, specs: []string{"other.spec.ts"}}}; !reflect.DeepEqual(runner.calls, want) {
		t.Errorf("attempt 2 runner calls = %+v, want %+v", runner.calls, want)
	}
	got := readVerdicts(t, root, "spec/story-p", playwrightTestCommit)
	var producers []string
	for _, r := range got {
		producers = append(producers, r.Producer)
	}
	sort.Strings(producers)
	if want := []string{"go-test:pkg/a:TestA", "playwright:e2e/tests/other.spec.ts:outcomes › passes", "verify:static"}; !reflect.DeepEqual(producers, want) {
		t.Fatalf("attempt 2 producers = %q, want %q", producers, want)
	}
	for _, r := range got {
		for _, u := range unmanaged {
			if r.Producer == u.Producer && !reflect.DeepEqual(r, u) {
				t.Errorf("unmanaged record changed: %+v, was %+v", r, u)
			}
		}
	}
	for ac, verdict := range map[string]string{"ac-1": "pass", "ac-2": "fail"} {
		line := playwrightDisclosureFor(stdout, "obligation/story-p--"+ac+"--behavioral")
		if !strings.Contains(line, "does not exist") || !strings.Contains(line, "the earlier "+verdict+" record for this producer at this commit was withdrawn") {
			t.Errorf("%s disclosure = %q, want the absence and the withdrawn %s record", ac, line, verdict)
		}
	}
}

// --- nothing to run ------------------------------------------------------------

// TestProducePlaywrightEvidence_NoRunWithoutPresentFiles proves the producer
// does no Node work at all unless a selected obligation names a file that
// exists: with no Playwright obligation (a go-test one only), with only
// another job's, and with only absent files, the runner is never called and
// a nil runner is no error.
func TestProducePlaywrightEvidence_NoRunWithoutPresentFiles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		ref, job  string
		wantLines int
	}{
		{"only a go-test obligation", "go-test:pkg/a:TestA", "verify", 0},
		{"only another job's", "playwright:e2e/tests/other.spec.ts:outcomes › passes", "lint", 0},
		{"only an absent file", "playwright:e2e/tests/missing.spec.ts:outcomes › passes", "verify", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			playwrightSpecFiles(t, root, "other.spec.ts")
			writeTestProducerObligation(t, root, "story-p", "ac-1", "behavioral", c.ref, c.job)
			runner := &playwrightFakeRunner{err: errors.New("must not run")}
			stdout, err := playwrightProduce(t, root, runner)
			if err != nil {
				t.Fatalf("playwrightProduceEvidence: %v", err)
			}
			if len(runner.calls) != 0 {
				t.Errorf("runner calls = %+v, want none", runner.calls)
			}
			if _, err := playwrightProduce(t, root, nil); err != nil {
				t.Errorf("playwrightProduceEvidence(nil runner) = %v, want nil", err)
			}
			lines := 0
			for _, l := range strings.Split(stdout, "\n") {
				if disclosure.IsRendered(l) {
					lines++
				}
			}
			if lines != c.wantLines {
				t.Errorf("printed %d disclosures, want %d:\n%s", lines, c.wantLines, stdout)
			}
		})
	}
}

// --- matched by internal/evidence (design §7 item 4) -------------------------

// TestProducePlaywrightEvidence_MatchedByEvidence proves a produced record
// satisfies its elaborated obligation through evidence.AssessObligation (the
// byte-equal producer match), a failing one reads violated-with-witness, and
// an obligation naming a renamed test gets no record and reads
// producer-missing, a closure blocker, never a pass.
func TestProducePlaywrightEvidence_MatchedByEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	playwrightSpecFiles(t, root, "outcomes.spec.ts", "other.spec.ts")
	cases := []playwrightOutcomeCase{
		{ac: "ac-1", kind: "behavioral", ref: "playwright:e2e/tests/outcomes.spec.ts:outcomes › passes", verdict: artifact.VerdictPass},
		{ac: "ac-2", kind: "behavioral", ref: "playwright:e2e/tests/outcomes.spec.ts:outcomes › fails", verdict: artifact.VerdictFail},
		{ac: "ac-3", kind: "behavioral", ref: "playwright:e2e/tests/outcomes.spec.ts:outcomes › passes (renamed)", absent: "is not in the run's report"},
	}
	playwrightWriteCases(t, root, cases)
	report := playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) { playwrightKeepFiles(t, r, "outcomes.spec.ts") })
	stdout, err := playwrightProduce(t, root, &playwrightFakeRunner{report: report})
	if err != nil {
		t.Fatalf("playwrightProduceEvidence: %v", err)
	}
	playwrightAssertOutcomes(t, root, stdout, cases)

	byAC := map[string]*artifact.Evidence{}
	for _, r := range readVerdicts(t, root, "spec/story-p", playwrightTestCommit) {
		r := r
		byAC[r.EvidenceFor[0]] = &r
	}
	want := map[string]struct {
		state  evidence.ObligationMatchState
		reason evidence.ObligationMatchReason
	}{
		"ac-1": {evidence.ObligationMatched, ""},
		"ac-2": {evidence.ObligationViolatedWithWitness, ""},
		"ac-3": {evidence.ObligationUnproven, evidence.ObligationReasonProducerMissing},
	}
	for ac, w := range want {
		got, err := evidence.AssessObligation(context.Background(), evidence.ObligationAssessmentInput{
			StoreRoot: root, SpecName: "story-p", ACID: ac, Kind: artifact.EvidenceBehavioral,
			Record: byAC[ac], EvaluationCommit: playwrightTestCommit,
		})
		if err != nil {
			t.Fatalf("AssessObligation(%s): %v", ac, err)
		}
		if got.StructuralState != evidence.ObligationElaborated || got.MatchState != w.state || got.Reason != w.reason {
			t.Errorf("%s: %s/%s reason %q, want elaborated/%s reason %q", ac, got.StructuralState, got.MatchState, got.Reason, w.state, w.reason)
		}
	}
}
