package artifact

import "fmt"

const evidenceSchema = "verdi.evidence/v1"

// EvidenceVerdict is an evidence record's `verdict` field
// (03 §Evidence records).
type EvidenceVerdict string

const (
	VerdictPass    EvidenceVerdict = "pass"
	VerdictFail    EvidenceVerdict = "fail"
	VerdictAbstain EvidenceVerdict = "abstain"
)

var validVerdicts = map[EvidenceVerdict]bool{
	VerdictPass:    true,
	VerdictFail:    true,
	VerdictAbstain: true,
}

// ProvenanceSource is an evidence record's `provenance.source` field.
// "ci" is authoritative; "local" is advisory (03 §Evidence records:
// "Provenance classes").
type ProvenanceSource string

const (
	SourceCI    ProvenanceSource = "ci"
	SourceLocal ProvenanceSource = "local"
)

var validProvenanceSources = map[ProvenanceSource]bool{
	SourceCI:    true,
	SourceLocal: true,
}

// EvidenceProvenance is an evidence record's provenance block, distinct
// from the frontmatter Provenance type (different fields: source/pipeline/
// commit, not generator/version/inputs/digest/integrity).
type EvidenceProvenance struct {
	Source   ProvenanceSource `json:"source"`
	Pipeline string           `json:"pipeline"`
	Commit   string           `json:"commit"`
}

// Evidence is schema verdi.evidence/v1 (03 §Evidence records), materialized
// under data/derived/<ref>/<commit>/ from CI bundles or local regeneration.
type Evidence struct {
	Schema      string             `json:"schema"`
	EvidenceFor []string           `json:"evidence_for"`
	Kind        EvidenceKind       `json:"kind"`
	Verdict     EvidenceVerdict    `json:"verdict"`
	Witness     string             `json:"witness"`
	Provenance  EvidenceProvenance `json:"provenance"`
	Digest      string             `json:"digest"`
}

// DecodeEvidence strict-decodes and validates a single evidence record.
func DecodeEvidence(data []byte) (*Evidence, error) {
	var e Evidence
	if err := DecodeStrictJSON(data, &e); err != nil {
		return nil, err
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return &e, nil
}

// Validate checks the schema literal, evidence_for lists at least one
// well-formed AC id, kind/verdict/provenance.source are known enums,
// provenance.commit looks like a real sha, and digest is sha256:<hex>.
func (e Evidence) Validate() error {
	if e.Schema != evidenceSchema {
		return fmt.Errorf("artifact: evidence schema %q, want %q", e.Schema, evidenceSchema)
	}
	if len(e.EvidenceFor) == 0 {
		return fmt.Errorf("artifact: evidence_for must name at least one AC")
	}
	for _, ac := range e.EvidenceFor {
		if !acIDRe.MatchString(ac) {
			return fmt.Errorf("artifact: evidence_for entry %q is not a valid ac-<slug> id", ac)
		}
	}
	if !validEvidenceKinds[e.Kind] {
		return fmt.Errorf("artifact: evidence kind %q is not a known kind", e.Kind)
	}
	if !validVerdicts[e.Verdict] {
		return fmt.Errorf("artifact: evidence verdict %q is not a known verdict", e.Verdict)
	}
	if !validProvenanceSources[e.Provenance.Source] {
		return fmt.Errorf("artifact: evidence provenance.source %q is not ci or local", e.Provenance.Source)
	}
	if !commitRe.MatchString(e.Provenance.Commit) {
		return fmt.Errorf("artifact: evidence provenance.commit %q is not a valid sha", e.Provenance.Commit)
	}
	if !sha256Re.MatchString(e.Digest) {
		return fmt.Errorf("artifact: evidence digest %q is not sha256:<64 hex> form", e.Digest)
	}
	return nil
}
