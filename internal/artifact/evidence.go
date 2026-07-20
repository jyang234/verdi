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
//
// Job is an I-25 addition (PLAN.md invention ledger): 03 §The fold orders
// "current" record selection by "(pipeline id, job id), monotonic", but the
// verdi.evidence/v1 example in 03 §Evidence records carries only
// `pipeline` — the spec's own example omits `job`. I-25 resolves this by
// adding an optional `job` field here; the fold (internal/evidence) treats
// an absent Job as sorting before any present Job within the same
// Pipeline, so same-pipeline retry ordering degrades gracefully rather
// than becoming ambiguous.
type EvidenceProvenance struct {
	Source   ProvenanceSource `json:"source"`
	Pipeline string           `json:"pipeline"`
	Job      string           `json:"job,omitempty"`
	Commit   string           `json:"commit"`
}

// Evidence is schema verdi.evidence/v1 (03 §Evidence records), materialized
// under data/derived/<ref>/<commit>/ from CI bundles or local regeneration.
//
// Producer is a second, phase-6 addition, in the same spirit as I-25's Job
// field and flagged as its own invention-ledger candidate: 03 §The fold
// defines "producer" as "the declared artifact id (obligation name, golden
// flow name, runtime check id)" and requires selecting the latest record
// per (kind, producer), but the verdi.evidence/v1 schema as specified in 03
// §Evidence records carries no producer field at all — only `witness`,
// which internal/bundle populates inconsistently (a static record's
// witness is usually "fn @ site", not the binding's producer id, once the
// matched graph obligation has a call site) and therefore cannot reliably
// recover producer identity by parsing. Rather than silently invent a
// parsing convention over free-text witness, this field makes producer
// identity explicit and optional; internal/bundle now stamps it from the
// binding that produced the record (JoinInput.Bindings), and
// internal/evidence falls back to grouping by (kind, witness) only when
// Producer is genuinely absent (e.g. hand-authored or pre-I-25 fixture
// records), which is the best-effort join the fold's "join through the
// bindings/witness" reading allows.
type Evidence struct {
	Schema      string             `json:"schema"`
	EvidenceFor []string           `json:"evidence_for"`
	Kind        EvidenceKind       `json:"kind"`
	Verdict     EvidenceVerdict    `json:"verdict"`
	Witness     string             `json:"witness"`
	Producer    string             `json:"producer,omitempty"`
	Provenance  EvidenceProvenance `json:"provenance"`
	// Quarantine is non-nil exactly when `verdi sync` found, at sync time,
	// that Provenance.Commit was not reachable from HEAD (spec/evidence-
	// resilience ac-1: X-15 — the routine shape a feature branch's
	// deletion produces once its PR has merged and CI evidence for it has
	// already been captured). The record is kept, never dropped, and
	// annotated here rather than silently removed from the synced set; a
	// quarantined record is never treated as authoritative evidence by the
	// fold (internal/evidence), which excludes it the same way it already
	// excludes any other non-ancestor record — see records.go's ancestry
	// check. Schema-additive (omitempty): every pre-existing record without
	// this field decodes exactly as before.
	Quarantine *EvidenceQuarantine `json:"quarantine,omitempty"`
	Digest     string              `json:"digest"`
}

// EvidenceQuarantine is the reason `verdi sync` quarantined a record
// (spec/evidence-resilience ac-1) — kept minimal (a single reason string)
// per the story's own "smallest reversible" instruction: an annotation on
// the record, not a restructuring of the schema.
type EvidenceQuarantine struct {
	Reason string `json:"reason"`
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
