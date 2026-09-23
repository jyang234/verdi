package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gotestjson"
	"github.com/jyang234/verdi/internal/store"
)

// The per-test producer's repeat-production contract (SI-238): each
// production run at a commit replaces its managed subset there, which per spec
// is every selected obligation's producer ref. A selected producer the run did
// not record loses any earlier record at that commit. Every other record stays
// as it was: other producers, another job's records, the coarse records, other
// specs, and runtime.json. Nothing is written unless every stream parsed and
// every affected file was read.

// rerunCommit is the one commit every attempt in this file produces at.
const rerunCommit = "9090909090909090909090909090909090909090"

// writeTestProducerObligation writes an elaborated obligation for story/ac/kind
// naming the test producer ref, authoritative for the CI job named job.
func writeTestProducerObligation(t *testing.T, root, story, ac, kind, ref, job string) {
	t.Helper()
	writeObligation(t, root, story, ac, kind, obligationMD(story, ac, kind, obligationQualityInput{
		State: "elaborated", ProducerKind: "test", ProducerRef: ref, SourceKind: "ci-job", SourceRef: job,
	}))
}

// rerunProv is the provenance of attempt job of the verify job at rerunCommit.
func rerunProv(job string) artifact.EvidenceProvenance {
	return artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: job, JobName: "verify", Commit: rerunCommit}
}

// produceAttempt runs one production attempt of the verify job at rerunCommit
// with runner, fails the test on an error, and returns what it printed.
func produceAttempt(t *testing.T, root, job string, runner namedGoTestRunner) string {
	t.Helper()
	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, rerunCommit, "verify", runner, rerunProv(job), &stdout); err != nil {
		t.Fatalf("attempt %s: produceGoTestEvidence: %v", job, err)
	}
	return stdout.String()
}

// streamRunner is a runner whose every package prints the canned stream.
func streamRunner(streams map[string][]byte) *fakeNamedGoTestRunner {
	return &fakeNamedGoTestRunner{output: streams}
}

// buildFailedStream is a go test -json stream in which relPkg's test binary
// failed to build, so none of its tests ran.
func buildFailedStream(relPkg string) []byte {
	pkg := fakeModulePath + "/" + relPkg
	testBin := pkg + " [" + pkg + ".test]"
	return []byte(`{"ImportPath":"` + testBin + `","Action":"build-fail"}` + "\n" +
		`{"Action":"start","Package":"` + pkg + `"}` + "\n" +
		`{"Action":"fail","Package":"` + pkg + `","FailedBuild":"` + testBin + `"}` + "\n")
}

// unloadedStream is a go test -json stream in which the go command could not
// load ./relPkg at all (a renamed or removed directory).
func unloadedStream(relPkg string) []byte {
	arg := "./" + relPkg
	return []byte(`{"ImportPath":"` + arg + `","Action":"build-fail"}` + "\n" +
		`{"Action":"start","Package":"` + arg + `"}` + "\n" +
		`{"Action":"fail","Package":"` + arg + `","FailedBuild":"` + arg + `"}` + "\n")
}

// verdictsPath is spec ref's verdicts.json at rerunCommit.
func verdictsPath(root, specRef string) string {
	return filepath.Join(store.DerivedSpecDir(root, store.RefSlug(specRef)), rerunCommit, "verdicts.json")
}

// rawRecords splits the verdicts.json at path into its records' raw bytes, so
// a kept record can be compared byte for byte.
func rawRecords(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	out := make([]string, len(raw))
	for i, r := range raw {
		out[i] = string(r)
	}
	return out
}

// assertCanonicalVerdicts fails unless the file at path is exactly the
// canonical JSON (sorted keys, trailing newline) of its own decoded records.
func assertCanonicalVerdicts(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	recs, err := readExistingEvidenceRecords(path)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	want, err := canonjson.Marshal(recs)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Errorf("%s is not canonical JSON:\n got %q\nwant %q", path, data, want)
	}
}

// snapshotDerived reads every file under root's derived tree, keyed by its
// path relative to root.
func snapshotDerived(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	derived := filepath.Join(root, ".verdi", "data", "derived")
	err := filepath.WalkDir(derived, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotting %s: %v", derived, err)
	}
	return out
}

