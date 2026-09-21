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
	"github.com/jyang234/verdi/internal/specstate"
)

// noConflictReportDisclosure is R-RR1-2's fixed disclosure: with no
// conflict report supplied, the three conflict-derived sources (mechanical,
// semantic, exemption) are not evaluated, but the eventual section is
// still Derived: true over everything else it CAN see (DC-11: a debt named
// earlier is still a debt, never manufactured, and an unavailable input is
// disclosed, never silently skipped — CO-1).
const noConflictReportDisclosure = "conflict rows and exemption bounds were not evaluated by this projection: no policy-conflict report was supplied"

// storyClassID and spikeClassID are the two class IDS whose DISPLAY words
// appear in a clearing condition below. They are identity-layer ids (the
// operating model's own class keys); every use resolves through
// model.DisplayClass before it reaches an operator's eye, so a store that
// renamed its vocabulary never sees the bare word.
const (
	storyClassID = "story"
	spikeClassID = "spike"
)

// acceptedStatus and closedStatus are the two lifecycle STATE ids this
// derivation resolves transitions by TARGET rather than by verb name
// (R-RR1-12, ledger SI-207: "do not hard-code merge" — a store may rename
// a verb's display word, and a later model may name the acceptance
// transition differently). Both are joined through specstate's own
// Result.ArtifactStatus mapping (DC-15), never written here as a second
// literal-to-literal status table of this package's own.
var (
	acceptedStatus = string(specstate.Result{State: specstate.AcceptedPendingBuild}.ArtifactStatus())
	closedStatus   = string(specstate.Result{State: specstate.Closed}.ArtifactStatus())
)

// eventualInput is deriveEventual's complete, already-gathered input. Every
// field is a value the caller (ProjectWith) resolved before calling in —
// deriveEventual itself performs no I/O, no wall-clock read, and no
// randomness (DC-11), so the same input always derives byte-identical
// output.
type eventualInput struct {
	// Class is the target's class ("feature" or "story"). The three
	// feature-only sources (stub reconciliation, the outcome floor,
	// claimed questions) derive only when Class is the feature class
	// (R-RR1-3).
	Class string
	// Model is the operating-model catalog: the ONLY source of transition
	// verbs and lifecycle shape (DC-3), and the display chain every class
	// and verb WORD in a derived clearing condition routes through (co-5).
	// A nil model declares no lifecycle for any class, exactly as an
	// absent key does.
	Model *model.Model
	// State is the target's CURRENT lifecycle state id, joined through
	// specstate.Result.ArtifactStatus() by the caller (DC-15) — the origin
	// of R-RR1-11's forward-reachability walk. A state the class's
	// lifecycle does not declare (specstate's own "unproven") reaches
	// nothing and is disclosed.
	State string
	// Owner is shared by every derived blocker (mirrors deriveBlockers'
	// own single-owner convention).
	Owner Owner
	// Candidates are the target's CURRENT candidate transitions
	// (candidateTransitions's own output) — consulted only to defensively
	// skip a LaterTransitions entry that duplicates one of them, so a
	// later-transition obligation/principal blocker can never collide with
	// (or shadow) a current one.
	Candidates []model.Transition
	// LaterTransitions are the transitions FORWARD-REACHABLE from State in
	// the class's lifecycle, minus Candidates — R-RR1-11's set, which the
	// caller computes via laterTransitions. Each one contributes its own
	// obligation and principal debts, naming its own verb; a transition
	// already behind the state is never in this set, so an accepted spec
	// never carries the acceptance verb as a debt.
	LaterTransitions []model.Transition
	// Spec is the target's own decoded frontmatter — needed for the
	// question-claimed-by-spike source (OpenQuestions x spike Stubs), both
	// feature-only fields no other Facts field carries forward.
	Spec *artifact.SpecFrontmatter
	// Stubs is the feature's stub reconciliation, when computable
	// (nil when the target is not a feature, or reconciliation errored —
	// Facts.EventualUnavailable names the source for the latter).
	Stubs *evidence.StubReconciliation
	// Fold is the feature's outcome-floor fold, when computable (same nil
	// convention as Stubs).
	Fold *evidence.FeatureResult
	// Conflict is the optional policy-conflict report extra (R-RR1-2).
	// nil means "not supplied" — never "supplied and empty" (a real,
	// empty report still derives its zero conflict-sourced items without
	// the no-report disclosure).
	Conflict *policyconflict.Report
	// Unavailable names the eventual SOURCES the caller could not compute
	// at all (Facts.EventualUnavailable: a stub-reconciliation or
	// outcome-floor fold error) — carried through to the returned
	// section's own Unavailable list, never merged into its Disclosures
	// (SI-213 / R-RRF-1: a partial derivation must stay distinguishable
	// from a complete one).
	Unavailable []string
}

