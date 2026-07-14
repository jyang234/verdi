// Diagram sweep report orchestration (spec/judged-sweep ac-1..4):
// `verdi align --diagram-sweep`'s analogue of decision_report.go's
// GenerateDecisionConflict, wiring diagram_judge.go's judged section into
// one sweep-report.md, reusing this package's disposition-preservation
// (identity.go) machinery — deliberately with NO computed section (a
// diagram sweep has no declared-edge completeness check to compute; every
// finding here is judged, spec/judged-sweep dc-3's own field list).
package align

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

const diagramSweepGeneratorVersion = "v1"

// DiagramSweepInput is GenerateDiagramSweep's input.
type DiagramSweepInput struct {
	// Root is the store root (for reading the ADR corpus).
	Root string
	// JudgeRunner mirrors DecisionConflictInput; nil defaults to
	// ExecJudgeRunner{}.
	JudgeRunner JudgeRunner
	// DiagramRef is the target diagram's own unpinned ref (e.g.
	// "diagram/loansvc-future").
	DiagramRef string
	// Body carries the diagram's ALREADY-READ mermaid body bytes — a value
	// parameter, never a file path or writable handle (spec/judged-sweep
	// ac-4/dc-5): this package can only read what its caller already read,
	// never open the diagram file itself.
	Body []byte
	// Diagram is the target diagram's already-decoded frontmatter, used
	// only to resolve an owning spec via its derived-from link
	// (resolveDiagramSpec) — nil is legal (an unlinked/from-scratch
	// proposal sweeps against the ADR corpus only).
	Diagram *artifact.DiagramFrontmatter
	// Covers is the commit this sweep ran at.
	Covers string
	// JudgeCmd/JudgeRequired/JudgeTimeout mirror verdi.yaml's align: block.
	JudgeCmd      []string
	JudgeRequired bool
	JudgeTimeout  time.Duration
	// ExistingFindings are a prior report's findings, the disposition-
	// preservation source (this mode's own report is regenerated and
	// overwritten on every invocation, spec/judged-sweep dc-3's own doc
	// comment — but a human's disposition of an unchanged finding must
	// still survive that regeneration).
	ExistingFindings []artifact.ConflictFinding
}

// DiagramSweepReport is GenerateDiagramSweep's output.
type DiagramSweepReport struct {
	Frontmatter *artifact.DiagramSweepFrontmatter
	Body        string
	Markdown    []byte
}

// GenerateDiagramSweep runs the diagram-sweep pipeline: load the ADR
// corpus, resolve the diagram's owning spec (if any), run the judged sweep
// through RunDiagramSweep, preserve dispositions and compute CODEOWNERS
// routing (the same computeRouting decision_report.go already established),
// and render sweep-report.md.
//
// Returns *ErrDiagramJudgeRequiredAbsent (never wrapped further) when
// JudgeRequired is true and no judge produced a judged section — the
// cmd/verdi caller maps this to exit 1, the same convention align.go's
// GenerateDecisionConflict/Generate use.
func GenerateDiagramSweep(ctx context.Context, in DiagramSweepInput) (*DiagramSweepReport, error) {
	if in.DiagramRef == "" {
		return nil, fmt.Errorf("align: GenerateDiagramSweep: DiagramRef is required")
	}
	if in.Covers == "" {
		return nil, fmt.Errorf("align: GenerateDiagramSweep: Covers must not be empty")
	}
	if in.Root == "" {
		return nil, fmt.Errorf("align: GenerateDiagramSweep: Root must not be empty")
	}

	judgeRunner := in.JudgeRunner
	if judgeRunner == nil {
		judgeRunner = ExecJudgeRunner{}
	}

	adrCorpus, adrDigest, err := loadADRCorpus(in.Root)
	if err != nil {
		return nil, err
	}

	var spec *artifact.SpecFrontmatter
	if in.Diagram != nil {
		spec, err = resolveDiagramSpec(in.Root, in.Diagram)
		if err != nil {
			return nil, err
		}
	}

	judged, err := RunDiagramSweep(ctx, judgeRunner, DiagramJudgedInput{
		DiagramRef:    in.DiagramRef,
		Body:          in.Body,
		ADRCorpus:     adrCorpus,
		Spec:          spec,
		JudgeCmd:      in.JudgeCmd,
		JudgeRequired: in.JudgeRequired,
		Timeout:       in.JudgeTimeout,
	})
	if err != nil {
		return nil, err // *ErrDiagramJudgeRequiredAbsent, propagated as-is
	}

	preserved := PreserveConflictDispositions(judged.Findings, in.ExistingFindings)
	preserved = computeRouting(preserved, adrCorpus)

	scanned := diagramSweepContext{Spec: spec}.scannedIDs()

	bodyDigest := sha256.Sum256(in.Body)
	digest, err := computeDiagramSweepDigest(in.Covers, in.DiagramRef, hex.EncodeToString(bodyDigest[:]), adrDigest, scanned)
	if err != nil {
		return nil, err
	}

	fm := &artifact.DiagramSweepFrontmatter{
		Schema:   "verdi.diagramsweep/v1",
		Covers:   in.Covers,
		Findings: preserved,
		SweepProvenance: &artifact.SweepProvenance{
			ADRCorpusDigest:  adrDigest,
			DecisionsScanned: scanned,
		},
		Integrity:      judged.Integrity,
		JudgeIntegrity: judged.JudgeIntegrity,
		Provenance: &artifact.Provenance{
			Generator: generatorName,
			Version:   diagramSweepGeneratorVersion,
			Inputs:    []string{in.DiagramRef + "@" + in.Covers},
			Digest:    digest,
			Integrity: judged.Integrity,
		},
	}

	if err := fm.Validate(); err != nil {
		return nil, fmt.Errorf("align: internal error: generated diagram-sweep frontmatter failed self-validation: %w", err)
	}

	body := RenderDiagramSweepBody(in.DiagramRef, preserved)
	return &DiagramSweepReport{Frontmatter: fm, Body: body, Markdown: RenderDiagramSweepMarkdown(fm, body)}, nil
}

// diagramSweepDigestInput mirrors decisionDigestInput
// (decision_report.go): the sweep's own pinned provenance (which diagram
// at which commit, its body's own content hash, the ADR corpus digest, and
// the constraint/decision set scanned) — recomputable and therefore
// stale-detectable even when the judge itself is skipped, unlike Integrity
// (which only exists after a real judge exchange).
type diagramSweepDigestInput struct {
	Covers           string   `json:"covers"`
	DiagramRef       string   `json:"diagram_ref"`
	BodySHA256       string   `json:"body_sha256"`
	ADRCorpusDigest  string   `json:"adr_corpus_digest"`
	DecisionsScanned []string `json:"decisions_scanned"`
}

// computeDiagramSweepDigest hashes the sweep's own provenance content,
// mirroring ComputeDecisionDigest's formula (canonjson.Digest,
// spec/shared-homes ac-2) so every generated-artifact digest in this
// package shares one convention.
func computeDiagramSweepDigest(covers, diagramRef, bodySHA256, adrCorpusDigest string, decisionsScanned []string) (string, error) {
	in := diagramSweepDigestInput{
		Covers:           covers,
		DiagramRef:       diagramRef,
		BodySHA256:       bodySHA256,
		ADRCorpusDigest:  adrCorpusDigest,
		DecisionsScanned: decisionsScanned,
	}
	digest, err := canonjson.Digest(in)
	if err != nil {
		return "", fmt.Errorf("align: marshaling diagram-sweep digest input: %w", err)
	}
	return digest, nil
}
