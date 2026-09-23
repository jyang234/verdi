package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
)

// The go-test producer's hermetic integration tests: they exec the REAL local
// `go` toolchain against the tiny, dependency-free fixture module under
// testdata/gotestfixture/ (its own go.mod, so the parent module's ./... never
// builds it). No network: hermeticGoEnv pins the child go command to the local
// toolchain with the module proxy off.

// hermeticGoEnv pins every child `go` invocation of this test to the local
// toolchain, no module proxy, and no workspace file, so the real-exec path
// can never reach the network or pick up a developer's GOFLAGS.
func hermeticGoEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
}

// copyGoTestFixture copies the fixture module into a fresh temp dir that is
// both the store root and the Go module root, so the production path can
// write obligations and derived records there without touching testdata/.
func copyGoTestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/gotestfixture")); err != nil {
		t.Fatalf("copying the go-test fixture module: %v", err)
	}
	return root
}

// realToolchainCase is one obligation the real-toolchain test authors, and
// the outcome the production path must reach for it: a record with
// wantVerdict, or (wantVerdict == "") no record and a disclosure naming it
// whose rendered line contains wantWhy.
type realToolchainCase struct {
	ac, kind, ref string
	wantVerdict   artifact.EvidenceVerdict
	wantWhy       string
}

const (
	whyAbsent    = "did not run (no terminal event)"
	whyNotBuilt  = "did not build or load"
	whyMalformed = "does not match the go-test:<package>:<TopLevelTestName> grammar"
)

// TestProduceGoTestEvidence_RealToolchain drives the REAL production path —
// produceGoTestEvidence with realNamedGoTestRunner, exactly as runProduce
// calls it — against the fixture module, and asserts what lands on disk:
// test2json reports every event's Package as the full import path
// (cmd/go/internal/test: test2json.NewConverter(..., p.ImportPath, ...)), so a
// passing, failing, and skipped named test must each reach its own record,
// and a named test that does not exist must reach none.
func TestProduceGoTestEvidence_RealToolchain(t *testing.T) {
	hermeticGoEnv(t)
	root := copyGoTestFixture(t)
	const story = "story-real"
	const commit = "abababababababababababababababababababab"

	cases := []realToolchainCase{
		{"ac-1", "behavioral", "go-test:sample:TestPass", artifact.VerdictPass, ""},
		{"ac-2", "behavioral", "go-test:sample:TestFail", artifact.VerdictFail, ""},
		{"ac-3", "static", "go-test:sample:TestSkip", artifact.VerdictAbstain, ""},
		{"ac-4", "behavioral", "go-test:sample:TestAbsent", "", whyAbsent},
		// Go 1.25 "attr" events (Key, Value) are part of a passing test's stream.
		{"ac-5", "behavioral", "go-test:sample:TestAttr", artifact.VerdictPass, ""},
		// The parent's own terminal fail decides, not its passing subtest's.
		{"ac-6", "behavioral", "go-test:sample:TestParentFails", artifact.VerdictFail, ""},
		// No test named TestPrefix exists; TestPrefixBar's pass is not its.
		{"ac-7", "behavioral", "go-test:sample:TestPrefix", "", whyAbsent},
		{"ac-8", "behavioral", "go-test:sample:TestPrefixBar", artifact.VerdictPass, ""},
		// An unbuildable package (build-output, build-fail, fail+FailedBuild)
		// means its named test did not run.
		{"ac-9", "behavioral", "go-test:nobuild:TestNeverBuilds", "", whyNotBuilt},
		// A renamed or removed package directory: the go command reports the
		// unresolved argument ./gone as Package, and nothing runs.
		{"ac-10", "behavioral", "go-test:gone:TestGone", "", whyNotBuilt},
		// A malformed sibling ref (a regexp metacharacter) is disclosed on its
		// own obligation only; it must not change TestPass's outcome above.
		{"ac-11", "behavioral", "go-test:sample:Test(", "", whyMalformed},
		// A package path into a nested module is malformed, never run.
		{"ac-12", "behavioral", "go-test:nested/inner:TestInner", "", whyMalformed},
	}
	for _, c := range cases {
		writeObligation(t, root, story, c.ac, c.kind, obligationMD(story, c.ac, c.kind, obligationQualityInput{
			State: "elaborated", ProducerKind: "test", ProducerRef: c.ref,
			SourceKind: "ci-job", SourceRef: "verify",
		}))
	}

	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "7", JobName: "verify", Commit: commit}
	var stdout bytes.Buffer
	if err := produceGoTestEvidence(context.Background(), root, commit, "verify", realNamedGoTestRunner{}, prov, &stdout); err != nil {
		t.Fatalf("produceGoTestEvidence through the real toolchain: %v", err)
	}

	byProducer := map[string]artifact.Evidence{}
	for _, r := range readVerdicts(t, root, "spec/"+story, commit) {
		byProducer[r.Producer] = r
	}
	for _, c := range cases {
		rec, ok := byProducer[c.ref]
		if c.wantVerdict == "" {
			if ok {
				t.Errorf("%s: got record %+v, want none (the named test does not exist)", c.ref, rec)
			}
			id := "obligation/" + story + "--" + c.ac + "--" + c.kind
			if line := disclosureLineFor(stdout.String(), id); !strings.Contains(line, c.wantWhy) {
				t.Errorf("%s: disclosure for %s = %q, want one containing %q", c.ref, id, line, c.wantWhy)
			}
			continue
		}
		if !ok {
			t.Errorf("%s: no record; stdout=%q", c.ref, stdout.String())
			continue
		}
		if rec.Verdict != c.wantVerdict || string(rec.Kind) != c.kind || len(rec.EvidenceFor) != 1 || rec.EvidenceFor[0] != c.ac {
			t.Errorf("%s: record = %+v, want verdict %s kind %s evidence_for [%s]", c.ref, rec, c.wantVerdict, c.kind, c.ac)
		}
	}

	// The passing test's record satisfies its obligation through the real
	// matcher; the failing test's record violates, never matches.
	for _, c := range []struct {
		ac, ref string
		want    evidence.ObligationMatchState
	}{
		{"ac-1", "go-test:sample:TestPass", evidence.ObligationMatched},
		{"ac-2", "go-test:sample:TestFail", evidence.ObligationViolatedWithWitness},
	} {
		rec := byProducer[c.ref]
		got, err := evidence.AssessObligation(context.Background(), evidence.ObligationAssessmentInput{
			StoreRoot: root, SpecName: story, ACID: c.ac, Kind: artifact.EvidenceBehavioral,
			Record: &rec, EvaluationCommit: commit,
		})
		if err != nil {
			t.Fatalf("AssessObligation(%s): %v", c.ac, err)
		}
		if got.MatchState != c.want {
			t.Errorf("AssessObligation(%s) = %q, want %q (reason %q)", c.ac, got.MatchState, c.want, got.Reason)
		}
	}
}

