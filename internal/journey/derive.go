package journey

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
)

// candidateTransitions returns cfg.Model.Lifecycle[class]'s transitions
// whose From equals the joined effective status (DC-15: joined via
// specstate.Result.ArtifactStatus() — never a literal-to-literal mapping
// table of this package's own, and never a comparison against spec.Status
// directly), sorted by Verb for determinism. Unproven state yields no
// candidates at all (DC-3: no from-state can be established); a class the
// model declares no lifecycle for likewise yields none — never a guess.
func candidateTransitions(mdl *model.Model, class string, result specstate.Result) []model.Transition {
	if result.State == specstate.Unproven {
		return nil
	}
	lifecycle, ok := mdl.Lifecycle[class]
	if !ok {
		return nil
	}
	from := string(result.ArtifactStatus())
	var out []model.Transition
	for _, tr := range lifecycle.Transitions {
		if tr.From == from {
			out = append(out, tr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Verb < out[j].Verb })
	return out
}

// obligationReason maps an obligation kind to its fixed ReasonCode and
// blocker-ID builder (the closed kind -> reason mapping the work order
// names: author-vouch/countersign/fold-green each get their own code; any
// other kind is the first-class "unknown kind" failure to classify, never
// silently folded into one of the three known reasons).
func obligationReason(kind string) (reason ReasonCode, idFor func(verb string) string) {
	switch kind {
	case "author-vouch":
		return ReasonObligationAuthorVouchUnproven, func(verb string) string { return "obligation-author-vouch-unproven/" + verb }
	case "countersign":
		return ReasonObligationCountersignUnproven, func(verb string) string { return "obligation-countersign-unproven/" + verb }
	case "fold-green":
		return ReasonObligationFoldGreenUnproven, func(verb string) string { return "obligation-fold-green-unproven/" + verb }
	default:
		return ReasonObligationUnknownKind, func(verb string) string { return "obligation-unknown-kind/" + verb }
	}
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
func deriveBlockers(defaultBranchKnown bool, result specstate.Result, candidates []model.Transition, owner Owner) []Blocker {
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
			Witnesses: []string{
				"the default branch could not be resolved: CI_DEFAULT_BRANCH is unset, no origin/HEAD symbolic ref is configured, and no lone conventional remote-tracking ref (origin/main or origin/master) was found",
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
			reason, idFor := obligationReason(ob.Kind)
			class, err := reason.Class()
			if err != nil {
				// obligationReason only ever returns a registered code; a
				// non-nil error here would mean this package's own reason
				// table and reasonClasses table disagree — fail loudly
				// rather than silently defaulting a class.
				panic(fmt.Sprintf("journey: obligationReason returned unregistered reason %q: %v", reason, err))
			}
			add(Blocker{
				ID:     idFor(tr.Verb),
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
		add(Blocker{
			ID:     "principal-resolution-unproven/" + tr.Verb,
			Reason: ReasonPrincipalResolutionUnproven,
			Class:  ClassGovernance,
			Witnesses: []string{
				"no governance profile is adopted at the evaluated revision; authenticated principal resolution is unproven (governance-principal kernel present, no adopted profile artifact)",
			},
			Owner:             owner,
			ClearingCondition: "a governance profile is adopted and the required principals resolve as authenticated",
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
// RequiredRole per attestation-scheme author-vouch/countersign obligation
// on a candidate transition (behavioral obligations are never principal
// requirements — DC-19's kernel attribution stays confined to actual
// attestation gates). ProfileAdopted is always false in this delivery unit
// (no governance profile storage exists in-tree yet — Context Integrity's
// constitution store is a later unit); Resolution is always "unproven" —
// v1 wires no resolver (DC-18's three-valued honesty: never a silent
// authenticated claim).
func derivePrincipals(candidates []model.Transition) PrincipalFacts {
	var required []RequiredRole
	for _, tr := range candidates {
		for _, ob := range tr.Obligations {
			if ob.Scheme != "attestation" {
				continue
			}
			switch ob.Kind {
			case "author-vouch":
				required = append(required, RequiredRole{
					Transition: tr.Verb,
					Obligation: "attestation/author-vouch",
					Count:      1,
					Resolution: "unproven",
				})
			case "countersign":
				count := ob.Count
				if count < 1 {
					count = 1
				}
				required = append(required, RequiredRole{
					Transition: tr.Verb,
					Obligation: "attestation/countersign",
					Count:      count,
					Resolution: "unproven",
				})
			}
		}
	}
	sort.Slice(required, func(i, j int) bool { return requiredRoleKey(required[i]) < requiredRoleKey(required[j]) })
	if required == nil {
		required = []RequiredRole{}
	}
	return PrincipalFacts{
		ProfileAdopted: false,
		Required:       required,
		Disclosures: []string{
			"no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown",
		},
	}
}

// deriveActions derives the record's safe actions (DC-3): a candidate
// transition becomes safe only when the lifecycle state is proven, the
// transition is registered for the class (both true by construction here
// — candidateTransitions only ever returns registered transitions whose
// From already matches the proven effective status), AND it carries zero
// obligations (any obligation is unprovable by this projection version, so
// the action is excluded and NeededFacts names it instead). Unproven state
// yields no candidates and one fixed NeededFacts entry.
func deriveActions(class string, result specstate.Result, candidates []model.Transition, targetRef string) Actions {
	var safe []Action
	var needed []string

	if result.State == specstate.Unproven {
		needed = append(needed, "lifecycle state is unproven; no transition's from-state can be established")
	} else {
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