// lifecycleFor returns class's declared lifecycle. A nil model, a nil
// Lifecycle map and an absent key all read the same way — "declared for
// nothing" — never a panic (candidateTransitions' own reading).
func lifecycleFor(mdl *model.Model, class string) (model.Lifecycle, bool) {
	if mdl == nil {
		return model.Lifecycle{}, false
	}
	lifecycle, ok := mdl.Lifecycle[class]
	return lifecycle, ok
}

// reachableStates returns the lifecycle states reachable from state by
// following declared transitions FORWARD (state itself included). A state
// the lifecycle does not declare — specstate's "unproven" status, or any
// state outside this class's own machine — reaches nothing: there is no
// from-state to walk from, and inventing one would forecast a lifecycle
// the target is not in. The walk runs to a fixed point over a membership
// map, so its result never depends on map iteration order.
func reachableStates(lifecycle model.Lifecycle, state string) map[string]bool {
	reached := map[string]bool{}
	for _, s := range lifecycle.States {
		if s == state {
			reached[state] = true
			break
		}
	}
	if len(reached) == 0 {
		return reached
	}
	for changed := true; changed; {
		changed = false
		for _, tr := range lifecycle.Transitions {
			if reached[tr.From] && !reached[tr.To] {
				reached[tr.To] = true
				changed = true
			}
		}
	}
	return reached
}

// laterTransitions returns the transitions FORWARD-REACHABLE from state in
// class's declared lifecycle, minus the immediate candidates — R-RR1-11
// (ledger SI-207), superseding the original "every declared transition
// minus the candidates" reading, which named transitions already BEHIND
// the state (an accepted feature's own acceptance verb among them). Order
// is the model's own declared transition order, never candidateTransitions'
// verb-sorted order. Candidates are subtracted by VERB, which is also what
// keeps an eventual obligation id (whose segments are verb/scheme/kind)
// distinct from its current counterpart. A class the model declares no
// lifecycle for yields nil, same as candidateTransitions' own
// classDeclared-false reading.
func laterTransitions(mdl *model.Model, class, state string, candidates []model.Transition) []model.Transition {
	lifecycle, ok := lifecycleFor(mdl, class)
	if !ok {
		return nil
	}
	reached := reachableStates(lifecycle, state)
	isCandidate := make(map[string]bool, len(candidates))
	for _, tr := range candidates {
		isCandidate[tr.Verb] = true
	}
	var out []model.Transition
	for _, tr := range lifecycle.Transitions {
		if !reached[tr.From] || isCandidate[tr.Verb] {
			continue
		}
		out = append(out, tr)
	}
	return out
}

// transitionToState returns the first declared transition whose target is
// state, in the model's own declared order. The whole transition (not just
// its verb) is returned because R-RR1-13 also needs its FROM-state: a
// closure gate is only ahead of the target when its from-state is still
// forward-reachable.
func transitionToState(lifecycle model.Lifecycle, state string) (model.Transition, bool) {
	for _, tr := range lifecycle.Transitions {
		if tr.To == state {
			return tr, true
		}
	}
	return model.Transition{}, false
}

