package evidence

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// Input is Fold's input: an already-resolved, already-decoded feature
// spec, the candidate evidence records to fold (any provenance source —
// Fold itself applies the authoritative-vs-preview filter), and enough
// store context to consult waivers and attestations.
type Input struct {
	// Spec is the feature spec whose acceptance_criteria the fold
	// evaluates. Required.
	Spec *artifact.SpecFrontmatter
	// Records are candidate evidence records already filtered to C-or-
	// ancestor-of-C (LoadRecords's job), of either provenance source —
	// Fold keeps only source:ci unless Preview is set.
	Records []artifact.Evidence
	// Preview folds source:local (advisory) records in alongside
	// source:ci (03 §Evidence records: "Provenance classes"). Output
	// produced this way must be clearly labeled by the caller (05 §CLI:
	// "--preview folds advisory records in, clearly labeled") — Fold
	// itself does not label; it is a pure computation.
	Preview bool
	// StoreRoot is the store root directory, used to resolve waiver and
	// attestation files on disk.
	StoreRoot string
	// StorySlug names the waivers/<StorySlug>/ and attestations/<StorySlug>/
	// directories to consult (I-6's "<story>--<ac-id>" compound name's
	// <story> half). Resolving this from a user-supplied story/spec ref is
	// the caller's job (cmd/verdi/matrix.go) — Fold takes it as given so
	// this package stays free of ref-resolution policy.
	StorySlug string
}

// Fold implements 03 §The fold for one story/spec: precedence is total,
// waived > violated > evidenced > pending > no-signal, computed
// independently per AC, then reduced to the story-level violated/eligible
// verdict. See doc.go for the fold pseudocode this mirrors line for line.
//
// Fold fails loudly (an error, never a silent no-signal) when a record's
// evidence_for names an AC the spec does not declare — 03 §Declarations:
// "a misspelled ac-3 must never surface as a silent no-signal."
func Fold(in Input) (StoryResult, error) {
	if in.Spec == nil {
		return StoryResult{}, fmt.Errorf("evidence: Fold: Spec is required")
	}
	if len(in.Spec.AcceptanceCriteria) == 0 {
		return StoryResult{}, fmt.Errorf("evidence: Fold: spec %q declares no acceptance criteria", in.Spec.ID)
	}

	acSet := make(map[string]bool, len(in.Spec.AcceptanceCriteria))
	for _, ac := range in.Spec.AcceptanceCriteria {
		acSet[ac.ID] = true
	}

	candidates, err := filterCandidates(in.Records, in.Preview, acSet, func(r artifact.Evidence, ac string) error {
		return fmt.Errorf("evidence: record (kind %s, witness %q) is evidence-for unknown AC %q (dangling binding, 03 §Declarations: \"a misspelled ac-3 must never surface as a silent no-signal\")", r.Kind, r.Witness, ac)
	})
	if err != nil {
		return StoryResult{}, err
	}

	result := StoryResult{Story: in.Spec.Story, SpecRef: in.Spec.ID}
	for _, ac := range in.Spec.AcceptanceCriteria {
		current := Current(RecordsForAC(candidates, ac.ID))

		waived, err := WaiverActive(in.StoreRoot, in.StorySlug, ac.ID)
		if err != nil {
			return StoryResult{}, err
		}

		attested := false
		if declaresKind(ac, artifact.EvidenceAttestation) {
			// spec/attest-helper dc-3: only the AUTHORED state satisfies —
			// an unauthored `verdi attest` scaffold collapses to exactly
			// the same not-satisfied outcome absence already produces
			// (parent spec/closure-ergonomics dc-2: "not foldable until
			// authored").
			state, stateErr := LoadAttestationState(in.StoreRoot, in.StorySlug, ac.ID)
			if stateErr != nil {
				return StoryResult{}, stateErr
			}
			attested = state == AttestationAuthored
		}

		status := foldAC(ac, current, waived, attested)
		result.ACs = append(result.ACs, ACResult{
			ID:      ac.ID,
			Text:    ac.Text,
			Status:  status,
			Summary: summarize(ac, current, attested),
		})
		if status == StatusViolated {
			result.Violated = true
		}
	}

	result.Eligible = true
	for _, r := range result.ACs {
		if r.Status != StatusEvidenced && r.Status != StatusWaived {
			result.Eligible = false
			break
		}
	}
	return result, nil
}

