package align

import (
	"encoding/base64"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// digestInput is the exact, ordered content ComputeDigest hashes. Every
// field here is already deterministic by construction (Compute's own sort
// calls) and independent of human disposition edits, so ComputeDigest is a
// pure function of pinned inputs (02 §Generated artifacts and digests:
// "computed content carries a digest recomputable ... from the pinned
// inputs") — a verifier that reruns Compute against the same tree at the
// same commit and calls ComputeDigest again must get the identical string
// back, regardless of how the report's dispositions were edited meanwhile.
type digestInput struct {
	Covers        string                `json:"covers"`
	Findings      []findingIdentityOnly `json:"findings"`
	BaselineDiffs []ServiceBoundaryDiff `json:"baseline_diffs"`
}

type findingIdentityOnly struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// ComputeDigest hashes the computed section's content — declares.boundaries
// finding id/kind/text (never Disposition/Note, which are human state) plus
// the acceptance-baseline boundary diffs — over covers, via canonjson.Digest
// (spec/shared-homes ac-2) for byte-identical, deterministic output.
func ComputeDigest(covers string, computedFindings []artifact.Finding, baselineDiffs []ServiceBoundaryDiff) (string, error) {
	in := digestInput{Covers: covers, BaselineDiffs: baselineDiffs}
	for _, f := range computedFindings {
		in.Findings = append(in.Findings, findingIdentityOnly{ID: f.ID, Kind: string(f.Kind), Text: f.Text})
	}
	digest, err := canonjson.Digest(in)
	if err != nil {
		return "", fmt.Errorf("align: marshaling digest input: %w", err)
	}
	return digest, nil
}

// VerifyIntegrity recomputes fm's judged-section integrity hash from its
// own persisted JudgeIntegrity record (stdin bytes + raw judge result
// string) and compares it to fm.Integrity — self-contained, no re-exec of
// the judge, which 03 §Alignment report is explicit is never reproducible.
// Tampering with either the persisted stdin/raw-result bytes or the stored
// Integrity hash itself breaks verification.
//
// A report with no judged exchange at all (Integrity == "" and
// JudgeIntegrity == nil — the absence-finding-only case) verifies trivially
// true: there is nothing to check. A report carrying Integrity but no
// JudgeIntegrity (internal/artifact's DeviationFrontmatter.Validate allows
// this one-directionally, for an older or hand-authored frozen report that
// predates this self-verification record) is honestly UNVERIFIABLE, not
// silently accepted: VerifyIntegrity returns an error saying so rather than
// claiming a check it cannot perform.
func VerifyIntegrity(fm *artifact.DeviationFrontmatter) error {
	if fm.Integrity == "" && fm.JudgeIntegrity == nil {
		return nil
	}
	if fm.Integrity != "" && fm.JudgeIntegrity == nil {
		return fmt.Errorf("align: VerifyIntegrity: integrity is present but no judge_integrity record was persisted to recompute it from — unverifiable")
	}
	if fm.Integrity == "" && fm.JudgeIntegrity != nil {
		return fmt.Errorf("align: VerifyIntegrity: judge_integrity is present but integrity is empty")
	}

	stdin, err := base64.StdEncoding.DecodeString(fm.JudgeIntegrity.StdinB64)
	if err != nil {
		return fmt.Errorf("align: VerifyIntegrity: decoding stdin_b64: %w", err)
	}
	got := computeIntegrity(stdin, fm.JudgeIntegrity.RawResult)
	if got != fm.Integrity {
		return fmt.Errorf("align: VerifyIntegrity: recomputed integrity %s does not match stored %s (judged content tampered)", got, fm.Integrity)
	}
	return nil
}

// VerifyDigest recomputes fm's computed-section digest from an
// independently-recomputed ComputedResult (a caller reruns Compute against
// the pinned tree/commit and passes the result here) and compares it to
// fm.Digest.
func VerifyDigest(fm *artifact.DeviationFrontmatter, computed *ComputedResult) error {
	got, err := ComputeDigest(fm.Covers, computed.Findings, computed.BaselineDiffs)
	if err != nil {
		return err
	}
	if got != fm.Digest {
		return fmt.Errorf("align: VerifyDigest: recomputed digest %s does not match stored %s", got, fm.Digest)
	}
	return nil
}