// eventualScopeResult carries R-RR1-12's per-source verb table for one
// class at one lifecycle state, plus the disclosure (if any) naming what
// could not be resolved. resolved false means no verb could be named at
// all, so every source that must name one derives nothing — the literal
// "unknown" is never emitted (SI-207).
type eventualScopeResult struct {
	closureVerb string
	policyVerb  string
	resolved    bool
	// closureAhead is R-RR1-13: true when the closure transition's own
	// FROM-state is still forward-reachable from the target's current
	// state, i.e. the closure gate has yet to run. False for a target that
	// is already closed or superseded — the three closure-gated feature
	// sources then derive nothing, because the gate that would have
	// consumed those requirements has already run and cannot run again. A
	// state the lifecycle does not declare leaves this TRUE: nothing there
	// proves the gate is behind, and an unproven absence is never reported
	// as a proven one.
	closureAhead bool
	disclosure   string
}

// resolveEventualScope resolves R-RR1-12's verb table (ledger SI-207):
//
//   - closureVerb — the transition whose target is the CLOSED state. Stub
//     reconciliation, the outcome floor and a spike-claimed question are
//     all consumed by the closure gate (the parent's AC-6 makes them
//     eventual even when close is the next legal transition), so each
//     names this verb whatever the current state.
//   - policyVerb — the EARLIEST forward-reachable transition whose gate
//     evaluates policy: the ACCEPTANCE transition (resolved as the one
//     whose target is the accepted state, never the hard-coded literal
//     "merge") while acceptance is still ahead of the target, the closure
//     verb once acceptance is behind it. Conflict rows and ineffective
//     exemptions name it.
//
// A class with no declared lifecycle — or a lifecycle declaring no
// closure transition at all — resolves nothing and discloses why; a
// declared lifecycle whose current state is not one of its own declared
// states (specstate's "unproven") still resolves both verbs, with the
// narrowing it could not perform disclosed.
func resolveEventualScope(mdl *model.Model, class, state string) eventualScopeResult {
	lifecycle, ok := lifecycleFor(mdl, class)
	if !ok {
		return eventualScopeResult{disclosure: noClosureTransitionDisclosure(mdl, class)}
	}
	closure, ok := transitionToState(lifecycle, closedStatus)
	if !ok {
		return eventualScopeResult{disclosure: noClosureTransitionDisclosure(mdl, class)}
	}

	out := eventualScopeResult{closureVerb: closure.Verb, policyVerb: closure.Verb, resolved: true, closureAhead: true}
	reached := reachableStates(lifecycle, state)
	if len(reached) == 0 {
		out.disclosure = stateNotDeclaredDisclosure(mdl, class, state)
		return out
	}
	// R-RR1-13: the closure gate lies AHEAD only while its own from-state
	// is still reachable. From a terminal state (closed, superseded) it is
	// not, so the closure-gated sources below derive nothing.
	out.closureAhead = reached[closure.From]
	for _, tr := range lifecycle.Transitions {
		if tr.To == acceptedStatus && reached[tr.From] {
			out.policyVerb = tr.Verb
			break
		}
	}
	return out
}

// noClosureTransitionDisclosure names the sources a missing closure
// transition silences. The class word routes through the display chain
// (co-5) — with the model in scope there is nothing to mark.
func noClosureTransitionDisclosure(mdl *model.Model, class string) string {
	classWord := mdl.DisplayClass(class)
	return fmt.Sprintf(
		"the operating model declares no closure transition for %s: stub, outcome-floor, claimed-question and policy-conflict debts were not derived for this target",
		classWord,
	)
}

// stateNotDeclaredDisclosure names what an unresolvable from-state costs:
// no transition can be walked forward from it, so no later-transition debt
// is derived and the earliest policy-evaluating transition cannot be
// narrowed below the closure gate.
func stateNotDeclaredDisclosure(mdl *model.Model, class, state string) string {
	classWord := mdl.DisplayClass(class)
	return fmt.Sprintf(
		"lifecycle state %q is not a declared state of the %s lifecycle: no later transition was derived from it, and the earliest policy-evaluating transition could not be narrowed below closure",
		state, classWord,
	)
}

