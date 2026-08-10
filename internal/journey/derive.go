package journey

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
)

const (
	profileNotAdoptedDisclosure         = "no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown"
	profileResolutionUnprovenDisclosure = "authenticated principal resolution and profile-contributed requirements remain unproven"
)

// candidateTransitions returns cfg.Model.Lifecycle[class]'s transitions
// whose From equals the joined effective status (DC-15: joined via
// specstate.Result.ArtifactStatus() — never a literal-to-literal mapping
// table of this package's own, and never a comparison against spec.Status
// directly), sorted by Verb for determinism, and classDeclared reporting
// whether the model declares a lifecycle for class at all (a nil
// Model.Lifecycle map reads as "declared for nothing", the same as any
// other absent key — never a nil-map panic). classDeclared is checked
// BEFORE state: whether a class has a lifecycle is a fact about the
// operating model, independent of whether this evaluation's own lifecycle
// state happened to resolve. Unproven state, once classDeclared is true,
// still yields no candidates (DC-3: no from-state can be established).
func candidateTransitions(mdl *model.Model, class string, result specstate.Result) (transitions []model.Transition, classDeclared bool) {
	lifecycle, ok := mdl.Lifecycle[class]
	if !ok {
		return nil, false
	}
	if result.State == specstate.Unproven {
		return nil, true
	}
	from := string(result.ArtifactStatus())
	for _, tr := range lifecycle.Transitions {
		if tr.From == from {
			transitions = append(transitions, tr)
		}
	}
	sort.Slice(transitions, func(i, j int) bool { return transitions[i].Verb < transitions[j].Verb })
	return transitions, true
}

func obligationQualityBlockerID(acID string, kind artifact.EvidenceKind) string {
	return obligationQualityBlockerPrefix + acID + "/" + string(kind)
}

// deriveObligationQualityBlockers emits one mechanical blocker for each
// non-elaborated or elaborated-but-unmatched pair. Input order is the source
// spec's AC-then-kind declaration order and is preserved exactly.
func deriveObligationQualityBlockers(facts []ObligationQualityFact, owner Owner) []Blocker {
	out := make([]Blocker, 0)
	seen := map[string]bool{}
	for _, fact := range facts {
		assessment := fact.Assessment
		blocks := assessment.StructuralState != evidence.ObligationElaborated ||
			assessment.MatchState != evidence.ObligationMatched
		if !blocks {
			continue
		}
		id := obligationQualityBlockerID(fact.ACID, fact.Kind)
		if seen[id] {
			continue
		}
		seen[id] = true
		detail := string(assessment.StructuralState)
		if assessment.Reason != "" {
			detail += "/" + string(assessment.Reason)
		}
		out = append(out, Blocker{
			ID:                id,
			Reason:            ReasonObligationDesignUnresolved,
			Class:             ClassMechanical,
			Witnesses:         []string{assessment.WitnessPath + ": " + detail},
			Owner:             owner,
			ClearingCondition: fmt.Sprintf("the obligation quality for %s/%s is elaborated and any positive evidence matches its producer, source, and freshness declaration", fact.ACID, fact.Kind),
			Transition:        buildStartActionIdentity,
		})
	}
	if out == nil {
		return []Blocker{}
	}
	return out
}

// mergeObligationQualityBlockers keeps every incumbent blocker ordered by ID
// and inserts the declaration-ordered quality group at its lexical prefix
// position. Stable sorting deliberately preserves the group's internal order.
func mergeObligationQualityBlockers(current, quality []Blocker) []Blocker {
	out := append(append([]Blocker(nil), current...), quality...)
	orderKey := func(blocker Blocker) string {
		if strings.HasPrefix(blocker.ID, obligationQualityBlockerPrefix) {
			return obligationQualityBlockerPrefix
		}
		return blocker.ID
	}
	sort.SliceStable(out, func(i, j int) bool { return orderKey(out[i]) < orderKey(out[j]) })
	if out == nil {
		return []Blocker{}
	}
	return out
}

// classUndeclaredMessage is F4's shared text: the record-level disclosure
// and the Actions.NeededFacts entry a class absent from Model.Lifecycle
// produces both carry this SAME message (one string, two sinks), so the
// two can never drift apart.
func classUndeclaredMessage(class string) string {
	return fmt.Sprintf("the operating model declares no lifecycle for class %s; its transitions are unknown", class)
}