// recordsFor returns the records in recs whose producer is ref.
func recordsFor(recs []artifact.Evidence, ref string) []artifact.Evidence {
	var out []artifact.Evidence
	for _, r := range recs {
		if r.Producer == ref {
			out = append(out, r)
		}
	}
	return out
}

// withdrawnPass is the note a disclosure carries when a run withdrew one
// earlier pass record for the obligation's producer at this commit.
const withdrawnPass = "the earlier pass record for this producer at this commit was withdrawn"

// TestProduceGoTestEvidence_RerunAtSameCommit proves what a second attempt at
// the same commit leaves for a producer whose first attempt recorded a pass.
// A pass, fail, or abstain replaces the earlier record with the new attempt's
// own. A test that did not run, a package that did not build or load, and a
// store root with no go.mod each withdraw the earlier pass: no record is left
// and the disclosure says so. Every write stays canonical JSON.
func TestProduceGoTestEvidence_RerunAtSameCommit(t *testing.T) {
	const ref = "go-test:pkg/a:TestA"
	cases := []struct {
		name        string
		stream      []byte // attempt 2's stream for ./pkg/a
		removeGoMod bool   // attempt 2 runs in a store root with no go.mod
		wantVerdict artifact.EvidenceVerdict
		wantWhy     string // "" when a record is wanted
	}{
		{name: "pass to pass", stream: testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass}), wantVerdict: artifact.VerdictPass},
		{name: "pass to fail", stream: testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionFail}), wantVerdict: artifact.VerdictFail},
		{name: "pass to abstain", stream: testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionSkip}), wantVerdict: artifact.VerdictAbstain},
		{name: "pass to did not run", stream: testGoTestJSON("pkg/a", map[string]string{"TestOther": gotestjson.ActionPass}), wantWhy: whyAbsent},
		{name: "pass to package failed to build", stream: buildFailedStream("pkg/a"), wantWhy: "it failed to build"},
		{name: "pass to package could not be loaded", stream: unloadedStream("pkg/a"), wantWhy: "the go command could not load it"},
		{name: "pass to no go.mod", removeGoMod: true, wantWhy: "has no go.mod"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeGoMod(t, root)
			writeTestProducerObligation(t, root, "story-a", "ac-1", "behavioral", ref, "verify")
			produceAttempt(t, root, "1", streamRunner(map[string][]byte{
				"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass}),
			}))
			if got := recordsFor(readVerdicts(t, root, "spec/story-a", rerunCommit), ref); len(got) != 1 || got[0].Verdict != artifact.VerdictPass {
				t.Fatalf("attempt 1 records for %s = %+v, want one pass", ref, got)
			}

			if c.removeGoMod {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
			}
			stdout := produceAttempt(t, root, "2", streamRunner(map[string][]byte{"./pkg/a": c.stream}))

			got := recordsFor(readVerdicts(t, root, "spec/story-a", rerunCommit), ref)
			line := disclosureLineFor(stdout, "obligation/story-a--ac-1--behavioral")
			if c.wantWhy == "" {
				if len(got) != 1 || got[0].Verdict != c.wantVerdict || got[0].Provenance.Job != "2" {
					t.Errorf("records for %s = %+v, want exactly attempt 2's %s record", ref, got, c.wantVerdict)
				}
				if line != "" || strings.Contains(stdout, "withdrawn") {
					t.Errorf("stdout = %q, want no disclosure for a recorded producer", stdout)
				}
			} else {
				if len(got) != 0 {
					t.Errorf("records for %s = %+v, want none: attempt 2 did not run the test", ref, got)
				}
				for _, want := range []string{"[" + goTestProducerAbsentSource + "]", c.wantWhy, withdrawnPass} {
					if !strings.Contains(line, want) {
						t.Errorf("disclosure = %q, want it to contain %q; stdout=%q", line, want, stdout)
					}
				}
			}
			assertCanonicalVerdicts(t, verdictsPath(root, "spec/story-a"))
		})
	}
}

