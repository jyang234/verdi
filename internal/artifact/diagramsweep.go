package artifact

import "fmt"

const diagramSweepSchema = "verdi.diagramsweep/v1"

// DiagramSweepFrontmatter is the frontmatter schema for
// <name>.sweep-report.md (spec/judged-sweep dc-3, dc-4): `verdi align
// --diagram-sweep`'s own report, a sibling shape to
// DecisionConflictFrontmatter that reuses ConflictFinding/
// ConflictDisposition/SweepProvenance/JudgeIntegrity/Provenance directly
// rather than redeclaring any of them.
//
// Deliberately carries NO Frozen field and no standalone top-level Digest
// field, unlike DeviationFrontmatter/DecisionConflictFrontmatter: this
// report is never a merge-gate artifact and is never frozen
// (spec/judged-sweep's own "on-demand, disposable" framing, dc-1) — every
// invocation simply regenerates and overwrites it, and its one digest
// (computed over inputs that stay meaningful even when the judge is
// absent) lives inside Provenance.Digest, not as a second top-level field
// (dc-3's own field list: schema, covers, findings, sweep_provenance,
// integrity, judge_integrity, provenance — nothing else).
type DiagramSweepFrontmatter struct {
	Schema          string            `yaml:"schema"`
	Covers          string            `yaml:"covers"`
	Findings        []ConflictFinding `yaml:"findings"`
	SweepProvenance *SweepProvenance  `yaml:"sweep_provenance,omitempty"`
	Integrity       string            `yaml:"integrity,omitempty"`
	JudgeIntegrity  *JudgeIntegrity   `yaml:"judge_integrity,omitempty"`
	Provenance      *Provenance       `yaml:"provenance,omitempty"`
}

// DecodeDiagramSweep strict-decodes and validates sweep-report.md
// frontmatter. A sweep-report.md is not itself a kind: diagram artifact
// (spec/judged-sweep dc-4), so it is decoded through this dedicated seam,
// never routed through DecodeDiagram.
func DecodeDiagramSweep(data []byte) (*DiagramSweepFrontmatter, error) {
	var fm DiagramSweepFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the schema literal, Covers is a valid commit sha, every
// finding is individually valid with a unique id, SweepProvenance (if
// present) is well-formed, Integrity (if present) is well-formed,
// JudgeIntegrity implies a non-empty Integrity, and Provenance (if
// present) is individually valid — mirroring DecisionConflictFrontmatter.
// Validate exactly, minus the Digest/Frozen fields that report carries and
// this one deliberately does not.
func (fm DiagramSweepFrontmatter) Validate() error {
	if fm.Schema != diagramSweepSchema {
		return fmt.Errorf("artifact: diagram-sweep schema %q, want %q", fm.Schema, diagramSweepSchema)
	}
	if !commitRe.MatchString(fm.Covers) {
		return fmt.Errorf("artifact: diagram-sweep covers %q is not a valid sha", fm.Covers)
	}
	seen := make(map[string]bool, len(fm.Findings))
	for i, f := range fm.Findings {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("artifact: findings[%d]: %w", i, err)
		}
		if seen[f.ID] {
			return fmt.Errorf("artifact: findings[%d]: duplicate id %q", i, f.ID)
		}
		seen[f.ID] = true
	}
	if fm.SweepProvenance != nil {
		if err := fm.SweepProvenance.Validate(); err != nil {
			return fmt.Errorf("artifact: sweep_provenance: %w", err)
		}
	}
	if fm.Integrity != "" && !sha256Re.MatchString(fm.Integrity) {
		return fmt.Errorf("artifact: diagram-sweep integrity %q is not sha256:<64 hex> form", fm.Integrity)
	}
	if fm.JudgeIntegrity != nil && fm.Integrity == "" {
		return fmt.Errorf("artifact: diagram-sweep judge_integrity is present but integrity is empty")
	}
	if fm.Provenance != nil {
		if err := fm.Provenance.Validate(); err != nil {
			return fmt.Errorf("artifact: diagram-sweep provenance: %w", err)
		}
	}
	return nil
}
