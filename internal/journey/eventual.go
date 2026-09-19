package journey

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// noConflictReportDisclosure is R-RR1-2's fixed disclosure: with no
// conflict report supplied, the three conflict-derived sources (mechanical,
// semantic, exemption) are not evaluated, but the eventual section is
// still Derived: true over everything else it CAN see (DC-11: a debt named
// earlier is still a debt, never manufactured, and an unavailable input is
// disclosed, never silently skipped — CO-1).
const noConflictReportDisclosure = "conflict rows and exemption bounds were not evaluated by this projection: no policy-conflict report was supplied"

// eventualInput is deriveEventual's complete, already-gathered input. Every
// field is a value the caller (ProjectWith) resolved before calling in —
// deriveEventual itself performs no I/O, no wall-clock read, and no
// randomness (DC-11), so the same input always derives byte-identical
// output.
type eventualInput struct {
	// Class is the target's class ("feature" or "story"). The three
	// feature-only sources (stub reconciliation, the outcome floor, claimed
	// questions) derive only when Class == "feature" (R-RR1-3).
	Class string
	// Owner is shared by every derived blocker (mirrors deriveBlockers'
	// own single-owner convention).
	Owner Owner
	// Candidates are the target's CURRENT candidate transitions
	// (candidateTransitions's own output) — consulted only to defensively
	// skip a LaterTransitions entry that duplicates one of them, so a
	// later-transition obligation/principal blocker can never collide with
	// (or shadow) a current one.
	Candidates []model.Transition
	// LaterTransitions are the class's lifecycle transitions that are NOT
	// candidates (resolution (a): the model's declared transitions minus
	// Candidates, in the model's declared order) — the caller computes
	// this via laterTransitions. The first entry names the transition
	// every derived item's own Transition field attaches to (mirroring
	// deriveBlockers' own "first candidate" convention for
	// forge-facts-unavailable): a debt named for a feature or a story is
	// framed as blocking whatever comes next in the declared catalog.
	LaterTransitions []model.Transition
	// Spec is the target's own decoded frontmatter — needed for the
	// question-claimed-by-spike source (OpenQuestions x spike Stubs), both
	// feature-only fields no other Facts field carries forward.
	Spec *artifact.SpecFrontmatter
	// Stubs is the feature's stub reconciliation, when computable
	// (nil when the target is not a feature, or reconciliation errored —
	// Facts.EventualDisclosures carries the disclosure for the latter).
	Stubs *evidence.StubReconciliation
	// Fold is the feature's outcome-floor fold, when computable (same nil
	// convention as Stubs).
	Fold *evidence.FeatureResult
	// Conflict is the optional policy-conflict report extra (R-RR1-2).
	// nil means "not supplied" — never "supplied and empty" (a real,
	// empty report still derives its zero conflict-sourced items without
	// the no-report disclosure).
	Conflict *policyconflict.Report
	// Disclosures are pre-existing disclosures the caller already resolved
	// (Facts.EventualDisclosures: a stub-reconciliation or outcome-floor
	// fold error) — merged into the returned section's own Disclosures.
	Disclosures []string
}

// laterTransitions returns class's declared lifecycle transitions that are
// NOT in candidates (resolution (a) of task-1-brief.md), in the model's
// own declared order — never candidateTransitions' verb-sorted order. A
// class the model declares no lifecycle for yields nil, same as
// candidateTransitions' own classDeclared-false reading.
func laterTransitions(mdl *model.Model, class string, candidates []model.Transition) []model.Transition {
	lifecycle, ok := mdl.Lifecycle[class]
	if !ok {
		return nil
	}
	isCandidate := make(map[string]bool, len(candidates))
	for _, tr := range candidates {
		isCandidate[tr.Verb] = true
	}
	var out []model.Transition
	for _, tr := range lifecycle.Transitions {
		if !isCandidate[tr.Verb] {
			out = append(out, tr)
		}
	}
	return out
}