// TestProduceGoTestEvidence_MixedRerun proves a rerun replaces each selected
// producer on its own: attempt 1 passes both of a spec's named tests; attempt
// 2 passes one and does not run the other. The present producer then holds
// attempt 2's record and still matches its obligation; the absent one has no
// record, reads producer-missing, and is the only one disclosed.
func TestProduceGoTestEvidence_MixedRerun(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	const refA, refB = "go-test:pkg/a:TestA", "go-test:pkg/a:TestB"
	writeTestProducerObligation(t, root, "story-a", "ac-1", "behavioral", refA, "verify")
	writeTestProducerObligation(t, root, "story-a", "ac-2", "behavioral", refB, "verify")

	produceAttempt(t, root, "1", streamRunner(map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass, "TestB": gotestjson.ActionPass}),
	}))
	stdout := produceAttempt(t, root, "2", streamRunner(map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass}),
	}))

	recs := readVerdicts(t, root, "spec/story-a", rerunCommit)
	if len(recs) != 1 || recs[0].Producer != refA || recs[0].Verdict != artifact.VerdictPass || recs[0].Provenance.Job != "2" {
		t.Fatalf("records = %+v, want exactly attempt 2's pass for %s", recs, refA)
	}
	if line := disclosureLineFor(stdout, "obligation/story-a--ac-1--behavioral"); line != "" {
		t.Errorf("ac-1 was recorded, yet disclosed: %q", line)
	}
	if line := disclosureLineFor(stdout, "obligation/story-a--ac-2--behavioral"); !strings.Contains(line, whyAbsent) || !strings.Contains(line, withdrawnPass) {
		t.Errorf("ac-2 disclosure = %q, want it to say the test did not run and %q", line, withdrawnPass)
	}

	ctx := context.Background()
	for _, c := range []struct {
		ac         string
		rec        *artifact.Evidence
		wantState  evidence.ObligationMatchState
		wantReason evidence.ObligationMatchReason
	}{
		{"ac-1", &recs[0], evidence.ObligationMatched, ""},
		{"ac-2", nil, evidence.ObligationUnproven, evidence.ObligationReasonProducerMissing},
	} {
		got, err := evidence.AssessObligation(ctx, evidence.ObligationAssessmentInput{
			StoreRoot: root, SpecName: "story-a", ACID: c.ac, Kind: artifact.EvidenceBehavioral,
			Record: c.rec, EvaluationCommit: rerunCommit,
		})
		if err != nil {
			t.Fatalf("AssessObligation(%s): %v", c.ac, err)
		}
		if got.MatchState != c.wantState || got.Reason != c.wantReason {
			t.Errorf("AssessObligation(%s) = (%q, %q), want (%q, %q)", c.ac, got.MatchState, got.Reason, c.wantState, c.wantReason)
		}
	}
}

