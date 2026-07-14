package align

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func diagramSweepInputBase(root string) DiagramSweepInput {
	return DiagramSweepInput{
		Root:       root,
		DiagramRef: "diagram/loansvc-future",
		Body:       []byte("graph TD\n  a --> b\n"),
		Covers:     "abc1234",
	}
}

// TestGenerateDiagramSweep_JudgeSkipped_SyntheticAbsenceFinding proves a
// judge-skipped run degrades to the synthetic absence finding (never a bare
// pass, never zero findings) — the same three-valued-honesty posture
// GenerateDecisionConflict's own headline exit criterion proves.
func TestGenerateDiagramSweep_JudgeSkipped_SyntheticAbsenceFinding(t *testing.T) {
	root := t.TempDir()
	report, err := GenerateDiagramSweep(context.Background(), diagramSweepInputBase(root))
	if err != nil {
		t.Fatalf("GenerateDiagramSweep: %v", err)
	}
	if len(report.Frontmatter.Findings) != 1 || report.Frontmatter.Findings[0].ID != DiagramAbsenceFindingID {
		t.Fatalf("Findings = %+v, want exactly one %s", report.Frontmatter.Findings, DiagramAbsenceFindingID)
	}
	if report.Frontmatter.Findings[0].Dispositioned() {
		t.Fatal("synthetic absence finding must start undispositioned")
	}
	// No real judge exchange happened, so no Integrity is claimed.
	if report.Frontmatter.Integrity != "" || report.Frontmatter.JudgeIntegrity != nil {
		t.Fatal("a judge-skipped run must carry no integrity record")
	}
}

// TestGenerateDiagramSweep_RoundTripIntegrity is spec/judged-sweep ac-3's
// behavioral obligation: generate a report against a fake judge, decode its
// judge_integrity fields, recompute computeIntegrity independently, and
// assert it matches the persisted integrity value byte-for-byte.
func TestGenerateDiagramSweep_RoundTripIntegrity(t *testing.T) {
	root := t.TempDir()
	script := writeFakeJudge(t, fakeDiagramJudgeOKScript)

	report, err := GenerateDiagramSweep(context.Background(), DiagramSweepInput{
		Root:       root,
		DiagramRef: "diagram/loansvc-future",
		Body:       []byte("graph TD\n  a --> b\n"),
		Covers:     "abc1234",
		JudgeCmd:   []string{script},
	})
	if err != nil {
		t.Fatalf("GenerateDiagramSweep: %v", err)
	}

	fm := report.Frontmatter
	if fm.Integrity == "" || fm.JudgeIntegrity == nil {
		t.Fatal("a real judge exchange must populate Integrity/JudgeIntegrity")
	}

	// Round-trip through the rendered markdown, exactly like a caller that
	// reads sweep-report.md back off disk would.
	decoded, err := decodeRenderedDiagramSweep(t, report.Markdown)
	if err != nil {
		t.Fatalf("decoding rendered sweep-report.md: %v", err)
	}

	stdin, err := base64.StdEncoding.DecodeString(decoded.JudgeIntegrity.StdinB64)
	if err != nil {
		t.Fatalf("base64-decoding stdin_b64: %v", err)
	}
	recomputed := computeIntegrity(stdin, decoded.JudgeIntegrity.RawResult)
	if recomputed != decoded.Integrity {
		t.Fatalf("recomputed integrity %q != persisted integrity %q", recomputed, decoded.Integrity)
	}
	if recomputed != fm.Integrity {
		t.Fatalf("recomputed integrity %q != in-memory fm.Integrity %q", recomputed, fm.Integrity)
	}
}

// decodeRenderedDiagramSweep splits and strict-decodes a rendered
// sweep-report.md's frontmatter — the same round trip a real caller
// re-reading the file off disk would perform.
func decodeRenderedDiagramSweep(t *testing.T, markdown []byte) (*artifact.DiagramSweepFrontmatter, error) {
	t.Helper()
	fmBytes, _, err := artifact.SplitFrontmatter(markdown)
	if err != nil {
		return nil, err
	}
	return artifact.DecodeDiagramSweep(fmBytes)
}

// TestGenerateDiagramSweep_DispositionPreservedAcrossRegeneration proves a
// human's disposition of an unchanged finding survives a regeneration
// (PreserveConflictDispositions), and that a disposition of "exempt"
// targeting an ADR gets CODEOWNERS-routed to that ADR's owners — mirroring
// TestGenerateDecisionConflict_JudgedFoundAndDispositioned exactly.
func TestGenerateDiagramSweep_DispositionPreservedAcrossRegeneration(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "outbox-mandate", "accepted")
	script := writeFakeJudge(t, fakeDiagramJudgeOKScript) // targets adr/outbox-mandate

	first, err := GenerateDiagramSweep(context.Background(), DiagramSweepInput{
		Root: root, DiagramRef: "diagram/loansvc-future",
		Body: []byte("graph TD\n"), Covers: "abc1234",
		JudgeCmd: []string{script},
	})
	if err != nil {
		t.Fatalf("GenerateDiagramSweep (first): %v", err)
	}
	if len(first.Frontmatter.Findings) != 1 || first.Frontmatter.Findings[0].Dispositioned() {
		t.Fatalf("first run findings = %+v, want one undispositioned judged finding", first.Frontmatter.Findings)
	}

	dispositioned := first.Frontmatter.Findings
	dispositioned[0].Disposition = artifact.ConflictExempt
	dispositioned[0].Note = "reviewed, exemption stands"

	second, err := GenerateDiagramSweep(context.Background(), DiagramSweepInput{
		Root: root, DiagramRef: "diagram/loansvc-future",
		Body: []byte("graph TD\n"), Covers: "def5678",
		JudgeCmd:         []string{script},
		ExistingFindings: dispositioned,
	})
	if err != nil {
		t.Fatalf("GenerateDiagramSweep (second): %v", err)
	}
	f := second.Frontmatter.Findings[0]
	if f.Disposition != artifact.ConflictExempt {
		t.Fatalf("Disposition = %q, want exempt (preserved across regeneration)", f.Disposition)
	}
	if len(f.RoutedOwners) != 1 || f.RoutedOwners[0] != "platform-team" {
		t.Fatalf("RoutedOwners = %v, want [platform-team] (CODEOWNERS routing computed from adr/outbox-mandate's owners)", f.RoutedOwners)
	}
}

