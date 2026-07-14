package runtimeprobe

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const testCommit = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

// TestEmit_Happy proves Emit builds a well-formed, (story, AC)-keyed,
// source: ci record when run inside a genuine, non-overridden CI
// environment (ac-1's behavioral evidence: "a probe run produces a
// well-formed (story, AC)-keyed record").
func TestEmit_Happy(t *testing.T) {
	rec, err := Emit(ProbeInput{
		StoryRef: "jira:VERDI-3",
		ACID:     "ac-2",
		Verdict:  artifact.VerdictPass,
		Witness:  "GET /healthz -> 200",
		Commit:   testCommit,
		Pipeline: "913",
		Job:      "7",
		InCI:     true,
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Emit produced a record that fails self-validation: %v", err)
	}
	if rec.Kind != artifact.EvidenceRuntime {
		t.Errorf("Kind = %q, want %q", rec.Kind, artifact.EvidenceRuntime)
	}
	if len(rec.EvidenceFor) != 1 || rec.EvidenceFor[0] != "ac-2" {
		t.Errorf("EvidenceFor = %v, want [ac-2]", rec.EvidenceFor)
	}
	if rec.Verdict != artifact.VerdictPass {
		t.Errorf("Verdict = %q, want pass", rec.Verdict)
	}
	if rec.Producer != CheckID("jira:VERDI-3", "ac-2") {
		t.Errorf("Producer = %q, want %q", rec.Producer, CheckID("jira:VERDI-3", "ac-2"))
	}
	if rec.Provenance.Source != artifact.SourceCI {
		t.Errorf("Provenance.Source = %q, want ci", rec.Provenance.Source)
	}
	if rec.Provenance.Pipeline != "913" || rec.Provenance.Job != "7" || rec.Provenance.Commit != testCommit {
		t.Errorf("Provenance = %+v, want pipeline=913 job=7 commit=%s", rec.Provenance, testCommit)
	}
	if !strings.HasPrefix(rec.Digest, "sha256:") {
		t.Errorf("Digest = %q, want sha256:<hex> form", rec.Digest)
	}
}

// TestEmit_LocalRunStampsSourceLocal proves a probe run outside a detected
// CI environment stamps source: local, never source: ci (dc-3: "a local
// probe run stamps source: local").
func TestEmit_LocalRunStampsSourceLocal(t *testing.T) {
	rec, err := Emit(ProbeInput{
		StoryRef: "jira:VERDI-3",
		ACID:     "ac-2",
		Verdict:  artifact.VerdictPass,
		Witness:  "manual local check",
		Commit:   testCommit,
		InCI:     false,
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if rec.Provenance.Source != artifact.SourceLocal {
		t.Errorf("Provenance.Source = %q, want local", rec.Provenance.Source)
	}
}

// TestEmit_ForceLocalOverridesCI proves an explicit ForceLocal override
// stamps source: local EVEN when InCI is true — mirroring sync.go's
// runProduce --force-local precedent exactly: no local invocation may ever
// emit a source: ci record just by claiming CI is detected.
func TestEmit_ForceLocalOverridesCI(t *testing.T) {
	rec, err := Emit(ProbeInput{
		StoryRef:   "jira:VERDI-3",
		ACID:       "ac-2",
		Verdict:    artifact.VerdictPass,
		Witness:    "forced local run for testing",
		Commit:     testCommit,
		InCI:       true,
		ForceLocal: true,
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if rec.Provenance.Source != artifact.SourceLocal {
		t.Errorf("Provenance.Source = %q, want local (ForceLocal must override InCI)", rec.Provenance.Source)
	}
}

// TestEmit_FailVerdict proves Emit stamps whatever verdict the probe
// actually observed — a failing check produces a verdict: fail record, not
// a silently-dropped or coerced one (dc-3: no fabrication in either
// direction).
func TestEmit_FailVerdict(t *testing.T) {
	rec, err := Emit(ProbeInput{
		StoryRef: "jira:VERDI-3",
		ACID:     "ac-2",
		Verdict:  artifact.VerdictFail,
		Witness:  "GET /healthz -> 500",
		Commit:   testCommit,
		InCI:     true,
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if rec.Verdict != artifact.VerdictFail {
		t.Errorf("Verdict = %q, want fail", rec.Verdict)
	}
}

// TestEmit_Negative proves Emit refuses a malformed input rather than
// silently emitting a half-formed record.
func TestEmit_Negative(t *testing.T) {
	base := ProbeInput{StoryRef: "jira:VERDI-3", ACID: "ac-2", Verdict: artifact.VerdictPass, Witness: "w", Commit: testCommit, InCI: true}

	t.Run("missing story ref", func(t *testing.T) {
		in := base
		in.StoryRef = ""
		if _, err := Emit(in); err == nil {
			t.Fatal("Emit with empty StoryRef: want error, got nil")
		}
	})
	t.Run("missing ac id", func(t *testing.T) {
		in := base
		in.ACID = ""
		if _, err := Emit(in); err == nil {
			t.Fatal("Emit with empty ACID: want error, got nil")
		}
	})
	t.Run("missing witness", func(t *testing.T) {
		in := base
		in.Witness = ""
		if _, err := Emit(in); err == nil {
			t.Fatal("Emit with empty Witness: want error, got nil")
		}
	})
	t.Run("malformed ac id shape", func(t *testing.T) {
		in := base
		in.ACID = "not-an-ac-id"
		if _, err := Emit(in); err == nil {
			t.Fatal("Emit with malformed ACID: want error (rec.Validate() rejects it), got nil")
		}
	})
}

// evidenceRecord is a small test helper building a minimal, valid runtime
// record for a given (story, AC), bypassing Emit so Query's own tests stay
// independent of Emit's implementation.
func evidenceRecord(kind artifact.EvidenceKind, producer string, ac string, source artifact.ProvenanceSource) artifact.Evidence {
	return artifact.Evidence{
		Schema:      "verdi.evidence/v1",
		EvidenceFor: []string{ac},
		Kind:        kind,
		Verdict:     artifact.VerdictPass,
		Witness:     "w",
		Producer:    producer,
		Provenance:  artifact.EvidenceProvenance{Source: source, Commit: testCommit},
		Digest:      "sha256:" + strings.Repeat("a", 64),
	}
}

// TestQuery_MatchesStoryAndAC proves Query returns exactly the records bound
// to the given (story, AC) pair — the queryable-by-(story, AC) mechanism
// (co-2) — out of a mixed set spanning other stories, other ACs, and other
// evidence kinds.
func TestQuery_MatchesStoryAndAC(t *testing.T) {
	want := evidenceRecord(artifact.EvidenceRuntime, CheckID("jira:VERDI-3", "ac-2"), "ac-2", artifact.SourceCI)
	records := []artifact.Evidence{
		want,
		evidenceRecord(artifact.EvidenceRuntime, CheckID("jira:VERDI-3", "ac-1"), "ac-1", artifact.SourceCI), // same story, different AC
		evidenceRecord(artifact.EvidenceRuntime, CheckID("jira:OTHER-1", "ac-2"), "ac-2", artifact.SourceCI), // different story, same AC
		evidenceRecord(artifact.EvidenceStatic, "some-static-producer", "ac-2", artifact.SourceCI),           // same AC, different kind
	}

	got := Query(records, "jira:VERDI-3", "ac-2")
	if len(got) != 1 {
		t.Fatalf("Query returned %d records, want exactly 1: %+v", len(got), got)
	}
	if got[0].Digest != want.Digest || got[0].Producer != want.Producer {
		t.Errorf("Query returned %+v, want %+v", got[0], want)
	}
}

// TestQuery_NoMatch proves Query returns nothing (not an error) when no
// record is bound to the queried (story, AC) — the ordinary "not evidenced
// yet" case, not a failure.
func TestQuery_NoMatch(t *testing.T) {
	records := []artifact.Evidence{
		evidenceRecord(artifact.EvidenceRuntime, CheckID("jira:OTHER-1", "ac-1"), "ac-1", artifact.SourceCI),
	}
	got := Query(records, "jira:VERDI-3", "ac-2")
	if len(got) != 0 {
		t.Fatalf("Query = %+v, want none", got)
	}
}

// TestQuery_ReturnsBothProvenanceSources proves Query itself does not
// filter by provenance — that is Fold's job (03 §The fold's Preview flag),
// not the mechanism's query layer, mirroring internal/evidence.LoadRecords'
// own "both provenance classes returned" contract.
func TestQuery_ReturnsBothProvenanceSources(t *testing.T) {
	records := []artifact.Evidence{
		evidenceRecord(artifact.EvidenceRuntime, CheckID("jira:VERDI-3", "ac-2"), "ac-2", artifact.SourceCI),
		evidenceRecord(artifact.EvidenceRuntime, CheckID("jira:VERDI-3", "ac-2"), "ac-2", artifact.SourceLocal),
	}
	got := Query(records, "jira:VERDI-3", "ac-2")
	if len(got) != 2 {
		t.Fatalf("Query returned %d records, want 2 (both provenance sources)", len(got))
	}
}

// TestCheckID_Deterministic proves CheckID is a pure, deterministic function
// of (storyRef, acID) — the same pair always yields the same id across
// separate probe runs, letting 03 §The fold's "(kind, producer), latest
// wins" grouping treat repeated runs as retries of the same check.
func TestCheckID_Deterministic(t *testing.T) {
	a := CheckID("jira:VERDI-3", "ac-2")
	b := CheckID("jira:VERDI-3", "ac-2")
	if a != b {
		t.Fatalf("CheckID not deterministic: %q != %q", a, b)
	}
	if CheckID("jira:VERDI-3", "ac-1") == a {
		t.Fatal("CheckID must differ for a different AC")
	}
	if CheckID("jira:OTHER-1", "ac-2") == a {
		t.Fatal("CheckID must differ for a different story")
	}
}