// TestProduceGoTestEvidence_RerunPreservesUnmanagedRecords proves a rerun that
// withdraws one producer and replaces another touches nothing outside its
// managed subset. In the rewritten spec, the coarse verdi-verify-* records, an
// unrelated producer's record, and another CI job's go-test record stay byte
// for byte, in order, ahead of the new record; runtime.json beside it is
// untouched. A second spec with no obligation selected for this job keeps its
// verdicts.json byte for byte, even though it holds a record for a producer
// this run withdrew from the first spec.
func TestProduceGoTestEvidence_RerunPreservesUnmanagedRecords(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	const refA, refB, refLint = "go-test:pkg/a:TestA", "go-test:pkg/a:TestB", "go-test:pkg/a:TestLint"
	writeTestProducerObligation(t, root, "story-a", "ac-1", "behavioral", refA, "verify")
	writeTestProducerObligation(t, root, "story-a", "ac-2", "behavioral", refB, "verify")
	// Another CI job's obligations: never selected for verify.
	writeTestProducerObligation(t, root, "story-a", "ac-3", "behavioral", refLint, "lint")
	writeTestProducerObligation(t, root, "story-b", "ac-1", "behavioral", refA, "lint")

	lintProv := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "5", JobName: "lint", Commit: rerunCommit}
	seed := []artifact.Evidence{
		{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-1", "ac-2"}, Kind: artifact.EvidenceStatic, Verdict: artifact.VerdictPass, Witness: "make verify: build + vet clean", Producer: selfHostedStaticProducer, Provenance: rerunProv("1"), Digest: "sha256:" + strings.Repeat("1", 64)},
		{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-1"}, Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictPass, Witness: "make verify: go test + e2e passed", Producer: selfHostedBehavioralProducer, Provenance: rerunProv("1"), Digest: "sha256:" + strings.Repeat("2", 64)},
		{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-2"}, Kind: artifact.EvidenceStatic, Verdict: artifact.VerdictFail, Witness: "unrelated checker", Producer: "checker:vet", Provenance: rerunProv("1"), Digest: "sha256:" + strings.Repeat("3", 64)},
		{Schema: "verdi.evidence/v1", EvidenceFor: []string{"ac-3"}, Kind: artifact.EvidenceBehavioral, Verdict: artifact.VerdictPass, Witness: "lint job", Producer: refLint, Provenance: lintProv, Digest: "sha256:" + strings.Repeat("4", 64)},
	}
	seedBytes, err := canonjson.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	aPath := verdictsPath(root, "spec/story-a")
	if err := os.MkdirAll(filepath.Dir(aPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, seedBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	seedRaw := rawRecords(t, aPath)
	runtimePath := filepath.Join(filepath.Dir(aPath), "runtime.json")
	runtimeBytes := `[{"digest":"sha256:` + strings.Repeat("5", 64) + `","evidence_for":["ac-1"],"kind":"runtime","producer":"runtime:story-a#ac-1","provenance":{"commit":"` + rerunCommit + `","source":"ci"},"schema":"verdi.evidence/v1","verdict":"pass","witness":"probe"}]` + "\n"
	if err := os.WriteFile(runtimePath, []byte(runtimeBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	// story-b's file is deliberately not canonical: a rewrite would show.
	bPath := verdictsPath(root, "spec/story-b")
	bBytes := "[\n  {\"schema\": \"verdi.evidence/v1\", \"evidence_for\": [\"ac-1\"], \"kind\": \"behavioral\", \"verdict\": \"pass\", \"witness\": \"lint job\", \"producer\": \"" + refA + "\", \"provenance\": {\"source\": \"ci\", \"pipeline\": \"913\", \"job\": \"5\", \"job_name\": \"lint\", \"commit\": \"" + rerunCommit + "\"}, \"digest\": \"sha256:" + strings.Repeat("6", 64) + "\"}\n]\n"
	if err := os.MkdirAll(filepath.Dir(bPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte(bBytes), 0o644); err != nil {
		t.Fatal(err)
	}

	produceAttempt(t, root, "1", streamRunner(map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass, "TestB": gotestjson.ActionPass}),
	}))
	stdout := produceAttempt(t, root, "2", streamRunner(map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestB": gotestjson.ActionFail}),
	}))
	if line := disclosureLineFor(stdout, "obligation/story-a--ac-1--behavioral"); !strings.Contains(line, withdrawnPass) {
		t.Errorf("ac-1 disclosure = %q, want it to contain %q", line, withdrawnPass)
	}

	gotRaw := rawRecords(t, aPath)
	if len(gotRaw) != len(seedRaw)+1 {
		t.Fatalf("story-a records = %q, want the %d seeded records then attempt 2's %s record", gotRaw, len(seedRaw), refB)
	}
	for i, want := range seedRaw {
		if gotRaw[i] != want {
			t.Errorf("story-a record %d = %s, want it kept byte for byte: %s", i, gotRaw[i], want)
		}
	}
	recs := readVerdicts(t, root, "spec/story-a", rerunCommit)
	if last := recs[len(recs)-1]; last.Producer != refB || last.Verdict != artifact.VerdictFail || last.Provenance.Job != "2" {
		t.Errorf("story-a's last record = %+v, want attempt 2's fail for %s", last, refB)
	}
	if got := recordsFor(recs, refA); len(got) != 0 {
		t.Errorf("story-a records for %s = %+v, want none", refA, got)
	}
	assertCanonicalVerdicts(t, aPath)
	for path, want := range map[string]string{runtimePath: runtimeBytes, bPath: bBytes} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s changed:\n got %q\nwant %q", path, got, want)
		}
	}
}

// TestProduceGoTestEvidence_AbsentWithoutEarlierRecordLeavesFileUntouched
// proves a selected producer that did not run, and had no record at this
// commit, withdraws nothing: its spec's verdicts.json is not rewritten, and the
// disclosure says only that the test did not run.
func TestProduceGoTestEvidence_AbsentWithoutEarlierRecordLeavesFileUntouched(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root)
	writeTestProducerObligation(t, root, "story-a", "ac-1", "behavioral", "go-test:pkg/a:TestA", "verify")
	path := verdictsPath(root, "spec/story-a")
	// Deliberately not canonical: a rewrite would show.
	existing := "[ {\"schema\":\"verdi.evidence/v1\",\"evidence_for\":[\"ac-1\"],\"kind\":\"behavioral\",\"verdict\":\"pass\",\"witness\":\"w\",\"producer\":\"verdi-verify-behavioral\",\"provenance\":{\"source\":\"ci\",\"commit\":\"" + rerunCommit + "\"},\"digest\":\"sha256:" + strings.Repeat("a", 64) + "\"} ]"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout := produceAttempt(t, root, "1", streamRunner(map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestOther": gotestjson.ActionPass}),
	}))
	if line := disclosureLineFor(stdout, "obligation/story-a--ac-1--behavioral"); !strings.Contains(line, whyAbsent) || strings.Contains(line, "withdrawn") {
		t.Errorf("disclosure = %q, want it to say the test did not run and nothing about a withdrawal", line)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != existing {
		t.Errorf("verdicts.json changed:\n got %q\nwant %q", got, existing)
	}
}

