package evidence

import (
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

// ImplementingStory is one story's already-folded contribution to a
// feature AC — resolved, loaded, and story-folded by the caller, exactly
// as Input.StorySlug is caller-resolved for the story-level Fold (doc.go:
// "leaves ref/story resolution to its caller"). Discovering which stories
// implement which feature AC is a computed backlink inversion
// (03 §The feature fold: "the authoritative AC→story mapping is computed
// ... the set of story specs whose implements edges name the feature AC")
// that internal/index.Index.Backlinks already provides; this package
// takes the result as given so it stays free of index-building policy.
type ImplementingStory struct {
	// SpecRef is the implementing story's own ref (e.g.
	// "spec/borrower-update-api"), carried through for disclosure and
	// rendering.
	SpecRef string
	// ACIDs are the feature AC ids (the fragment target of this story's
	// `implements` edges into the feature under fold) this story
	// implements. A story with two implements edges into the same feature
	// contributes one entry per AC id.
	ACIDs []string
	// Closed reports whether the story spec's own status is "closed" —
	// the feature fold's "every implementing story is closed or
	// eligible" bullet's first half.
	Closed bool
	// Eligible is the story-level fold's Eligible (StoryResult.Eligible)
	// — the second half of "closed or eligible". Meaningless once Closed
	// is true (a closed story's spec no longer changes), but harmless to
	// leave populated either way.
	Eligible bool
	// Violated is the story-level fold's Violated (StoryResult.Violated)
	// — feeds the feature AC's violated propagation (03 §The feature
	// fold: "violated propagates up from any implementing story's
	// violated status").
	Violated bool
}

// FeatureInput is FoldFeature's input for one feature spec.
type FeatureInput struct {
	// Spec is the feature spec (class feature) whose acceptance_criteria
	// the fold evaluates. Required.
	Spec *artifact.SpecFrontmatter
	// Stories maps each feature AC id to the implementing stories the
	// caller discovered for it (via internal/index.Index.Backlinks or
	// equivalent) and already story-folded. An AC id with no entry (or an
	// empty slice) has no implementing story at all.
	Stories map[string][]ImplementingStory
	// Records are candidate outcome-level evidence records already
	// filtered to C-or-ancestor-of-C (LoadRecords's job), bound directly
	// to this feature's own AC ids (loaded from this feature's own
	// derived directory, not any implementing story's) — the outcome
	// floor's automated-record path (03 §The feature fold).
	Records []artifact.Evidence
	// Preview folds source:local (advisory) records in alongside
	// source:ci, mirroring the story-level Fold's Preview flag.
	Preview bool
	// StoreRoot is the store root directory, used to resolve outcome
	// attestation files on disk.
	StoreRoot string
	// FeatureSlug names the attestations/<FeatureSlug>/ directory to
	// consult for the outcome floor's attestation form (R4-I-11 / R4-I-17,
	// 08 §Round 4 E2 as amended: the feature spec's own NAME — the `name`
	// half of its ref, passed verbatim by the caller — not tracker-derived,
	// unlike the story-level StorySlug). Resolving this from the feature
	// spec is the caller's job (cmd/verdi/matrix.go, which passes ref.Name
	// directly) — FoldFeature takes it as given, the same "caller resolves,
	// fold reduces" idiom Input.StorySlug already establishes.
	FeatureSlug string
}

// FeatureACResult is one feature acceptance criterion's folded outcome.
type FeatureACResult struct {
	ID                  string
	Text                string
	Status              Status
	ImplementingStories []string // implementing stories' spec refs, sorted as given by the caller
	Summary             string   // one-line outcome-evidence summary, e.g. "attestation:present" or "behavioral:pass"
}

// FeatureResult is a whole feature's folded outcome.
type FeatureResult struct {
	SpecRef  string
	ACs      []FeatureACResult
	Violated bool
}

// FoldFeature implements 03 §The feature fold: a feature AC's status folds
// over its implementing stories plus the mandatory outcome floor.
// Precedence is total: violated > evidenced > pending > no-signal — there
// is no `waived` status at the feature level (03's feature-fold table
// names exactly four statuses; waivers are a story-level-only mechanism,
// §Attestations and waivers).
//
// FoldFeature fails loudly — never a silent no-signal — when a record's
// evidence_for names an AC the feature spec does not declare (03
// §Declarations: "a misspelled ac-3 must never surface as a silent
// no-signal"), mirroring Fold's own dangling-binding check.
func FoldFeature(in FeatureInput) (FeatureResult, error) {
	if in.Spec == nil {
		return FeatureResult{}, fmt.Errorf("evidence: FoldFeature: Spec is required")
	}
	if in.Spec.Class != artifact.ClassFeature {
		return FeatureResult{}, fmt.Errorf("evidence: FoldFeature: spec %q is class %q, not a feature spec", in.Spec.ID, in.Spec.Class)
	}
	if len(in.Spec.AcceptanceCriteria) == 0 {
		return FeatureResult{}, fmt.Errorf("evidence: FoldFeature: spec %q declares no acceptance criteria", in.Spec.ID)
	}

	acSet := make(map[string]bool, len(in.Spec.AcceptanceCriteria))
	for _, ac := range in.Spec.AcceptanceCriteria {
		acSet[ac.ID] = true
	}

	candidates := make([]artifact.Evidence, 0, len(in.Records))
	for _, r := range in.Records {
		switch r.Provenance.Source {
		case artifact.SourceCI:
			candidates = append(candidates, r)
		case artifact.SourceLocal:
			if in.Preview {
				candidates = append(candidates, r)
			}
		}
	}
	for _, r := range candidates {
		for _, ac := range r.EvidenceFor {
			if !acSet[ac] {
				return FeatureResult{}, fmt.Errorf("evidence: FoldFeature: record (kind %s, witness %q) is evidence-for unknown feature AC %q (dangling binding, 03 §Declarations)", r.Kind, r.Witness, ac)
			}
		}
	}

	result := FeatureResult{SpecRef: in.Spec.ID}
	for _, ac := range in.Spec.AcceptanceCriteria {
		stories := in.Stories[ac.ID]
		current := Current(filterEvidenceFor(candidates, ac.ID))

		attested := false
		if declaresKind(ac, artifact.EvidenceAttestation) {
			var err error
			attested, err = AttestationExists(in.StoreRoot, in.FeatureSlug, ac.ID)
			if err != nil {
				return FeatureResult{}, err
			}
		}

		status := foldFeatureAC(stories, current, attested)
		result.ACs = append(result.ACs, FeatureACResult{
			ID:                  ac.ID,
			Text:                ac.Text,
			Status:              status,
			ImplementingStories: implementingSpecRefs(stories),
			Summary:             summarizeFeatureAC(current, attested),
		})
		if status == StatusViolated {
			result.Violated = true
		}
	}
	return result, nil
}

// foldFeatureAC applies the feature fold's per-AC precedence
// (03 §The feature fold table):
//
//	violated  — any implementing story violated, or any current outcome
//	            record for this AC failed
//	no-signal — (checked before evidenced/pending, since "every implementing
//	            story closed or eligible" is vacuously true over an empty
//	            story set) no story carries an implements edge to this AC
//	evidenced — every implementing story closed or eligible AND >=1 direct
//	            authoritative record or outcome attestation bound to this AC
//	pending   — otherwise, once implementing stories exist
func foldFeatureAC(stories []ImplementingStory, current []artifact.Evidence, attested bool) Status {
	for _, s := range stories {
		if s.Violated {
			return StatusViolated
		}
	}
	for _, r := range current {
		if r.Verdict == artifact.VerdictFail {
			return StatusViolated
		}
	}

	if len(stories) == 0 {
		return StatusNoSignal
	}

	allClosedOrEligible := true
	for _, s := range stories {
		if !s.Closed && !s.Eligible {
			allClosedOrEligible = false
			break
		}
	}

	floorSatisfied := attested
	if !floorSatisfied {
		for _, r := range current {
			if r.Verdict == artifact.VerdictPass {
				floorSatisfied = true
				break
			}
		}
	}

	if allClosedOrEligible && floorSatisfied {
		return StatusEvidenced
	}
	return StatusPending
}

func implementingSpecRefs(stories []ImplementingStory) []string {
	if len(stories) == 0 {
		return nil
	}
	out := make([]string, len(stories))
	for i, s := range stories {
		out[i] = s.SpecRef
	}
	return out
}

// summarizeFeatureAC renders a one-line outcome-evidence summary for one
// feature AC's matrix row: the attestation floor's presence/absence plus
// any current outcome-level record verdicts, e.g. "attestation:present" or
// "attestation:absent; behavioral:pass".
func summarizeFeatureAC(current []artifact.Evidence, attested bool) string {
	parts := []string{"attestation:" + presentAbsent(attested)}
	for _, r := range current {
		if r.Kind == artifact.EvidenceAttestation {
			continue // already summarized via the attested bool above
		}
		parts = append(parts, string(r.Kind)+":"+string(r.Verdict))
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "; " + p
	}
	return out
}

func presentAbsent(b bool) string {
	if b {
		return "present"
	}
	return "absent"
}