// deriveEventual derives the record's eventual-blocker section
// (spec/readiness-recovery ac-1, parent spec/guided-lifecycle-governance-v3
// AC-6/DC-11): the named debts that will block a later transition, drawn
// ONLY from already-declared requirements — never a forecast of
// unimplemented behavior or a manufactured future failure. The section is
// always Derived: true (a partial derivation still discloses what it could
// not evaluate — CO-1 — rather than presenting the whole section as
// underived).
func deriveEventual(in eventualInput) EventualBlockers {
	// F-verb: the transition every derived item's Transition field names —
	// the first later transition in the model's declared order, or the
	// literal "unknown" when none exists (mirrors deriveBlockers'
	// forge-facts-unavailable "first candidate, else unknown" fallback).
	verb := "unknown"
	if len(in.LaterTransitions) > 0 {
		verb = in.LaterTransitions[0].Verb
	}
	candidateVerbs := make(map[string]bool, len(in.Candidates))
	for _, tr := range in.Candidates {
		candidateVerbs[tr.Verb] = true
	}

	var items []Blocker
	seen := map[string]bool{}
	add := func(bs ...Blocker) {
		for _, b := range bs {
			if seen[b.ID] {
				continue
			}
			seen[b.ID] = true
			items = append(items, b)
		}
	}

	disclosures := append([]string(nil), in.Disclosures...)

	if in.Class == "feature" {
		add(stubUnreconciledBlockers(in, verb)...)
		add(outcomeFloorBlockers(in, verb)...)
		add(questionClaimedBlockers(in, verb)...)
	}

	for _, tr := range in.LaterTransitions {
		if candidateVerbs[tr.Verb] {
			// Defensive: a later transition that duplicates a candidate
			// must never shadow (or collide with) that candidate's own
			// current blocker.
			continue
		}
		add(laterObligationBlockers(tr, in.Owner)...)
		if transitionHasCountersign(tr) {
			add(laterPrincipalBlocker(tr, in.Owner))
		}
	}

	if in.Conflict != nil {
		mech, mechDisc := conflictMechanicalBlockers(in.Conflict, verb, in.Owner)
		add(mech...)
		disclosures = append(disclosures, mechDisc...)

		sem, semDisc := conflictSemanticBlockers(in.Conflict, verb, in.Owner)
		add(sem...)
		disclosures = append(disclosures, semDisc...)

		exempt, exemptDisc := exemptionIneffectiveBlockers(in.Conflict, verb, in.Owner)
		add(exempt...)
		disclosures = append(disclosures, exemptDisc...)
	} else {
		disclosures = append(disclosures, noConflictReportDisclosure)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if items == nil {
		items = []Blocker{}
	}

	return EventualBlockers{
		Derived:     true,
		Items:       items,
		Disclosures: sortDedupStrings(disclosures),
	}
}

// --- source 1: stub reconciliation ----------------------------------------

// spikeStubSlugs returns the set of spec's own Spike-flagged stub slugs —
// consulted so a spike stub's reconciliation bucket (always unreconciled
// through evidence.ReconcileStubs' AC-only realization mechanism, since a
// spike stub declares no acceptance_criteria) is never ALSO surfaced as a
// stub-unreconciled item: its debt is question-claimed-by-spike instead
// (source 3), one blocker per underlying debt.
func spikeStubSlugs(spec *artifact.SpecFrontmatter) map[string]bool {
	out := map[string]bool{}
	if spec == nil {
		return out
	}
	for _, s := range spec.Stubs {
		if s.Spike {
			out[s.Slug] = true
		}
	}
	return out
}

func stubUnreconciledBlockers(in eventualInput, verb string) []Blocker {
	if in.Stubs == nil {
		return nil
	}
	spikeSlugs := spikeStubSlugs(in.Spec)
	var out []Blocker
	for _, sr := range in.Stubs.Stubs {
		if sr.Bucket != evidence.StubUnreconciled || spikeSlugs[sr.Slug] {
			continue
		}
		out = append(out, Blocker{
			ID:        "stub-unreconciled/" + sr.Slug,
			Reason:    ReasonStubUnreconciled,
			Class:     ClassMechanical,
			Witnesses: []string{fmt.Sprintf("stub %s: unreconciled (no realized-by coverage, no withdrawal note)", sr.Slug)},
			Owner:     in.Owner,
			// vocab:identity — 03 §Stub reconciliation's own closure-gate clearing prose (task-1-brief.md's verbatim sentence), naming the implementing-spec class informally, not routed through *model.Model since deriveEventual carries no model reference
			ClearingCondition: fmt.Sprintf("reconcile stub %s: instantiate a story that claims it, or withdraw it with a note", sr.Slug),
			Transition:        verb,
		})
	}
	return out
}

// --- source 2: the feature outcome floor -----------------------------------

// eventualFeatureName resolves "the spec's name (the name half of its
// ref)" resolution (f) of task-1-brief.md names for the outcome-floor
// clearing condition's attestations/<feature-name>/<ac>.md path — the same
// meaning evidence.FeatureInput.FeatureSlug documents. Spec is preferred
// (the authoritative source); Fold.SpecRef (evidence.FoldFeature always
// sets this to the folded spec's own ID) is the fallback for a caller that
// supplies Fold without Spec.
func eventualFeatureName(in eventualInput) string {
	if in.Spec != nil {
		if ref, err := artifact.ParseRef(in.Spec.ID); err == nil {
			return ref.Name
		}
	}
	if in.Fold != nil {
		if ref, err := artifact.ParseRef(in.Fold.SpecRef); err == nil {
			return ref.Name
		}
	}
	return ""
}

func outcomeFloorWitness(ac evidence.FeatureACResult) string {
	if ac.Floor.Violating != nil && ac.Floor.Violating.Witness != "" {
		return fmt.Sprintf("AC %s: a current outcome record failed: %s", ac.ID, ac.Floor.Violating.Witness)
	}
	if ac.Floor.DeclaresAttestation {
		return fmt.Sprintf("AC %s: outcome floor unsatisfied; attestation is %s", ac.ID, ac.Floor.Attestation)
	}
	return fmt.Sprintf("AC %s: outcome floor unsatisfied; no passing outcome record and no attestation declared", ac.ID)
}

func outcomeFloorBlockers(in eventualInput, verb string) []Blocker {
	if in.Fold == nil {
		return nil
	}
	featureName := eventualFeatureName(in)
	var out []Blocker
	for _, ac := range in.Fold.ACs {
		if ac.Floor.Satisfied {
			continue
		}
		out = append(out, Blocker{
			ID:                "outcome-floor/" + ac.ID,
			Reason:            ReasonOutcomeFloorUnsatisfied,
			Class:             ClassJudgmental,
			Witnesses:         []string{outcomeFloorWitness(ac)},
			Owner:             in.Owner,
			ClearingCondition: fmt.Sprintf("author attestations/%s/%s.md or land a passing outcome record for %s", featureName, ac.ID, ac.ID),
			Transition:        verb,
		})
	}
	return out
}

// --- source 3: an open question claimed by a spike stub --------------------

func questionClaimedBlockers(in eventualInput, verb string) []Blocker {
	if in.Spec == nil || len(in.Spec.OpenQuestions) == 0 {
		return nil
	}
	claimedBy := map[string][]string{}
	for _, s := range in.Spec.Stubs {
		if !s.Spike {
			continue
		}
		for _, oq := range s.Resolves {
			claimedBy[oq] = append(claimedBy[oq], s.Slug)
		}
	}

	var out []Blocker
	for _, oq := range in.Spec.OpenQuestions {
		stubs := claimedBy[oq.ID]
		if len(stubs) == 0 {
			// An UNCLAIMED question is not a journey blocker at all (the
			// plan's own R-RR1-1 note): it stays the readiness-shape
			// concern it already is, never counted twice.
			continue
		}
		sortedStubs := append([]string(nil), stubs...)
		sort.Strings(sortedStubs)
		witnesses := make([]string, 0, len(sortedStubs))
		for _, s := range sortedStubs {
			// vocab:identity — names the Stub.Spike frontmatter field/stub-kind discriminator (02 §Kind registry round 5.4) and its resolves edge; identity-layer field/edge names, not model display prose
			witnesses = append(witnesses, fmt.Sprintf("spike stub %s claims %s via its resolves edge", s, oq.ID))
		}
		out = append(out, Blocker{
			ID:        "question-claimed/" + oq.ID,
			Reason:    ReasonQuestionClaimedBySpike,
			Class:     ClassMechanical,
			Witnesses: sortDedupStrings(witnesses),
			Owner:     in.Owner,
			// vocab:identity — task-1-brief.md's verbatim clearing-condition sentence, naming the Stub.Spike field/resolves edge and (literal prose) the closure the open question must be resolved before; deriveEventual carries no *model.Model to route the word through
			ClearingCondition: fmt.Sprintf("spike stub %s resolves %s before close", strings.Join(sortedStubs, ", "), oq.ID),
			Transition:        verb,
		})
	}
	return out
}

// --- source 4/5: obligation and principal blockers at a later transition --

// laterObligationBlockers mirrors deriveBlockers' own per-obligation loop
// byte-for-byte (same witness/clearing-condition wording), applied to a
// LATER transition tr instead of a candidate one — obligationBlockerID's
// own verb segment is what keeps every id distinct from its current
// counterpart.
func laterObligationBlockers(tr model.Transition, owner Owner) []Blocker {
	var out []Blocker
	for _, ob := range tr.Obligations {
		reason, idPrefix := obligationReason(ob.Kind)
		class, err := reason.Class()
		if err != nil {
			// obligationReason only ever returns a registered code (see
			// deriveBlockers' identical invariant) — fail loudly rather
			// than silently defaulting a class.
			panic(fmt.Sprintf("journey: obligationReason returned unregistered reason %q: %v", reason, err))
		}
		out = append(out, Blocker{
			ID:     obligationBlockerID(idPrefix, tr.Verb, ob.Scheme, ob.Kind),
			Reason: reason,
			Class:  class,
			Witnesses: []string{fmt.Sprintf(
				"obligation %s/%s for transition %s (%s -> %s) is not proven by this projection; obligation gates are not yet journey contributors",
				ob.Scheme, ob.Kind, tr.Verb, tr.From, tr.To,
			)},
			Owner:             owner,
			ClearingCondition: fmt.Sprintf("obligation %s/%s is proven for transition %s", ob.Scheme, ob.Kind, tr.Verb),
			Transition:        tr.Verb,
		})
	}
	return out
}

// laterPrincipalBlocker mirrors deriveBlockers' own countersign-gated
// principal blocker, for a later transition. Unlike deriveBlockers, this
// package has no profile-adopted fact available at derivation time
// (eventualInput carries none), so the witness states only what is always
// true of a later transition: its principal resolution is unproven,
// without claiming a profile-adoption posture it cannot itself observe.
func laterPrincipalBlocker(tr model.Transition, owner Owner) Blocker {
	return Blocker{
		ID:                "principal-resolution-unproven/" + tr.Verb,
		Reason:            ReasonPrincipalResolutionUnproven,
		Class:             ClassGovernance,
		Witnesses:         []string{fmt.Sprintf("authenticated principal resolution for transition %s remains unproven; principal resolution is not yet a journey contributor", tr.Verb)},
		Owner:             owner,
		ClearingCondition: "the required principals resolve as authenticated",
		Transition:        tr.Verb,
	}
}

// --- sources 6/7/8: the policy-conflict report ------------------------------

// conflictIDInvalidRe matches any run of characters outside [a-z0-9] — the
// mechanical half of resolution (d)'s row/exemption-id normalization.
var conflictIDInvalidRe = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeConflictID lowercases raw and maps every run of characters
// outside [a-z0-9] to a single '-', trimming leading/trailing '-'
// (resolution (d)): policyconflict's own row and exemption ids are
// validated only as non-empty strings (internal/policyconflict/
// validate.go's validateNonEmpty), never constrained to this package's
// blockerIDRe grammar. An empty result, or one that does not start with a
// lowercase letter, is prefixed "row-" so the composed blocker id always
// satisfies ^[a-z][a-z0-9-]*$.
func sanitizeConflictID(raw string) string {
	s := conflictIDInvalidRe.ReplaceAllString(strings.ToLower(raw), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "row"
	}
	if s[0] < 'a' || s[0] > 'z' {
		s = "row-" + s
	}
	return s
}

// dedupeConflictIDs sanitizes each raw id in report order, appending -2,
// -3, ... deterministically (resolution (d)) when normalization collides
// two DISTINCT raw ids onto the same sanitized form; the first occurrence
// of any sanitized id keeps its bare form. Returns the resolved id per
// input index, plus one disclosure sentence per collision.
func dedupeConflictIDs(raw []string) (ids []string, disclosures []string) {
	counts := map[string]int{}
	ids = make([]string, len(raw))
	for i, r := range raw {
		base := sanitizeConflictID(r)
		counts[base]++
		id := base
		if counts[base] > 1 {
			id = fmt.Sprintf("%s-%d", base, counts[base])
			disclosures = append(disclosures, fmt.Sprintf("conflict-report row/exemption id %q normalized to %q, which collided with an earlier row; disambiguated as %q", r, base, id))
		}
		ids[i] = id
	}
	return ids, disclosures
}

func joinPolicyReasons(reasons []policyconflict.ReasonCode) string {
	if len(reasons) == 0 {
		return "none disclosed"
	}
	ss := make([]string, len(reasons))
	for i, r := range reasons {
		ss[i] = string(r)
	}
	return strings.Join(ss, ", ")
}

func conflictMechanicalBlockers(report *policyconflict.Report, verb string, owner Owner) ([]Blocker, []string) {
	raw := make([]string, len(report.Mechanical))
	for i, m := range report.Mechanical {
		raw[i] = m.ID
	}
	ids, disclosures := dedupeConflictIDs(raw)

	var out []Blocker
	for i, m := range report.Mechanical {
		if m.State == policyconflict.ProofProven {
			continue
		}
		out = append(out, Blocker{
			ID:                "conflict-mechanical/" + ids[i],
			Reason:            ReasonConflictMechanicalUnresolved,
			Class:             ClassMechanical,
			Witnesses:         []string{fmt.Sprintf("policy-conflict report mechanical row %s is %s (reasons: %s)", m.ID, m.State, joinPolicyReasons(m.Reasons))},
			Owner:             owner,
			ClearingCondition: fmt.Sprintf("resolve conflict row %s or record a disposition", m.ID),
			Transition:        verb,
		})
	}
	return out, disclosures
}

func conflictSemanticBlockers(report *policyconflict.Report, verb string, owner Owner) ([]Blocker, []string) {
	raw := make([]string, len(report.Semantic))
	for i, s := range report.Semantic {
		raw[i] = s.ID
	}
	ids, disclosures := dedupeConflictIDs(raw)

	var out []Blocker
	for i, s := range report.Semantic {
		if s.State == policyconflict.ProofProven {
			continue
		}
		out = append(out, Blocker{
			ID:                "conflict-semantic/" + ids[i],
			Reason:            ReasonConflictSemanticUnresolved,
			Class:             ClassJudgmental,
			Witnesses:         []string{fmt.Sprintf("policy-conflict report semantic row %s is %s (reasons: %s)", s.ID, s.State, joinPolicyReasons(s.Reasons))},
			Owner:             owner,
			ClearingCondition: fmt.Sprintf("resolve conflict row %s or record a disposition", s.ID),
			Transition:        verb,
		})
	}
	return out, disclosures
}

// exemptionIneffective reports whether an exemption's authority resolution
// is NOT effective — resolution (e): "ineffective when Resolution.Bound or
// Resolution.Freshness is not proven."
func exemptionIneffective(res policyconflict.AuthorityResolution) bool {
	return res.Bound != policyconflict.ProofProven || res.Freshness != policyconflict.ProofProven
}

func exemptionIneffectiveBlockers(report *policyconflict.Report, verb string, owner Owner) ([]Blocker, []string) {
	var raw []string
	var resolutions []policyconflict.ExemptionResolution
	for _, m := range report.Mechanical {
		for _, ex := range m.Exemptions {
			raw = append(raw, ex.ID)
			resolutions = append(resolutions, ex)
		}
	}
	ids, disclosures := dedupeConflictIDs(raw)

	var out []Blocker
	for i, ex := range resolutions {
		if !exemptionIneffective(ex.Resolution) {
			continue
		}
		out = append(out, Blocker{
			ID:                "exemption-ineffective/" + ids[i],
			Reason:            ReasonExemptionIneffective,
			Class:             ClassGovernance,
			Witnesses:         []string{fmt.Sprintf("exemption %s: bound=%s freshness=%s", ex.ID, ex.Resolution.Bound, ex.Resolution.Freshness)},
			Owner:             owner,
			ClearingCondition: fmt.Sprintf("renew or replace exemption %s: its bound or freshness is not proven", ex.ID),
			Transition:        verb,
		})
	}
	return out, disclosures
}