// TestProduceGoTestEvidence_OperationalErrorWritesNothing proves an
// operational error during a rerun leaves every file in the derived tree byte
// for byte as the previous attempt left it: a malformed stream for the second
// package, after the first package's stream parsed and would have changed a
// record; or an existing verdicts.json that cannot be decoded, for the spec
// sorted after one whose record would have changed.
func TestProduceGoTestEvidence_OperationalErrorWritesNothing(t *testing.T) {
	cases := []struct {
		name       string
		streamB    []byte // attempt 2's stream for ./pkg/b
		corruptB   bool   // story-b's verdicts.json is not JSON before attempt 2
		wantErrSub string
	}{
		{name: "malformed stream for one package", streamB: []byte("{\"Action\":\"start\",\"Package\":\"" + fakeModulePath + "/pkg/b\"}\nnot json\n"), wantErrSub: "go-test producer"},
		{name: "undecodable existing verdicts.json", streamB: testGoTestJSON("pkg/b", map[string]string{"TestOther": gotestjson.ActionPass}), corruptB: true, wantErrSub: "verdicts.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeGoMod(t, root)
			writeTestProducerObligation(t, root, "story-a", "ac-1", "behavioral", "go-test:pkg/a:TestA", "verify")
			writeTestProducerObligation(t, root, "story-b", "ac-1", "behavioral", "go-test:pkg/b:TestB", "verify")
			produceAttempt(t, root, "1", streamRunner(map[string][]byte{
				"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass}),
				"./pkg/b": testGoTestJSON("pkg/b", map[string]string{"TestB": gotestjson.ActionPass}),
			}))
			if c.corruptB {
				if err := os.WriteFile(verdictsPath(root, "spec/story-b"), []byte("not json\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotDerived(t, root)
			if len(before) != 2 {
				t.Fatalf("derived tree before attempt 2 = %v, want both specs' verdicts.json", before)
			}

			runner := streamRunner(map[string][]byte{
				"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionFail}),
				"./pkg/b": c.streamB,
			})
			var stdout bytes.Buffer
			err := produceGoTestEvidence(context.Background(), root, rerunCommit, "verify", runner, rerunProv("2"), &stdout)
			if err == nil || !strings.Contains(err.Error(), c.wantErrSub) {
				t.Fatalf("attempt 2 err = %v, want an operational error containing %q", err, c.wantErrSub)
			}
			after := snapshotDerived(t, root)
			if len(after) != len(before) {
				t.Errorf("derived tree after the error = %v, want exactly %v", after, before)
			}
			for path, want := range before {
				if after[path] != want {
					t.Errorf("%s changed on an operational error:\n got %q\nwant %q", path, after[path], want)
				}
			}
		})
	}
}

// TestGoTestManagedSubset proves a run's managed subset is, per owning spec
// ref, the producer ref of every selected obligation: two obligations sharing
// one ref name it once, and a spec with no selected obligation has no entry.
func TestGoTestManagedSubset(t *testing.T) {
	sel := func(spec, ac, ref string) selectedGoTestObligation {
		return selectedGoTestObligation{testProducerCandidate: testProducerCandidate{SpecName: spec, ACID: ac, ProducerRef: ref}}
	}
	cases := []struct {
		name     string
		selected []selectedGoTestObligation
		want     map[string]map[string]bool
	}{
		{"nothing selected", nil, map[string]map[string]bool{}},
		{"one spec, two refs", []selectedGoTestObligation{sel("a", "ac-1", "go-test:p:TestA"), sel("a", "ac-2", "go-test:p:TestB")},
			map[string]map[string]bool{"spec/a": {"go-test:p:TestA": true, "go-test:p:TestB": true}}},
		{"a shared ref, two specs", []selectedGoTestObligation{sel("a", "ac-1", "go-test:p:TestA"), sel("a", "ac-2", "go-test:p:TestA"), sel("b", "ac-1", "go-test:p:TestA")},
			map[string]map[string]bool{"spec/a": {"go-test:p:TestA": true}, "spec/b": {"go-test:p:TestA": true}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := goTestManagedSubset(c.selected); !reflect.DeepEqual(got, c.want) {
				t.Errorf("goTestManagedSubset = %v, want %v", got, c.want)
			}
		})
	}
}

// TestDiscloseGoTestAbsence proves an absence's disclosure names each earlier
// record this run withdrew for its own producer in its own spec, and is left
// exactly as detected when nothing of its own was withdrawn.
func TestDiscloseGoTestAbsence(t *testing.T) {
	const ref = "go-test:pkg/a:TestA"
	s := selectedGoTestObligation{
		testProducerCandidate: testProducerCandidate{SpecName: "story-a", ACID: "ac-1", Kind: artifact.EvidenceBehavioral, ProducerRef: ref, ObligationID: "obligation/story-a--ac-1--behavioral"},
		Package:               "pkg/a", Test: "TestA",
	}
	a := goTestAbsence{obligation: s, disclosed: goTestProducerAbsentDisclosure(s)}
	rec := func(producer string, v artifact.EvidenceVerdict) artifact.Evidence {
		return artifact.Evidence{Producer: producer, Verdict: v}
	}
	cases := []struct {
		name      string
		withdrawn map[string][]artifact.Evidence
		wantNote  string // "": the disclosure is unchanged
	}{
		{"nothing withdrawn", nil, ""},
		{"another producer's record withdrawn", map[string][]artifact.Evidence{"spec/story-a": {rec("go-test:pkg/a:TestB", artifact.VerdictPass)}}, ""},
		{"the same producer's record withdrawn in another spec", map[string][]artifact.Evidence{"spec/story-b": {rec(ref, artifact.VerdictPass)}}, ""},
		{"one earlier pass", map[string][]artifact.Evidence{"spec/story-a": {rec(ref, artifact.VerdictPass)}}, "; " + withdrawnPass},
		{"one earlier abstain", map[string][]artifact.Evidence{"spec/story-a": {rec(ref, artifact.VerdictAbstain)}}, "; the earlier abstain record for this producer at this commit was withdrawn"},
		{"two earlier records", map[string][]artifact.Evidence{"spec/story-a": {rec(ref, artifact.VerdictPass), rec("go-test:pkg/a:TestB", artifact.VerdictFail), rec(ref, artifact.VerdictFail)}},
			"; the 2 earlier records for this producer at this commit (pass, fail) were withdrawn"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := discloseGoTestAbsence(a, c.withdrawn)
			want := a.disclosed
			if c.wantNote != "" {
				want = disclosure.New(a.disclosed.Source, a.disclosed.Scope, a.disclosed.Text+c.wantNote)
			}
			if got != want {
				t.Errorf("discloseGoTestAbsence = %+v, want %+v", got, want)
			}
		})
	}
}