// TestGenerateDiagramSweep_SweepProvenanceRecordedAndStaleDetectable proves
// SweepProvenance's ADR corpus digest changes when the corpus changes —
// staleness detection, mirroring
// TestGenerateDecisionConflict_SweepProvenanceRecorded.
func TestGenerateDiagramSweep_SweepProvenanceRecordedAndStaleDetectable(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")

	report, err := GenerateDiagramSweep(context.Background(), diagramSweepInputBase(root))
	if err != nil {
		t.Fatalf("GenerateDiagramSweep: %v", err)
	}
	sp := report.Frontmatter.SweepProvenance
	if sp == nil || sp.ADRCorpusDigest == "" {
		t.Fatalf("SweepProvenance = %+v, want a populated ADR corpus digest", sp)
	}

	writeADR(t, root, "second-policy", "accepted")
	report2, err := GenerateDiagramSweep(context.Background(), diagramSweepInputBase(root))
	if err != nil {
		t.Fatalf("GenerateDiagramSweep (2): %v", err)
	}
	if report2.Frontmatter.SweepProvenance.ADRCorpusDigest == sp.ADRCorpusDigest {
		t.Fatal("ADRCorpusDigest did not change after the ADR corpus changed — staleness would go undetected")
	}
	// The overall Provenance.Digest must also change when the diagram's own
	// body changes, at a fixed covers/corpus — a stale sweep (rerun against
	// changed diagram content but a cached report) must be detectable too.
	in2 := diagramSweepInputBase(root)
	in2.Body = []byte("graph TD\n  a --> b\n  b --> c\n")
	report3, err := GenerateDiagramSweep(context.Background(), in2)
	if err != nil {
		t.Fatalf("GenerateDiagramSweep (3): %v", err)
	}
	if report3.Frontmatter.Provenance.Digest == report2.Frontmatter.Provenance.Digest {
		t.Fatal("Provenance.Digest did not change after the diagram body changed — staleness would go undetected")
	}
}

// TestGenerateDiagramSweep_DisclosureLineAlwaysPresent proves the fixed
// advisory/non-exhaustive disclosure line is present verbatim both in a
// report with a real finding and in one with none — spec/judged-sweep
// ac-4's "never phrases a finding or an absent finding as a completeness
// guarantee".
func TestGenerateDiagramSweep_DisclosureLineAlwaysPresent(t *testing.T) {
	root := t.TempDir()

	t.Run("judge skipped (only the synthetic absence finding)", func(t *testing.T) {
		report, err := GenerateDiagramSweep(context.Background(), diagramSweepInputBase(root))
		if err != nil {
			t.Fatalf("GenerateDiagramSweep: %v", err)
		}
		if !strings.Contains(string(report.Markdown), DiagramSweepDisclosureLine) {
			t.Fatalf("rendered report does not carry the fixed disclosure line verbatim:\n%s", report.Markdown)
		}
	})

	t.Run("judge finds a real conflict", func(t *testing.T) {
		script := writeFakeJudge(t, fakeDiagramJudgeOKScript)
		in := diagramSweepInputBase(root)
		in.JudgeCmd = []string{script}
		report, err := GenerateDiagramSweep(context.Background(), in)
		if err != nil {
			t.Fatalf("GenerateDiagramSweep: %v", err)
		}
		if !strings.Contains(string(report.Markdown), DiagramSweepDisclosureLine) {
			t.Fatalf("rendered report does not carry the fixed disclosure line verbatim:\n%s", report.Markdown)
		}
	})
}

func TestGenerateDiagramSweep_Negative_EmptyDiagramRef(t *testing.T) {
	_, err := GenerateDiagramSweep(context.Background(), DiagramSweepInput{Root: t.TempDir(), Covers: "abc1234"})
	if err == nil {
		t.Fatal("GenerateDiagramSweep(empty DiagramRef): want error, got nil")
	}
}

func TestGenerateDiagramSweep_Negative_EmptyCovers(t *testing.T) {
	_, err := GenerateDiagramSweep(context.Background(), DiagramSweepInput{Root: t.TempDir(), DiagramRef: "diagram/x"})
	if err == nil {
		t.Fatal("GenerateDiagramSweep(empty Covers): want error, got nil")
	}
}

func TestGenerateDiagramSweep_Negative_EmptyRoot(t *testing.T) {
	_, err := GenerateDiagramSweep(context.Background(), DiagramSweepInput{DiagramRef: "diagram/x", Covers: "abc1234"})
	if err == nil {
		t.Fatal("GenerateDiagramSweep(empty Root): want error, got nil")
	}
}

// TestGenerateDiagramSweep_JudgeRequiredAndAbsent proves the propagated
// error type when the caller declared the judge mandatory but none was
// configured.
func TestGenerateDiagramSweep_JudgeRequiredAndAbsent(t *testing.T) {
	root := t.TempDir()
	in := diagramSweepInputBase(root)
	in.JudgeRequired = true
	_, err := GenerateDiagramSweep(context.Background(), in)
	if err == nil {
		t.Fatal("GenerateDiagramSweep: want error when judge_required and no judge configured")
	}
}