// disclosureLineFor returns the one stdout line that names obligation id
// (followed by the ": " a rendered disclosure puts after its subject), or "".
func disclosureLineFor(stdout, id string) string {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, " "+id+": ") {
			return line
		}
	}
	return ""
}

// TestProduceGoTestEvidence_RealToolchain_RerunWithdrawsAbsentPass is the
// owner risk-gate review's F1 reproduction, made permanent (SI-238). Attempt
// 1 at commit C records a pass for go-test:sample:TestPass through the real
// toolchain. Attempt 2 runs at the same C, in the same derived tree, and
// inherits an ordinary GOFLAGS=-skip=^TestPass$, so the real go test runs
// nothing and the named test did not run. Attempt 2 must leave no record for
// that producer, so the obligation reads producer-missing and never matched,
// and its disclosure must say the earlier pass was withdrawn.
func TestProduceGoTestEvidence_RealToolchain_RerunWithdrawsAbsentPass(t *testing.T) {
	hermeticGoEnv(t)
	root := copyGoTestFixture(t)
	const story = "story-real"
	const commit = "abababababababababababababababababababab"
	const ref = "go-test:sample:TestPass"
	const obligationID = "obligation/" + story + "--ac-1--behavioral"
	writeObligation(t, root, story, "ac-1", "behavioral", obligationMD(story, "ac-1", "behavioral", obligationQualityInput{
		State: "elaborated", ProducerKind: "test", ProducerRef: ref, SourceKind: "ci-job", SourceRef: "verify",
	}))
	ctx := context.Background()
	assess := func(rec *artifact.Evidence) evidence.ObligationAssessment {
		t.Helper()
		got, err := evidence.AssessObligation(ctx, evidence.ObligationAssessmentInput{
			StoreRoot: root, SpecName: story, ACID: "ac-1", Kind: artifact.EvidenceBehavioral,
			Record: rec, EvaluationCommit: commit,
		})
		if err != nil {
			t.Fatalf("AssessObligation: %v", err)
		}
		return got
	}

	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "1", JobName: "verify", Commit: commit}
	var first bytes.Buffer
	if err := produceGoTestEvidence(ctx, root, commit, "verify", realNamedGoTestRunner{}, prov, &first); err != nil {
		t.Fatalf("attempt 1: %v", err)
	}
	before := readVerdicts(t, root, "spec/"+story, commit)
	if len(before) != 1 || before[0].Producer != ref || before[0].Verdict != artifact.VerdictPass {
		t.Fatalf("attempt 1 records = %+v, want one pass for %s", before, ref)
	}
	if got := assess(&before[0]); got.MatchState != evidence.ObligationMatched {
		t.Fatalf("attempt 1's pass assesses %q (reason %q), want matched", got.MatchState, got.Reason)
	}

	t.Setenv("GOFLAGS", "-skip=^TestPass$")
	prov.Job = "2"
	var second bytes.Buffer
	if err := produceGoTestEvidence(ctx, root, commit, "verify", realNamedGoTestRunner{}, prov, &second); err != nil {
		t.Fatalf("attempt 2: %v", err)
	}

	after := readVerdicts(t, root, "spec/"+story, commit)
	for i := range after {
		if after[i].Producer == ref {
			t.Errorf("attempt %s's %s record for %s survived attempt 2, which did not run the test", after[i].Provenance.Job, after[i].Verdict, ref)
		}
		if got := assess(&after[i]); got.MatchState == evidence.ObligationMatched {
			t.Errorf("record %+v still matches %s after attempt 2 disclosed the test did not run", after[i], obligationID)
		}
	}
	if got := assess(nil); got.MatchState != evidence.ObligationUnproven || got.Reason != evidence.ObligationReasonProducerMissing {
		t.Errorf("with no current record, %s assesses (%q, %q), want (unproven, producer-missing)", obligationID, got.MatchState, got.Reason)
	}
	line := disclosureLineFor(second.String(), obligationID)
	for _, want := range []string{"[" + goTestProducerAbsentSource + "]", whyAbsent, "the earlier pass record for this producer at this commit was withdrawn"} {
		if !strings.Contains(line, want) {
			t.Errorf("attempt 2's disclosure for %s = %q, want it to contain %q; stdout=%q", obligationID, line, want, second.String())
		}
	}
}