// obligationReason maps an obligation kind to its fixed ReasonCode and
// blocker-ID prefix (the closed kind -> reason mapping the work order
// names: author-vouch/countersign/fold-green each get their own code; any
// other kind is the first-class "unknown kind" failure to classify, never
// silently folded into one of the three known reasons). The full blocker
// ID additionally carries the transition verb AND the obligation's own
// (scheme, kind) — obligationIDSegments below — so two DISTINCT
// obligations on the same transition (e.g. attestation/fold-green and
// behavioral/fold-green) never collide into one blocker id and silently
// lose a witness to seen-map dedup.
func obligationReason(kind string) (reason ReasonCode, idPrefix string) {
	switch kind {
	case "author-vouch":
		return ReasonObligationAuthorVouchUnproven, "obligation-author-vouch-unproven"
	case "countersign":
		return ReasonObligationCountersignUnproven, "obligation-countersign-unproven"
	case "fold-green":
		return ReasonObligationFoldGreenUnproven, "obligation-fold-green-unproven"
	default:
		return ReasonObligationUnknownKind, "obligation-unknown-kind"
	}
}

// obligationBlockerID composes an obligation blocker's full id:
// "<reason-prefix>/<verb>/<scheme>/<kind>" — blockerIDRe already admits
// arbitrarily many slash segments, and the (scheme, kind) suffix is what
// makes the id unique per DISTINCT obligation rather than per (reason,
// verb) alone (F3). A byte-identical duplicate obligation (same scheme AND
// kind, same transition) still produces the same id and correctly merges
// via deriveBlockers' own seen-map — that is correct dedup, not a
// collision.
func obligationBlockerID(idPrefix, verb, scheme, kind string) string {
	return fmt.Sprintf("%s/%s/%s/%s", idPrefix, verb, scheme, kind)
}

// transitionHasCountersign reports whether tr carries an attestation/
// countersign obligation — DC-18/DC-19's governance-principal gate:
// countersign specifically (never author-vouch, which is judgmental, not
// governance — reason.go's own reasonClasses table draws that same line).
func transitionHasCountersign(tr model.Transition) bool {
	for _, ob := range tr.Obligations {
		if ob.Scheme == "attestation" && ob.Kind == "countersign" {
			return true
		}
	}
	return false
}