// noPolicyGateAheadDisclosure is R-RR1-19's one sentence: a supplied
// policy-conflict report carries real findings, so a reader could
// reasonably expect them as eventual debt. When no policy-evaluating
// transition lies ahead of the target, those findings are NOT derived —
// and unlike R-RR1-13's closure-gated feature sources, that absence is
// disclosed rather than silent (CO-1/co-6), because the input exists and
// only the gate that would consume it has gone. The state word routes
// through the display chain (co-5); the format string itself carries no
// class, state or verb word of its own.
func noPolicyGateAheadDisclosure(mdl *model.Model, class, state string) string {
	stateWord := mdl.DisplayState(class, state)
	return fmt.Sprintf(
		"no policy-evaluating transition lies ahead of state %s: policy-conflict findings were not derived as eventual debt",
		stateWord,
	)
}

// duplicateBlockerIDDisclosure is the last line of defence for CO-1: two
// derived debts that resolve to one blocker id cannot both be listed (the
// record's ids are unique by schema), so the one that is dropped is named
// rather than silently lost.
func duplicateBlockerIDDisclosure(id string) string {
	return fmt.Sprintf("two derived debts resolved to the same blocker id %q; only the first is listed", id)
}

// deriveEventual derives the record's eventual-blocker section
// (spec/readiness-recovery ac-1, parent spec/guided-lifecycle-governance-v3
// AC-6/DC-11): the named debts that will block a later transition, drawn
// ONLY from already-declared requirements — never a forecast of
// unimplemented behavior or a manufactured future failure. Each item names
// the transition whose gate CONSUMES it (R-RR1-12), never a shared "first
// later verb" and never the literal "unknown". The section is always
// Derived: true (a partial derivation still NAMES what it could not
// evaluate, in Unavailable — CO-1/co-6 — rather than presenting the whole
// section as underived).
func deriveEventual(in eventualInput) EventualBlockers {
	var items []Blocker
	var disclosures []string
	seen := map[string]bool{}
	add := func(bs ...Blocker) {
		for _, b := range bs {
			if seen[b.ID] {
				disclosures = append(disclosures, duplicateBlockerIDDisclosure(b.ID))
				continue
			}
			seen[b.ID] = true
			items = append(items, b)
		}
	}

	scope := resolveEventualScope(in.Model, in.Class, in.State)
	if scope.disclosure != "" {
		disclosures = append(disclosures, scope.disclosure)
	}

	// R-RR1-13: a closed feature carries no closure debt. All three
	// sources below are consumed by the CLOSURE gate, so once that gate is
	// behind the state they derive nothing — a proven absence, which needs
	// no disclosure (the section stays Derived: true either way).
	if scope.resolved && scope.closureAhead && in.Class == string(artifact.ClassFeature) {
		add(stubUnreconciledBlockers(in, scope.closureVerb)...)
		add(outcomeFloorBlockers(in, scope.closureVerb)...)
		add(questionClaimedBlockers(in, scope.closureVerb)...)
	}

	candidateVerbs := make(map[string]bool, len(in.Candidates))
	for _, tr := range in.Candidates {
		candidateVerbs[tr.Verb] = true
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

	switch {
	case in.Conflict == nil:
		// The report itself is absent. This branch is about the missing
		// INPUT, never about the target's state, so it is untouched by
		// R-RR1-19 below.
		disclosures = append(disclosures, noConflictReportDisclosure)
	case !scope.resolved:
		// No verb could be named at all; scope.disclosure above already
		// says why, and adding a second sentence would double-report it.
	case !scope.closureAhead:
		// R-RR1-19: R-RR1-13 extended to the policy sources. Each of the
		// three below names the earliest forward-reachable transition
		// whose gate EVALUATES policy, and resolveEventualScope narrows
		// that to the acceptance transition only while acceptance is
		// itself reachable — which it cannot be once the closure gate is
		// behind, since acceptance precedes closure in the lifecycle.
		// With no policy gate ahead there is no gate to owe these
		// findings to, so none is derived; the report still exists, so
		// the absence is disclosed rather than silent.
		disclosures = append(disclosures, noPolicyGateAheadDisclosure(in.Model, in.Class, in.State))
	default:
		mech, mechDisc := conflictMechanicalBlockers(in.Conflict, scope.policyVerb, in.Owner)
		add(mech...)
		disclosures = append(disclosures, mechDisc...)

		sem, semDisc := conflictSemanticBlockers(in.Conflict, scope.policyVerb, in.Owner)
		add(sem...)
		disclosures = append(disclosures, semDisc...)

		exempt, exemptDisc := exemptionIneffectiveBlockers(in.Conflict, scope.policyVerb, in.Owner)
		add(exempt...)
		disclosures = append(disclosures, exemptDisc...)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if items == nil {
		items = []Blocker{}
	}

	return EventualBlockers{
		Derived:     true,
		Items:       items,
		Disclosures: sortDedupStrings(disclosures),
		Unavailable: sortDedupStrings(in.Unavailable),
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
	storyWord := in.Model.DisplayClass(storyClassID)
	spikeSlugs := spikeStubSlugs(in.Spec)
	var out []Blocker
	for _, sr := range in.Stubs.Stubs {
		if sr.Bucket != evidence.StubUnreconciled || spikeSlugs[sr.Slug] {
			continue
		}
		out = append(out, Blocker{
			// C1: a stub slug is validated by internal/artifact's own
			// simpleNameRe (^[a-z0-9]+(?:-[a-z0-9]+)*$), which admits a
			// LEADING DIGIT, while a blocker-id segment must start with a
			// letter (record.go's blockerIDRe). The id segment is
			// normalized; the witness and clearing condition below name
			// the raw slug, so the operator still reads the slug they
			// wrote.
			ID:        "stub-unreconciled/" + sanitizeStubSlug(sr.Slug),
			Reason:    ReasonStubUnreconciled,
			Class:     ClassMechanical,
			Witnesses: []string{fmt.Sprintf("stub %s: unreconciled (no realized-by coverage, no withdrawal note)", sr.Slug)},
			Owner:     in.Owner,
			// 03 §Stub reconciliation's own closure-gate clearing prose
			// (task-1-brief.md's sentence), with the implementing class
			// word routed through the display chain (co-5).
			ClearingCondition: fmt.Sprintf("reconcile stub %s: instantiate a %s that claims it, or withdraw it with a note", sr.Slug, storyWord),
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
// supplies Fold without Spec. When NEITHER parses as a ref, the raw ref
// text is named rather than composing an empty path segment into the
// clearing condition: a sentence that names something unparseable is still
// actionable, one that names nothing is not.
func eventualFeatureName(in eventualInput) string {
	var raws []string
	if in.Spec != nil && in.Spec.ID != "" {
		raws = append(raws, in.Spec.ID)
	}
	if in.Fold != nil && in.Fold.SpecRef != "" {
		raws = append(raws, in.Fold.SpecRef)
	}
	for _, raw := range raws {
		if ref, err := artifact.ParseRef(raw); err == nil {
			return ref.Name
		}
	}
	if len(raws) > 0 {
		return raws[0]
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

// outcomeFloorClearingCondition composes the floor's remedy from the
// routes that can actually clear it (R-RRF-5, independent review
// 2026-09-21 R5). evidence.foldFeatureAC reads
// attestations/<feature>/<ac>.md only for a criterion that DECLARES the
// attestation evidence kind, so naming that path for a criterion that
// does not declare it advertises a remedy the fold ignores — an
// instruction an operator can follow to completion without clearing the
// debt it names. ac-7's "names the attestation path or a passing outcome
// record" is therefore read as the EFFECTIVE route(s): both when the kind
// is declared, the passing-record route alone when it is not.
func outcomeFloorClearingCondition(ac evidence.FeatureACResult, featureName string) string {
	record := fmt.Sprintf("land a passing outcome record for %s", ac.ID)
	if !ac.Floor.DeclaresAttestation {
		return record
	}
	return fmt.Sprintf("author attestations/%s/%s.md or %s", featureName, ac.ID, record)
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
			ClearingCondition: outcomeFloorClearingCondition(ac, featureName),
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
	spikeWord := in.Model.DisplayClass(spikeClassID)
	closeWord := in.Model.DisplayVerb(verb)

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
			ID:                "question-claimed/" + oq.ID,
			Reason:            ReasonQuestionClaimedBySpike,
			Class:             ClassMechanical,
			Witnesses:         sortDedupStrings(witnesses),
			Owner:             in.Owner,
			ClearingCondition: fmt.Sprintf("%s stub %s resolves %s before %s", spikeWord, strings.Join(sortedStubs, ", "), oq.ID, closeWord),
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

// sanitizeIDSegment lowercases raw and maps every run of characters
// outside [a-z0-9] to a single '-', trimming leading/trailing '-'. An
// empty result, or one that does not start with a lowercase letter, takes
// fallbackPrefix so the composed blocker id always satisfies
// ^[a-z][a-z0-9-]*$ (record.go's blockerIDRe). The transformation is
// deterministic and leaves a letter-led, already-kebab-case input
// byte-identical, so the id an operator reads is still the id they wrote
// wherever the grammar allows it.
func sanitizeIDSegment(raw, fallbackPrefix string) string {
	s := conflictIDInvalidRe.ReplaceAllString(strings.ToLower(raw), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return fallbackPrefix
	}
	if s[0] < 'a' || s[0] > 'z' {
		s = fallbackPrefix + "-" + s
	}
	return s
}

// sanitizeConflictID normalizes a policy-conflict row or exemption id
// (resolution (d)): policyconflict's own ids are validated only as
// non-empty strings (internal/policyconflict/validate.go's
// validateNonEmpty), never constrained to this package's grammar.
func sanitizeConflictID(raw string) string { return sanitizeIDSegment(raw, "row") }

// sanitizeStubSlug normalizes a feature stub slug (C1). A slug is already
// validated by internal/artifact's simpleNameRe
// (^[a-z0-9]+(?:-[a-z0-9]+)*$), so the ONLY reachable normalization is
// the letter prefix a DIGIT-led slug needs ("2fa-login" -> "s-2fa-login");
// every letter-led slug passes through unchanged.
func sanitizeStubSlug(raw string) string { return sanitizeIDSegment(raw, "s") }

// dedupeConflictIDs sanitizes each raw id in report order, appending -2,
// -3, ... deterministically (resolution (d)) when normalization collides
// two DISTINCT raw ids onto the same sanitized form; the first occurrence
// of any sanitized id keeps its bare form. The suffixed candidate is
// itself checked for use and incremented until it is free, so a raw id
// that already reads like an earlier id's disambiguated form can never
// re-collide with it. Returns the resolved id per input index, plus one
// disclosure sentence per collision. Callers pass only the rows that will
// actually produce a blocker, so a skipped (proven) row never consumes an
// id or emits a disclosure with no item behind it.
func dedupeConflictIDs(raw []string) (ids []string, disclosures []string) {
	used := make(map[string]bool, len(raw))
	ids = make([]string, len(raw))
	for i, r := range raw {
		base := sanitizeConflictID(r)
		id := base
		if used[id] {
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s-%d", base, n)
				if !used[candidate] {
					id = candidate
					break
				}
			}
			disclosures = append(disclosures, fmt.Sprintf("conflict-report row/exemption id %q normalized to %q, which collided with an earlier row; disambiguated as %q", r, base, id))
		}
		used[id] = true
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
	var rows []policyconflict.MechanicalEvaluation
	var raw []string
	for _, m := range report.Mechanical {
		if m.State == policyconflict.ProofProven {
			continue
		}
		rows = append(rows, m)
		raw = append(raw, m.ID)
	}
	ids, disclosures := dedupeConflictIDs(raw)

	out := make([]Blocker, 0, len(rows))
	for i, m := range rows {
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
	var rows []policyconflict.SemanticEvaluation
	var raw []string
	for _, s := range report.Semantic {
		if s.State == policyconflict.ProofProven {
			continue
		}
		rows = append(rows, s)
		raw = append(raw, s.ID)
	}
	ids, disclosures := dedupeConflictIDs(raw)

	out := make([]Blocker, 0, len(rows))
	for i, s := range rows {
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
			if !exemptionIneffective(ex.Resolution) {
				continue
			}
			raw = append(raw, ex.ID)
			resolutions = append(resolutions, ex)
		}
	}
	ids, disclosures := dedupeConflictIDs(raw)

	out := make([]Blocker, 0, len(resolutions))
	for i, ex := range resolutions {
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
