package evidence

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// Input is Fold's input: an already-resolved, already-decoded feature
// spec, the candidate evidence records to fold (any provenance source —
// Fold itself applies the authoritative-vs-preview filter), and enough
// store context to consult waivers and attestations.
type Input struct {
	// Context scopes Git ancestry checks used by obligation quality. A nil
	// context defaults to context.Background for compatibility with pure unit
	// callers.
	Context context.Context
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
	// EvaluationCommit is the exact commit whose authoritative evidence is
	// being folded. Production consumers always set it. An empty value keeps
	// the pre-adoption compatibility posture for legacy direct callers; an
	// explicitly present quality block still uses new semantics.
	EvaluationCommit string
	// SpecLandingCommit is the first-parent acceptance landing of Spec and is
	// required when an elaborated obligation declares spec freshness.
	SpecLandingCommit string
	// Git supplies only ancestry, defined at this consumer.
	Git ObligationAncestryReader
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
	ctx := in.Context
	if ctx == nil {
		ctx = context.Background()
	}
	specRef, err := artifact.ParseRef(in.Spec.ID)
	if err != nil {
		return StoryResult{}, fmt.Errorf("evidence: Fold: invalid spec id %q: %w", in.Spec.ID, err)
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

		attState := AttestationAbsent
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
			attState = state
		}

		status, kinds, qualityBlocking, err := foldAC(ctx, in, specRef.Name, ac, current, waived, attState)
		if err != nil {
			return StoryResult{}, err
		}
		result.ACs = append(result.ACs, ACResult{
			ID:      ac.ID,
			Text:    ac.Text,
			Status:  status,
			Summary: summarize(ac, current, attState == AttestationAuthored, kinds, qualityBlocking),
			Kinds:   kinds,
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
// reduced current record set, and returns the per-declared-kind evaluation
// (KindResult) it computes along the way so a disclosure consumer renders
// the SAME per-kind outcome the verdict folded, never a re-derived one
// (spec/close-preflight dc-2; ADJ-56). attState is the AC's attestation
// state (AttestationAbsent when the AC declares no attestation kind).
//
// The status reduction below is byte-for-byte the pre-breakdown fold: the
// per-kind breakdown is captured additively (kindStatus is called exactly
// once per kind, as before), the "any current record failed" violated rule
// still scans ALL current records — not just declared kinds — and the
// waived short-circuit still wins. No verdict changes; only the KindResult
// projection is new.
func foldAC(ctx context.Context, in Input, specName string, ac artifact.AcceptanceCriterion, current []artifact.Evidence, waived bool, attState AttestationState) (Status, []KindResult, []bool, error) {
	attested := attState == AttestationAuthored

	kinds := make([]KindResult, 0, len(ac.Evidence))
	qualityBlocking := make([]bool, 0, len(ac.Evidence))
	allSatisfied := true
	anySignal := false
	for _, kind := range ac.Evidence {
		incumbentSatisfied, hasRecords := kindStatus(kind, current, attested)
		assessment, err := AssessObligation(ctx, ObligationAssessmentInput{
			StoreRoot: in.StoreRoot,
			SpecName:  specName,
			ACID:      ac.ID,
			Kind:      kind,
		})
		if err != nil {
			return "", nil, nil, err
		}

		enforce := true
		historicalLegacy := false
		switch assessment.StructuralState {
		case ObligationLegacyUnelaborated:
			if in.EvaluationCommit == "" {
				enforce = false
			} else {
				class, err := ClassifyObligationEvaluation(ctx, in.Git, in.StoreRoot, in.EvaluationCommit)
				if err != nil {
					return "", nil, nil, err
				}
				enforce = class == ObligationEvaluationPostAdoption
				historicalLegacy = class == ObligationEvaluationHistorical
			}
		case ObligationMissing:
			// Legacy callers that do not identify an evaluation commit retain the
			// incumbent fold. Production consumers always identify the commit.
			enforce = in.EvaluationCommit != ""
		}
		if historicalLegacy && incumbentSatisfied && kind != artifact.EvidenceAttestation {
			// Compatibility belongs to historical proof, not merely historical
			// evaluation. A later positive record cannot borrow an old obligation's
			// undefined meaning, even if a direct caller supplies a record that
			// production LoadRecords would reject as outside the evaluation ancestry.
			historicalPass := false
			for i := range current {
				if current[i].Kind != kind || current[i].Verdict != artifact.VerdictPass {
					continue
				}
				recordClass, classErr := ClassifyObligationEvaluation(ctx, in.Git, in.StoreRoot, current[i].Provenance.Commit)
				if classErr != nil {
					return "", nil, nil, classErr
				}
				if recordClass == ObligationEvaluationHistorical {
					historicalPass = true
				}
			}
			if !historicalPass {
				enforce = true
			}
		}

		selected := assessment
		matchedPass := false
		if enforce {
			selected, err = MatchObligation(ctx, assessment, ObligationAssessmentInput{
				StoreRoot:         in.StoreRoot,
				SpecName:          specName,
				ACID:              ac.ID,
				Kind:              kind,
				EvaluationCommit:  in.EvaluationCommit,
				SpecLandingCommit: in.SpecLandingCommit,
				Git:               in.Git,
			})
			if err != nil {
				return "", nil, nil, err
			}
			for i := range current {
				if current[i].Kind != kind {
					continue
				}
				candidate, matchErr := MatchObligation(ctx, assessment, ObligationAssessmentInput{
					StoreRoot:         in.StoreRoot,
					SpecName:          specName,
					ACID:              ac.ID,
					Kind:              kind,
					Record:            &current[i],
					EvaluationCommit:  in.EvaluationCommit,
					SpecLandingCommit: in.SpecLandingCommit,
					Git:               in.Git,
				})
				if matchErr != nil {
					return "", nil, nil, matchErr
				}
				if candidate.MatchState == ObligationViolatedWithWitness {
					selected = candidate
					break
				}
				if candidate.MatchState == ObligationMatched && current[i].Verdict == artifact.VerdictPass {
					selected = candidate
					matchedPass = true
					continue
				}
				if selected.Reason == ObligationReasonProducerMissing {
					selected = candidate
				}
			}
		}

		satisfied := incumbentSatisfied
		if enforce {
			satisfied = assessment.StructuralState == ObligationElaborated && matchedPass
		}
		kr := KindResult{Kind: kind, Satisfied: satisfied}
		kr.ObligationQuality = ObligationQualityProjection{
			StructuralState: selected.StructuralState,
			MatchState:      selected.MatchState,
			Reason:          selected.Reason,
			WitnessPath:     selected.WitnessPath,
		}
		if kind == artifact.EvidenceAttestation {
			kr.Attestation = attState
		} else {
			kr.Violating = firstFailingOfKind(current, kind)
		}
		kinds = append(kinds, kr)
		qualityBlocking = append(qualityBlocking, enforce && !satisfied)

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
		if enforce && !satisfied {
			anySignal = true
		}
		if !satisfied {
			allSatisfied = false
		}
	}

	if waived {
		return StatusWaived, kinds, qualityBlocking, nil
	}
	for _, r := range current {
		if r.Verdict == artifact.VerdictFail {
			return StatusViolated, kinds, qualityBlocking, nil
		}
	}

	switch {
	case allSatisfied:
		return StatusEvidenced, kinds, qualityBlocking, nil
	case anySignal:
		return StatusPending, kinds, qualityBlocking, nil
	default:
		return StatusNoSignal, kinds, qualityBlocking, nil
	}
}

// firstFailingOfKind returns a copy of the first current record of kind
// whose verdict is fail (deterministic — current is already in the fold's
// stable group order), or nil when none failed. It is the fold's own
// per-kind violation witness, so a disclosure names the failing record
// (ADJ-56 finding 3) instead of misdescribing it as missing evidence.
func firstFailingOfKind(current []artifact.Evidence, kind artifact.EvidenceKind) *artifact.Evidence {
	for i := range current {
		if current[i].Kind == kind && current[i].Verdict == artifact.VerdictFail {
			r := current[i]
			return &r
		}
	}
	return nil
}

// firstFailing returns a copy of the first current record of ANY kind whose
// verdict is fail, or nil when none failed — the feature outcome floor's
// violation witness (foldFeatureAC scans all current records alike, since
// the floor is over outcome-level records regardless of kind).
func firstFailing(current []artifact.Evidence) *artifact.Evidence {
	for i := range current {
		if current[i].Verdict == artifact.VerdictFail {
			r := current[i]
			return &r
		}
	}
	return nil
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
func summarize(ac artifact.AcceptanceCriterion, current []artifact.Evidence, attested bool, kinds []KindResult, qualityBlocking []bool) string {
	parts := make([]string, 0, len(ac.Evidence))
	for i, kind := range ac.Evidence {
		incumbent := summarizeKind(kind, current, attested)
		// Design debt can block positive satisfaction, but it must never hide
		// an observed failure or replace its existing matrix witness.
		if incumbent == "fail" {
			parts = append(parts, string(kind)+":"+incumbent)
			continue
		}
		if i < len(qualityBlocking) && qualityBlocking[i] {
			q := kinds[i].ObligationQuality
			value := "pending(obligation-quality:" + string(q.StructuralState)
			if q.Reason != "" {
				value += "/" + string(q.Reason)
			}
			parts = append(parts, string(kind)+":"+value+")")
			continue
		}
		parts = append(parts, string(kind)+":"+incumbent)
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