// deriveBlockers derives the record's current blockers (AC-1, DC-4): the
// default-branch and lifecycle-state unknowns, one blocker per unproven
// obligation on a candidate transition, the proposed-state forge-facts gap,
// and the countersign-gated principal-resolution gap. Every blocker shares
// the same Owner (the target spec's own declared owners, DC-19's explicit
// unauthenticated attribution — v1 wires no resolver). Terminal states
// (closed/superseded) and a fully-proven, obligation-free candidate set
// naturally yield none of the per-transition blockers; an empty result is
// legal (Blockers.Current may be empty).
func deriveBlockers(defaultBranchKnown, profileAdopted bool, result specstate.Result, candidates []model.Transition, owner Owner) []Blocker {
	var out []Blocker
	seen := map[string]bool{}
	add := func(b Blocker) {
		if seen[b.ID] {
			return
		}
		seen[b.ID] = true
		out = append(out, b)
	}

	if !defaultBranchKnown {
		add(Blocker{
			ID:     "default-branch-unresolved/unknown",
			Reason: ReasonDefaultBranchUnresolved,
			Class:  ClassUnknown,
			// F2: states only what was OBSERVED (DefaultBranch.Known ==
			// false) — never an assertion about which of the three
			// resolution steps individually failed, none of which this
			// projection actually inspects one at a time.
			Witnesses: []string{
				"no default branch could be resolved by the resolution chain (CI_DEFAULT_BRANCH, origin/HEAD symbolic ref, lone conventional remote-tracking ref)",
			},
			Owner:             owner,
			ClearingCondition: "a default branch resolves via CI_DEFAULT_BRANCH, origin/HEAD, or a lone conventional remote-tracking ref",
			Transition:        "unknown",
		})
	}

	if result.State == specstate.Unproven {
		witnesses := sortDedupStrings(result.Disclosures)
		if len(witnesses) == 0 {
			witnesses = []string{"internal/specstate could not prove a lifecycle state and disclosed no further witness"}
		}
		add(Blocker{
			ID:                "lifecycle-state-unproven/unknown",
			Reason:            ReasonLifecycleStateUnproven,
			Class:             ClassUnknown,
			Witnesses:         witnesses,
			Owner:             owner,
			ClearingCondition: "the disclosed lifecycle witnesses resolve and internal/specstate derives a proven state",
			Transition:        "unknown",
		})
	}

	for _, tr := range candidates {
		for _, ob := range tr.Obligations {
			reason, idPrefix := obligationReason(ob.Kind)
			class, err := reason.Class()
			if err != nil {
				// obligationReason only ever returns a registered code; a
				// non-nil error here would mean this package's own reason
				// table and reasonClasses table disagree — fail loudly
				// rather than silently defaulting a class.
				panic(fmt.Sprintf("journey: obligationReason returned unregistered reason %q: %v", reason, err))
			}
			add(Blocker{
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
	}

	if result.State == specstate.Proposed {
		verb := "unknown"
		if len(candidates) > 0 {
			verb = candidates[0].Verb
		}
		add(Blocker{
			ID:     "forge-facts-unavailable/" + verb,
			Reason: ReasonForgeFactsUnavailable,
			Class:  ClassExternalWait,
			Witnesses: []string{
				// vocab:identity — the merge-signals design document's own name and its protocol term ("merge-signaled"), plus the forge's merge state; none is the renameable `merge` transition word
				"acceptance is merge-signaled (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md); this projection consults no forge facts, so review and merge state is unknown",
			},
			Owner:             owner,
			ClearingCondition: "forge facts become available to the projection",
			Transition:        verb,
		})
	}

	for _, tr := range candidates {
		if !transitionHasCountersign(tr) {
			continue
		}
		witness := "no governance profile is adopted at the evaluated revision; authenticated principal resolution is unproven (governance-principal kernel present, no adopted profile artifact)"
		clearingCondition := "a governance profile is adopted and the required principals resolve as authenticated"
		if profileAdopted {
			witness = "a governance profile is adopted, but authenticated principal resolution remains unproven because principal resolution is not yet a journey contributor"
			clearingCondition = "the required principals resolve as authenticated"
		}
		add(Blocker{
			ID:                "principal-resolution-unproven/" + tr.Verb,
			Reason:            ReasonPrincipalResolutionUnproven,
			Class:             ClassGovernance,
			Witnesses:         []string{witness},
			Owner:             owner,
			ClearingCondition: clearingCondition,
			Transition:        tr.Verb,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if out == nil {
		out = []Blocker{}
	}
	return out
}

// deriveEventual returns the record's eventual-blocker section: never
// derived by this delivery unit (journey-projection, GLG's Delivery
// sequence step 1) — a later delivery unit (GLG AC-6, continuous-
// readiness) supplies it. An underived section discloses itself (CO-1).
func deriveEventual() EventualBlockers {
	return EventualBlockers{
		Derived: false,
		Items:   []Blocker{},
		Disclosures: []string{
			"eventual closure blockers are not derived by this delivery unit; a later delivery unit (GLG AC-6, continuous-readiness) supplies them",
		},
	}
}

// derivePrincipals derives the record's principals section: one
// RequiredRole per DISTINCT (transition, obligation) pair drawn from every
// attestation-scheme author-vouch/countersign obligation on a candidate
// transition (behavioral obligations are never principal requirements —
// DC-19's kernel attribution stays confined to actual attestation gates).
// Profile adoption is overlaid by Project after this operating-model-only
// derivation. Resolution remains "unproven": this delivery unit records the
// installed profile but wires no authenticated-principal or profile-rule
// contributor (DC-18's three-valued honesty: never a silent authenticated
// claim).
//
// F6: two obligations that resolve to the SAME (transition, obligation)
// key (a model declaring two countersign obligations on one transition,
// legal per model.Validate — nothing in the kernel forbids it) are
// deduplicated by that key, keeping the LARGER Count, rather than each
// producing its own RequiredRole entry: PrincipalFacts.validate requires
// principals.required to be STRICTLY ascending by (transition,
// obligation), so two entries sharing a key would fail Record.Validate
// and abort the whole projection over a Validate-clean model — the
// correct behavior is one entry that asks for the more demanding count,
// never a projection failure.
func derivePrincipals(candidates []model.Transition) PrincipalFacts {
	byKey := map[string]RequiredRole{}
	var extra []string
	for _, tr := range candidates {
		for _, ob := range tr.Obligations {
			if ob.Scheme != "attestation" {
				continue
			}
			var rr RequiredRole
			switch ob.Kind {
			case "author-vouch":
				rr = RequiredRole{
					Transition: tr.Verb,
					Obligation: "attestation/author-vouch",
					Count:      1,
					Resolution: "unproven",
				}
			case "countersign":
				count := ob.Count
				if count < 1 {
					// F5: the operating model's Obligation.Count is
					// documented "countersign only" (model.go) but carries
					// no floor of its own; a zero or negative value is
					// silently treated as 1, and that assumption is
					// disclosed rather than presented as though the model
					// stated it.
					extra = append(extra, fmt.Sprintf(
						"countersign count is unstated in the operating model for transition %s; a minimum of 1 is assumed",
						tr.Verb,
					))
					count = 1
				}
				rr = RequiredRole{
					Transition: tr.Verb,
					Obligation: "attestation/countersign",
					Count:      count,
					Resolution: "unproven",
				}
			default:
				continue
			}
			key := requiredRoleKey(rr)
			if existing, ok := byKey[key]; !ok || rr.Count > existing.Count {
				byKey[key] = rr
			}
		}
	}

	required := make([]RequiredRole, 0, len(byKey))
	for _, rr := range byKey {
		required = append(required, rr)
	}
	sort.Slice(required, func(i, j int) bool { return requiredRoleKey(required[i]) < requiredRoleKey(required[j]) })

	disclosures := append([]string{profileNotAdoptedDisclosure}, extra...)

	return PrincipalFacts{
		ProfileAdopted: false,
		Required:       required,
		Disclosures:    sortDedupStrings(disclosures),
	}
}

// deriveActions derives the record's safe actions (DC-3): a candidate
// transition becomes safe only when the lifecycle state is proven, the
// transition is registered for the class (both true by construction here
// — candidateTransitions only ever returns registered transitions whose
// From already matches the proven effective status), AND it carries zero
// obligations (any obligation is unprovable by this projection version, so
// the action is excluded and NeededFacts names it instead). A class the
// model declares no lifecycle for at all (F4) yields no candidates and
// classUndeclaredMessage's own NeededFacts entry; otherwise unproven state
// yields no candidates and one fixed NeededFacts entry.
func deriveActions(class string, result specstate.Result, candidates []model.Transition, classDeclared bool, targetRef string) Actions {
	var safe []Action
	var needed []string

	switch {
	case !classDeclared:
		needed = append(needed, classUndeclaredMessage(class))
	case result.State == specstate.Unproven:
		needed = append(needed, "lifecycle state is unproven; no transition's from-state can be established")
	default:
		from := string(result.ArtifactStatus())
		for _, tr := range candidates {
			if len(tr.Obligations) == 0 {
				safe = append(safe, Action{
					ID:           tr.Verb,
					Verb:         tr.Verb,
					Arguments:    []string{targetRef},
					FromState:    tr.From,
					ToState:      tr.To,
					Confirmation: "none",
					Preconditions: []Precondition{
						{
							ID: "lifecycle-state-matches",
							Witness: fmt.Sprintf(
								"the resolved lifecycle state %q matches transition %q's declared from-state %q",
								from, tr.Verb, tr.From,
							),
						},
						{
							ID: "transition-registered",
							Witness: fmt.Sprintf(
								"transition %q is registered for class %q in the operating model",
								tr.Verb, class,
							),
						},
					},
					// Authority names requirements, never proof — with zero
					// obligations on this transition there are none to name.
					Authority: []RequiredRole{},
				})
				continue
			}
			for _, ob := range tr.Obligations {
				needed = append(needed, fmt.Sprintf("obligation %s/%s for transition %s is unproven", ob.Scheme, ob.Kind, tr.Verb))
			}
		}
	}

	sort.Slice(safe, func(i, j int) bool { return safe[i].ID < safe[j].ID })
	if safe == nil {
		safe = []Action{}
	}
	return Actions{Safe: safe, NeededFacts: sortDedupStrings(needed)}
}
