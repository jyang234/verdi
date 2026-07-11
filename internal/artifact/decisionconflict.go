package artifact

import "fmt"

const decisionConflictSchema = "verdi.decisionconflict/v1"

// ConflictDisposition is a decision-conflict report's judged-finding
// disposition (03 §Decision-conflict gate: "Every judged finding is
// dispositioned with one of four values"). Deliberately a DIFFERENT,
// wider vocabulary than the build-branch deviation report's
// FindingDisposition (fixed/accepted-deviation, deviation.go): a decision
// conflict resolves to one of four distinct outcomes, not two, so this is
// its own closed enum rather than a reuse of FindingDisposition — see the
// phase report's judgment-call writeup for why a shared type was rejected
// (Finding.Validate hardcodes the two-value build-branch vocabulary and is
// not pluggable).
type ConflictDisposition string

const (
	ConflictSuperseded ConflictDisposition = "superseded"
	ConflictExempt     ConflictDisposition = "exempt"
	ConflictRejected   ConflictDisposition = "rejected"
	ConflictNoConflict ConflictDisposition = "no-conflict"
)

var validConflictDispositions = map[ConflictDisposition]bool{
	ConflictSuperseded: true,
	ConflictExempt:     true,
	ConflictRejected:   true,
	ConflictNoConflict: true,
}

// ConflictFinding is one entry in a decision-conflict report's `findings:`
// block — the design-branch analogue of Finding (deviation.go), sharing
// FindingKind (computed/judged) and the same content-hash finding-identity
// shape (kind, id, text) but carrying the four-value ConflictDisposition
// instead of Finding's fixed/accepted-deviation pair.
//
// TargetRef (optional) names the ADR or decision this finding is ABOUT —
// present on computed findings (the declared edge's target) and on judged
// findings the sweep prompt asked the judge to identify (internal/align's
// judge contract). RoutedOwners is computed-and-disclosed, never persisted
// by a human and never enforced: when TargetRef resolves to an ADR and
// Disposition is exempt or no-conflict, internal/align fills it with that
// ADR's own Owners (03 §Decision-conflict gate: "CODEOWNERS-routed to that
// ADR's owners ... verdi computes and discloses, never enforces
// approvals" — the routing IS the artifact path under CODEOWNERS; verdi
// never calls a forge API).
type ConflictFinding struct {
	ID           string              `yaml:"id"`
	Kind         FindingKind         `yaml:"kind"`
	Text         string              `yaml:"text"`
	Disposition  ConflictDisposition `yaml:"disposition,omitempty"`
	Note         string              `yaml:"note,omitempty"`
	TargetRef    string              `yaml:"target_ref,omitempty"`
	RoutedOwners []string            `yaml:"routed_owners,omitempty"`
}

// Validate checks ID/Text are present, Kind is computed or judged, and
// Disposition (if present) is one of the four known values, requiring a
// Note the same way Finding does for accepted-deviation — every
// disposition here records a human (or CODEOWNERS-routed) judgment call,
// so all four require a note.
func (f ConflictFinding) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("artifact: conflict finding has no id")
	}
	if f.Text == "" {
		return fmt.Errorf("artifact: conflict finding %s has no text", f.ID)
	}
	if !validFindingKinds[f.Kind] {
		return fmt.Errorf("artifact: conflict finding %s: kind %q is not computed or judged", f.ID, f.Kind)
	}
	if f.Disposition != "" && !validConflictDispositions[f.Disposition] {
		return fmt.Errorf("artifact: conflict finding %s: disposition %q is not a known value (superseded/exempt/rejected/no-conflict)", f.ID, f.Disposition)
	}
	if f.Disposition != "" && f.Note == "" {
		return fmt.Errorf("artifact: conflict finding %s: disposition %q requires a note", f.ID, f.Disposition)
	}
	return nil
}

// Dispositioned reports whether f carries a disposition at all.
func (f ConflictFinding) Dispositioned() bool { return f.Disposition != "" }

