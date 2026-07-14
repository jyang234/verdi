package runtimeprobe

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// CheckID returns the deterministic runtime check id Emit stamps as a
// record's Producer for (storyRef, acID) — 03 §Evidence records' "producer
// ... the declared artifact id (obligation name, golden flow name, RUNTIME
// CHECK ID)", specialized here: unlike a static/behavioral producer (one id
// shared across every AC a service's graph/golden-flow proves), a runtime
// check is inherently (story, AC)-scoped — a post-deploy health check
// proving one story's one AC is live has no other referent — so the check
// id is DERIVED from the (story, AC) pair itself, deterministically, rather
// than hand-declared per call. This is what makes a loaded record set
// queryable by (story, AC) (co-2) with no artifact.Evidence schema change:
// Query below recomputes the same id and matches on it.
//
// Deterministic across repeated probe runs for the SAME (story, AC): 03
// §The fold's "(kind, producer), latest wins" grouping then treats retries
// of one check as retries of the same producer, exactly like a
// static/behavioral producer id — a pass-after-fail flake resolves to the
// latest run's verdict, the same honest reading the fold already applies
// everywhere else.
func CheckID(storyRef, acID string) string {
	return "runtime-probe:" + storyRef + ":" + acID
}

// ProbeInput is Emit's input: which (story, AC) pair this probe run is
// evidence for, what it observed, and the provenance context needed to
// stamp the record honestly.
type ProbeInput struct {
	// StoryRef is the story half of the (story, AC) key — a spec's
	// story: tracker ref (e.g. "jira:VERDI-3").
	StoryRef string
	// ACID is the AC half of the (story, AC) key (e.g. "ac-2").
	ACID string
	// Verdict is what the probe observed. Required: Emit never invents one
	// (dc-3: "do not fabricate a fake 'passing' verdi runtime record" — the
	// same discipline generalizes to every (story, AC), not only verdi's
	// own).
	Verdict artifact.EvidenceVerdict
	// Witness is free-form text describing what the probe checked (03
	// §Evidence records' witness field) — e.g. "GET /healthz -> 200".
	Witness string
	// Commit is the commit this probe run is evidence for
	// (provenance.commit).
	Commit string
	// Pipeline/Job are the running CI pipeline/job identifiers, "" outside
	// CI — forge.CIInfo's own fields, mirrored here rather than imported so
	// this package stays free of a forge dependency (Keep it small and
	// pure: Emit is a pure function of its input, not a forge client).
	Pipeline string
	Job      string
	// InCI reports whether this run is executing inside a genuine, detected
	// CI environment (internal/lint.ReadCIEnv().InCI).
	InCI bool
	// ForceLocal is an explicit local override: even inside CI, a
	// forced-local run never stamps source: ci (mirrors cmd/verdi/sync.go's
	// runProduce and its own --force-local discipline exactly, dc-3).
	ForceLocal bool
}

// Emit builds, digests, and validates one well-formed kind: runtime
// artifact.Evidence record for in's (story, AC) pair (ac-1, dc-1).
// provenance.source is ci only when in.InCI && !in.ForceLocal (dc-3's D6-10
// discipline, identical to sync.go's runProduce); every other case — not in
// CI, or an explicit --force-local override — stamps source: local, which
// the fold (internal/evidence) only ever consumes under --preview.
func Emit(in ProbeInput) (artifact.Evidence, error) {
	if in.StoryRef == "" {
		return artifact.Evidence{}, fmt.Errorf("runtime: Emit: StoryRef is required")
	}
	if in.ACID == "" {
		return artifact.Evidence{}, fmt.Errorf("runtime: Emit: ACID is required")
	}
	if in.Witness == "" {
		return artifact.Evidence{}, fmt.Errorf("runtime: Emit: Witness is required (a runtime record with no description of what it checked is not well-formed)")
	}

	source := artifact.SourceLocal
	if in.InCI && !in.ForceLocal {
		source = artifact.SourceCI
	}

	rec := artifact.Evidence{
		Schema:      "verdi.evidence/v1",
		EvidenceFor: []string{in.ACID},
		Kind:        artifact.EvidenceRuntime,
		Verdict:     in.Verdict,
		Witness:     in.Witness,
		Producer:    CheckID(in.StoryRef, in.ACID),
		Provenance: artifact.EvidenceProvenance{
			Source:   source,
			Commit:   in.Commit,
			Pipeline: in.Pipeline,
			Job:      in.Job,
		},
	}
	d, err := recordDigest(rec)
	if err != nil {
		return artifact.Evidence{}, err
	}
	rec.Digest = d

	if err := rec.Validate(); err != nil {
		return artifact.Evidence{}, fmt.Errorf("runtime: Emit: built record failed self-validation: %w", err)
	}
	return rec, nil
}

// Query returns the subset of records that are kind: runtime and bound to
// (storyRef, acID) — the queryable-by-(story, AC) hard constraint (03
// §Runtime evidence residence; co-2: "verdi close must be able to ask 'give
// me the runtime records for (this story, this AC)' and get them"). Matching
// is by Producer against CheckID(storyRef, acID) — deterministic and exact,
// never a fuzzy witness-text search. Returns nil (never an error) for no
// match: an absent runtime record is the ordinary "not evidenced yet" case
// (03 §The fold), not a failure.
func Query(records []artifact.Evidence, storyRef, acID string) []artifact.Evidence {
	want := CheckID(storyRef, acID)
	var out []artifact.Evidence
	for _, r := range records {
		if r.Kind != artifact.EvidenceRuntime {
			continue
		}
		if r.Producer != want {
			continue
		}
		out = append(out, r)
	}
	return out
}

// recordDigest hashes rec's declared content (kind, producer, evidence_for,
// verdict, witness) — recomputable from those pinned inputs, mirroring
// internal/bundle's recordDigest / cmd/verdi's selfHostedDigest posture (02
// §Generated artifacts and digests): a content-address of the fact this
// record asserts, not of wall-clock or provenance metadata. The hash tail
// itself is canonjson.Digest (spec/shared-homes ac-2).
func recordDigest(rec artifact.Evidence) (string, error) {
	keyed := struct {
		Kind        artifact.EvidenceKind    `json:"kind"`
		Producer    string                   `json:"producer"`
		EvidenceFor []string                 `json:"evidence_for"`
		Verdict     artifact.EvidenceVerdict `json:"verdict"`
		Witness     string                   `json:"witness"`
	}{Kind: rec.Kind, Producer: rec.Producer, EvidenceFor: rec.EvidenceFor, Verdict: rec.Verdict, Witness: rec.Witness}
	digest, err := canonjson.Digest(keyed)
	if err != nil {
		return "", fmt.Errorf("runtime: computing record digest: %w", err)
	}
	return digest, nil
}
