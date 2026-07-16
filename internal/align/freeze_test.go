package align

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// livingReport builds a fresh, fully-dispositioned deviation report whose
// judged section self-verifies (Integrity == computeIntegrity(stdin, raw)) —
// the adjudicated state FreezeInPlace must stamp permanent without touching a
// byte of it.
func livingReport(t *testing.T) (*artifact.DeviationFrontmatter, string) {
	t.Helper()
	covers := strings.Repeat("a", 40)
	stdin := []byte("the exact judge prompt bytes")
	raw := `{"findings":[{"id":"j-1","text":"retry semantics match","confidence":0.9}]}`
	fm := &artifact.DeviationFrontmatter{
		Schema: "verdi.deviation/v1",
		Covers: covers,
		Findings: []artifact.Finding{
			{ID: "boundary-loansvc-notification-svc-events", Kind: artifact.FindingComputed, Text: "declared boundary holds", Disposition: artifact.FindingFixed},
			{ID: "judged-j-1", Kind: artifact.FindingJudged, Text: "retry semantics match (confidence 0.90)", Disposition: artifact.FindingAcceptedDeviation, Note: "owner-ratified: intentional deviation"},
		},
		Digest:    "sha256:" + strings.Repeat("b", 64),
		Integrity: computeIntegrity(stdin, raw),
		JudgeIntegrity: &artifact.JudgeIntegrity{
			StdinB64:  base64.StdEncoding.EncodeToString(stdin),
			RawResult: raw,
		},
	}
	if err := fm.Validate(); err != nil {
		t.Fatalf("test setup: living report invalid: %v", err)
	}
	if err := VerifyIntegrity(fm); err != nil {
		t.Fatalf("test setup: living report does not self-verify: %v", err)
	}
	return fm, "## Findings\n\n- declared boundary holds\n"
}

func findFinding(fs []artifact.Finding, id string) (artifact.Finding, bool) {
	for _, f := range fs {
		if f.ID == id {
			return f, true
		}
	}
	return artifact.Finding{}, false
}

// TestFreezeInPlace_PreservesAdjudicatedStateVerbatim proves the faithful
// freeze: every finding, disposition, and note is carried over exactly; the
// digest and judge exchange are reused unchanged (so the freeze stays
// verifiable); the body is reattached byte-for-byte; only the frozen stamp is
// added; and the caller's report is never mutated.
func TestFreezeInPlace_PreservesAdjudicatedStateVerbatim(t *testing.T) {
	living, body := livingReport(t)
	frozenAt := "2026-07-15"

	report, err := FreezeInPlace(living, body, frozenAt)
	if err != nil {
		t.Fatalf("FreezeInPlace: %v", err)
	}

	// The frozen stamp names the covered commit, dated frozenAt.
	if report.Frontmatter.Frozen == nil {
		t.Fatal("frozen report has no Frozen stamp")
	}
	if report.Frontmatter.Frozen.At != frozenAt || report.Frontmatter.Frozen.Commit != living.Covers {
		t.Fatalf("Frozen = %+v, want {At:%s Commit:%s}", report.Frontmatter.Frozen, frozenAt, living.Covers)
	}

	// Every finding — text, disposition, note — is carried over verbatim.
	if len(report.Frontmatter.Findings) != len(living.Findings) {
		t.Fatalf("frozen has %d findings, living had %d", len(report.Frontmatter.Findings), len(living.Findings))
	}
	for i, got := range report.Frontmatter.Findings {
		if want := living.Findings[i]; got != want {
			t.Fatalf("finding[%d] = %+v, want verbatim %+v", i, got, want)
		}
	}

	// Digest and judge exchange are reused unchanged, so the freeze stays
	// verifiable (VerifyIntegrity recomputes the same hash).
	if report.Frontmatter.Digest != living.Digest {
		t.Fatalf("digest changed: %q -> %q", living.Digest, report.Frontmatter.Digest)
	}
	if report.Frontmatter.Integrity != living.Integrity {
		t.Fatalf("integrity changed: %q -> %q", living.Integrity, report.Frontmatter.Integrity)
	}
	if err := VerifyIntegrity(report.Frontmatter); err != nil {
		t.Fatalf("frozen report no longer self-verifies: %v", err)
	}

	// The body is reattached byte-for-byte.
	if report.Body != body {
		t.Fatalf("body changed: %q -> %q", body, report.Body)
	}

	// The rendered markdown round-trips through the strict decode seam and
	// still carries the stamp + the preserved judged disposition.
	fmBytes, gotBody, err := artifact.SplitFrontmatter(report.Markdown)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fmBytes)
	if err != nil {
		t.Fatalf("DecodeDeviation(frozen markdown): %v\n%s", err, report.Markdown)
	}
	if decoded.Frozen == nil {
		t.Fatal("decoded frozen markdown has no Frozen stamp")
	}
	if string(gotBody) != body {
		t.Fatalf("round-tripped body = %q, want %q", gotBody, body)
	}
	j, ok := findFinding(decoded.Findings, "judged-j-1")
	if !ok || j.Disposition != artifact.FindingAcceptedDeviation || j.Note == "" {
		t.Fatalf("judged disposition not preserved through the markdown round trip: %+v (present=%v)", j, ok)
	}

	// The caller's report is never mutated.
	if living.Frozen != nil {
		t.Fatal("FreezeInPlace mutated the caller's report (set Frozen on the original)")
	}
}

// TestFreezeInPlace_Rejects covers the fail-closed precondition checks: a
// faithful freeze must refuse rather than fake success on a missing report, an
// already-frozen report (immutable), or a missing frozen-at stamp.
func TestFreezeInPlace_Rejects(t *testing.T) {
	valid, body := livingReport(t)
	alreadyFrozen := *valid
	alreadyFrozen.Frozen = &artifact.Frozen{At: "2024-01-01", Commit: valid.Covers}

	tests := []struct {
		name     string
		existing *artifact.DeviationFrontmatter
		frozenAt string
	}{
		{"nil existing report", nil, "2026-07-15"},
		{"already frozen is immutable", &alreadyFrozen, "2026-07-15"},
		{"empty frozenAt", valid, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := FreezeInPlace(tc.existing, body, tc.frozenAt); err == nil {
				t.Fatal("FreezeInPlace: want error, got nil")
			}
		})
	}
}
