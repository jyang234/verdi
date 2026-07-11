package artifact

import "fmt"

const deviationSchema = "verdi.deviation/v1"

// FindingKind tags a deviation finding as computed (regenerated graph/
// contract diff) or judged (the alignment subagent's semantic reading)
// (03 §Alignment report).
type FindingKind string

const (
	FindingComputed FindingKind = "computed"
	FindingJudged   FindingKind = "judged"
)

var validFindingKinds = map[FindingKind]bool{
	FindingComputed: true,
	FindingJudged:   true,
}

// FindingDisposition is a deviation finding's pre-merge disposition
// (03 §Gates: "every finding ... carries a disposition: fixed or
// accepted-deviation with a note").
type FindingDisposition string

const (
	FindingFixed             FindingDisposition = "fixed"
	FindingAcceptedDeviation FindingDisposition = "accepted-deviation"
)

var validFindingDispositions = map[FindingDisposition]bool{
	FindingFixed:             true,
	FindingAcceptedDeviation: true,
}

// Finding is one entry in a deviation report's `findings:` block.
type Finding struct {
	ID          string             `yaml:"id"`
	Kind        FindingKind        `yaml:"kind"`
	Text        string             `yaml:"text"`
	Disposition FindingDisposition `yaml:"disposition"`
	Note        string             `yaml:"note,omitempty"`
}

// Validate checks ID/Text are present, Kind is a known enum, Disposition is
// either empty (**undispositioned** — a living report's normal state for a
// new or changed finding before human review, PLAN.md Phase 8: "align ...
// marks new/changed findings undispositioned") or a known disposition
// value, and accepted-deviation carries a note (03 §Alignment report: "the
// sanctioned record of how the build diverged from the accepted design").
// An empty Disposition is legal at THIS decode seam deliberately: the merge
// gate — not schema decode — is what enforces "every finding carries a
// disposition" (03 §Gates condition 3), via Dispositioned/AllDispositioned
// below, since a living, mid-build report is a legitimate, decodable
// artifact even while findings remain open.
func (f Finding) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("artifact: finding has no id")
	}
	if f.Text == "" {
		return fmt.Errorf("artifact: finding %s has no text", f.ID)
	}
	if !validFindingKinds[f.Kind] {
		return fmt.Errorf("artifact: finding %s: kind %q is not computed or judged", f.ID, f.Kind)
	}
	if f.Disposition != "" && !validFindingDispositions[f.Disposition] {
		return fmt.Errorf("artifact: finding %s: disposition %q is not a known value", f.ID, f.Disposition)
	}
	if f.Disposition == FindingAcceptedDeviation && f.Note == "" {
		return fmt.Errorf("artifact: finding %s: accepted-deviation requires a note", f.ID)
	}
	return nil
}

// Dispositioned reports whether f carries a disposition at all — false is
// the "undispositioned" state Validate legally permits.
func (f Finding) Dispositioned() bool { return f.Disposition != "" }

// DeviationFrontmatter is the frontmatter schema for deviation-report.md,
// schema verdi.deviation/v1 (03 §Alignment report). It is decoded via the
// YAML frontmatter seam (the file is markdown, not plain JSON), unlike
// board/evidence/rollup which live in plain JSON files.
type DeviationFrontmatter struct {
	Schema     string      `yaml:"schema"`
	Covers     string      `yaml:"covers"`
	Findings   []Finding   `yaml:"findings"`
	Digest     string      `yaml:"digest,omitempty"`
	Integrity  string      `yaml:"integrity,omitempty"`
	Frozen     *Frozen     `yaml:"frozen,omitempty"`
	Provenance *Provenance `yaml:"provenance,omitempty"`
}

// DecodeDeviation strict-decodes and validates deviation-report.md
// frontmatter.
func DecodeDeviation(data []byte) (*DeviationFrontmatter, error) {
	var fm DeviationFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the schema literal, Covers is a valid commit sha, every
// finding is individually valid with a unique id, Digest/Integrity (if
// present) are well-formed, and Frozen (if present) is well-formed.
func (fm DeviationFrontmatter) Validate() error {
	if fm.Schema != deviationSchema {
		return fmt.Errorf("artifact: deviation schema %q, want %q", fm.Schema, deviationSchema)
	}
	if !commitRe.MatchString(fm.Covers) {
		return fmt.Errorf("artifact: deviation covers %q is not a valid sha", fm.Covers)
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
	if fm.Digest != "" && !sha256Re.MatchString(fm.Digest) {
		return fmt.Errorf("artifact: deviation digest %q is not sha256:<64 hex> form", fm.Digest)
	}
	if fm.Integrity != "" && !sha256Re.MatchString(fm.Integrity) {
		return fmt.Errorf("artifact: deviation integrity %q is not sha256:<64 hex> form", fm.Integrity)
	}
	if fm.Frozen != nil {
		if err := fm.Frozen.Validate(); err != nil {
			return fmt.Errorf("artifact: deviation frozen: %w", err)
		}
	}
	if fm.Provenance != nil {
		if err := fm.Provenance.Validate(); err != nil {
			return fmt.Errorf("artifact: deviation provenance: %w", err)
		}
	}
	return nil
}