// SweepProvenance records the judged-sweep's own inputs (03 §Decision-
// conflict gate: "The sweep records its inputs — ADR corpus revision,
// decision set scanned — as computed provenance, so a partial or stale
// sweep is detectable"). ADRCorpusDigest is a content digest over every
// ADR the sweep read (id + raw content hash), recomputable and therefore
// stale-detectable the same way Provenance.Digest is elsewhere; a
// judge-skipped run still records this (the corpus was still read to
// build the — unsent — prompt) with an empty DecisionsScanned only when
// there were truly no decisions to scan.
type SweepProvenance struct {
	ADRCorpusDigest  string   `yaml:"adr_corpus_digest"`
	DecisionsScanned []string `yaml:"decisions_scanned"`
}

// Validate checks ADRCorpusDigest is a well-formed digest.
func (p SweepProvenance) Validate() error {
	if p.ADRCorpusDigest != "" && !sha256Re.MatchString(p.ADRCorpusDigest) {
		return fmt.Errorf("artifact: sweep_provenance.adr_corpus_digest %q is not sha256:<64 hex> form", p.ADRCorpusDigest)
	}
	return nil
}

// DecisionConflictFrontmatter is the frontmatter schema for
// decision-conflict-report.md (03 §Decision-conflict gate; 05 §CLI's
// `align` design-branch mode row), `verdi align`'s design-branch analogue
// of deviation-report.md (schema verdi.deviation/v1, deviation.go),
// written to the same spec-directory location
// (.verdi/specs/active/<name>/decision-conflict-report.md) — a documented
// invention-ledger choice (03 does not name a file path for this report;
// see the phase report), mirroring deviation-report.md's own placement
// exactly since both are produced by the same `verdi align` command over
// the same spec directory.
type DecisionConflictFrontmatter struct {
	Schema          string            `yaml:"schema"`
	Covers          string            `yaml:"covers"`
	Findings        []ConflictFinding `yaml:"findings"`
	SweepProvenance *SweepProvenance  `yaml:"sweep_provenance,omitempty"`
	Digest          string            `yaml:"digest,omitempty"`
	Integrity       string            `yaml:"integrity,omitempty"`
	JudgeIntegrity  *JudgeIntegrity   `yaml:"judge_integrity,omitempty"`
	Frozen          *Frozen           `yaml:"frozen,omitempty"`
	Provenance      *Provenance       `yaml:"provenance,omitempty"`
}

// DecodeDecisionConflict strict-decodes and validates
// decision-conflict-report.md frontmatter.
func DecodeDecisionConflict(data []byte) (*DecisionConflictFrontmatter, error) {
	var fm DecisionConflictFrontmatter
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
// present) is well-formed, Digest/Integrity (if present) are well-formed,
// and Frozen/Provenance (if present) are individually valid — the same
// shape as DeviationFrontmatter.Validate (deviation.go).
func (fm DecisionConflictFrontmatter) Validate() error {
	if fm.Schema != decisionConflictSchema {
		return fmt.Errorf("artifact: decision-conflict schema %q, want %q", fm.Schema, decisionConflictSchema)
	}
	if !commitRe.MatchString(fm.Covers) {
		return fmt.Errorf("artifact: decision-conflict covers %q is not a valid sha", fm.Covers)
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
	if fm.Digest != "" && !sha256Re.MatchString(fm.Digest) {
		return fmt.Errorf("artifact: decision-conflict digest %q is not sha256:<64 hex> form", fm.Digest)
	}
	if fm.Integrity != "" && !sha256Re.MatchString(fm.Integrity) {
		return fmt.Errorf("artifact: decision-conflict integrity %q is not sha256:<64 hex> form", fm.Integrity)
	}
	if fm.JudgeIntegrity != nil && fm.Integrity == "" {
		return fmt.Errorf("artifact: decision-conflict judge_integrity is present but integrity is empty")
	}
	if fm.Frozen != nil {
		if err := fm.Frozen.Validate(); err != nil {
			return fmt.Errorf("artifact: decision-conflict frozen: %w", err)
		}
	}
	if fm.Provenance != nil {
		if err := fm.Provenance.Validate(); err != nil {
			return fmt.Errorf("artifact: decision-conflict provenance: %w", err)
		}
	}
	return nil
}