// foldAC applies 03 §The fold's per-AC precedence to one AC's already-
// reduced current record set.
func foldAC(ac artifact.AcceptanceCriterion, current []artifact.Evidence, waived, attested bool) Status {
	if waived {
		return StatusWaived
	}
	for _, r := range current {
		if r.Verdict == artifact.VerdictFail {
			return StatusViolated
		}
	}

	allSatisfied := true
	anySignal := false
	for _, kind := range ac.Evidence {
		satisfied, hasRecords := kindStatus(kind, current, attested)
		if hasRecords {
			anySignal = true
		}
		// Runtime has no v0 producer (OQ-2): a declared runtime kind is
		// always "awaited post-merge" regardless of whether a record
		// exists yet, so it always contributes signal — pending, never
		// no-signal, for that kind (PLAN.md Phase 6 stubs: "runtime
		// producer absent per OQ-2 but its pending rendering is
		// exercised").
		if kind == artifact.EvidenceRuntime {
			anySignal = true
		}
		if !satisfied {
			allSatisfied = false
		}
	}

	switch {
	case allSatisfied:
		return StatusEvidenced
	case anySignal:
		return StatusPending
	default:
		return StatusNoSignal
	}
}

// kindStatus reports, for one expected evidence kind, whether it is
// satisfied (attestation: file exists; otherwise: at least one current
// record of that kind passed) and whether it has any record/signal at all
// (attestation: the same as satisfied; otherwise: at least one current
// record of that kind exists, pass/fail/abstain alike).
func kindStatus(kind artifact.EvidenceKind, current []artifact.Evidence, attested bool) (satisfied, hasRecords bool) {
	if kind == artifact.EvidenceAttestation {
		return attested, attested
	}
	for _, r := range current {
		if r.Kind != kind {
			continue
		}
		hasRecords = true
		if r.Verdict == artifact.VerdictPass {
			satisfied = true
		}
	}
	return satisfied, hasRecords
}

// RecordsForAC returns the subset of records whose evidence_for names ac
// — the fold's own per-AC candidate filter (the exact step Fold applies
// before its Current reduction). Exported so a fold consumer computing
// per-AC record presence (spec/evidence-slot dc-1/co-3: "the slot's
// record loading and per-kind reduction reuse the evidence package's
// existing loader and Current reduction") shares this one filter instead
// of growing a lookalike.
func RecordsForAC(records []artifact.Evidence, ac string) []artifact.Evidence {
	var out []artifact.Evidence
	for _, r := range records {
		for _, a := range r.EvidenceFor {
			if a == ac {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func declaresKind(ac artifact.AcceptanceCriterion, kind artifact.EvidenceKind) bool {
	for _, k := range ac.Evidence {
		if k == kind {
			return true
		}
	}
	return false
}

// summarize renders a one-line, per-kind evidence summary for one AC's
// matrix row, e.g. "static:pass; behavioral:pending".
func summarize(ac artifact.AcceptanceCriterion, current []artifact.Evidence, attested bool) string {
	parts := make([]string, 0, len(ac.Evidence))
	for _, kind := range ac.Evidence {
		parts = append(parts, string(kind)+":"+summarizeKind(kind, current, attested))
	}
	return strings.Join(parts, "; ")
}

func summarizeKind(kind artifact.EvidenceKind, current []artifact.Evidence, attested bool) string {
	if kind == artifact.EvidenceAttestation {
		if attested {
			return "present"
		}
		return "absent"
	}

	var sawFail, sawPass, sawAbstain bool
	for _, r := range current {
		if r.Kind != kind {
			continue
		}
		switch r.Verdict {
		case artifact.VerdictFail:
			sawFail = true
		case artifact.VerdictPass:
			sawPass = true
		case artifact.VerdictAbstain:
			sawAbstain = true
		}
	}
	switch {
	case sawFail:
		return "fail"
	case sawPass:
		return "pass"
	case sawAbstain:
		return "abstain"
	case kind == artifact.EvidenceRuntime:
		return "awaited"
	default:
		return "none"
	}
}
