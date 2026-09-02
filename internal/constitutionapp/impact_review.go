package constitutionapp

import (
	"context"
	"errors"
	"os"
	"sort"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// ImpactTarget is one caller-declared governed operation to check for
// impact — the caller does the explicit selecting, exactly as
// internal/designapp.GetDesignContextRequest.ChildStories already
// establishes the precedent for this package's sibling: ImpactReview never
// scans the whole corpus for "every spec that might consume this policy" (a
// reverse-applicability query this package does not own and the Task 3 stop
// gate forbids inventing).
type ImpactTarget struct {
	Spec    string                    `json:"spec"`
	Phase   contextcompile.Phase      `json:"phase"`
	Adapter contextcompile.AdapterRef `json:"adapter"`
	Scope   policyartifact.Scope      `json:"scope"`
}

// ImpactReviewRequest is ImpactReview's strict request: every governed
// target the caller wants checked against the proposed constitution. The
// checkout root is resolved by the adapter, never accepted as request
// content (InspectRequest's own doc comment explains why).
type ImpactReviewRequest struct {
	Schema  string         `json:"schema"`
	Targets []ImpactTarget `json:"targets"`
}

func (r ImpactReviewRequest) validate() error { return validateTargets(r.Targets) }

// LayerChange is one added, removed, or changed source layer between the
// accepted and proposed constitutions, identified by kind+ID with both
// digests carried so a caller can see exactly what moved without Verdi
// collapsing "added" and "changed" into one ambiguous row.
type LayerChange struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	Change         string `json:"change"` // "added" | "removed" | "changed"
	AcceptedDigest string `json:"accepted_digest,omitempty"`
	ProposedDigest string `json:"proposed_digest,omitempty"`
}

// TargetConflict is one governed target's conflict posture under the
// proposed constitution, evaluated through the one existing conflict gate
// (internal/policyconflict). Refusal carries a disclosed, closed reason
// when the target itself could not be evaluated at all (unknown scope, an
// unresolvable spec, judge unavailability is instead carried inside Report
// itself as policyconflict.ReasonJudgeUnavailable — this field is only for
// a target the evaluator could not accept in the first place).
type TargetConflict struct {
	Target  ImpactTarget           `json:"target"`
	Report  *policyconflict.Report `json:"report,omitempty"`
	Refusal string                 `json:"refusal,omitempty"`
}

// ImpactReviewResult is ImpactReview's exact envelope: the diff between the
// accepted and proposed effective policies (never flattened — each row
// names its own kind/id/digests) plus every declared target's conflict
// posture. AffectedConsumers restates the exact caller-declared target
// specs that were evaluated, alongside their verdict — the caller-declared
// governed-operation set IS the affected-consumers set this operation
// reports; it is never widened by an undeclared corpus scan.
type ImpactReviewResult struct {
	Schema            string           `json:"schema"`
	Identity          Identity         `json:"identity"`
	Accepted          Snapshot         `json:"accepted"`
	Proposed          Snapshot         `json:"proposed"`
	Layers            []LayerChange    `json:"layers"`
	Conflicts         []TargetConflict `json:"conflicts"`
	AffectedConsumers []string         `json:"affected_consumers"`
}

// ImpactReview diffs the accepted and proposed effective policies and runs
// mechanical/semantic conflict evaluation over every caller-declared target
// through internal/policyconflict.Service.Evaluate, rooted at the current
// checkout (the proposed state) — never a second conflict evaluator or a
// second applicability derivation. It changes no governance state.
func (s Service) ImpactReview(ctx context.Context, root string, req ImpactReviewRequest) (*ImpactReviewResult, *Error) {
	if root == "" {
		return nil, inputInvalid("input-invalid", errRootRequired.Error())
	}
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, root)
	if typed != nil {
		return nil, typed
	}
	return s.impactReviewAt(ctx, root, identity, req)
}

// impactReviewAt is ImpactReview's body over an ALREADY-RESOLVED identity —
// validateAt's doc comment explains why a composing operation must thread one
// immutable identity snapshot rather than let each sub-operation resolve its
// own.
func (s Service) impactReviewAt(ctx context.Context, root string, identity Identity, req ImpactReviewRequest) (*ImpactReviewResult, *Error) {
	proposed, typed := loadSnapshot(s.Authority, os.DirFS(root), identity.Head, "corrupted-policy")
	if typed != nil {
		return nil, typed
	}
	var accepted Snapshot
	if identity.AcceptedKnown {
		source, err := s.acceptedSource(ctx, root, identity.AcceptedHead)
		if err != nil {
			return nil, operational("io-failure", "reading accepted constitution tree", err)
		}
		accepted, typed = loadSnapshot(s.Authority, source, identity.AcceptedHead, "corrupted-policy")
		if typed != nil {
			return nil, typed
		}
	} else {
		accepted = Snapshot{Adopted: false, Reason: "the accepted default branch is unresolved"}
	}

	layers := diffLayers(accepted.Layers, proposed.Layers)

	conflicts := make([]TargetConflict, 0, len(req.Targets))
	affected := make([]string, 0, len(req.Targets))
	for _, target := range req.Targets {
		request := policyconflict.Request{
			Schema: policyconflict.RequestSchema,
			Target: policyconflict.Target{
				Kind: policyconflict.TargetAcceptedContext,
				AcceptedContext: &contextcompile.Request{
					Schema:  contextcompile.RequestSchema,
					Adapter: target.Adapter,
					Phase:   target.Phase,
					Scope:   target.Scope,
					Spec:    target.Spec,
				},
			},
		}
		if s.Conflict == nil {
			return nil, operational("conflict-evaluator-unavailable", "conflict evaluator is not configured", nil)
		}
		result, err := s.Conflict.Evaluate(ctx, root, request)
		if err != nil {
			if policyconflict.IsNotAdopted(err) {
				conflicts = append(conflicts, TargetConflict{Target: target, Refusal: "not-adopted: " + err.Error()})
				continue
			}
			var scopeRefusal *contextcompile.DeclaredScopeRefusal
			var specRefusal *contextcompile.AcceptedSpecRefusal
			var adapterRefusal *contextcompile.AdapterMismatchRefusal
			var phaseScopeRefusal *contextcompile.PhaseScopeRefusal
			switch {
			case errors.As(err, &scopeRefusal), errors.As(err, &specRefusal), errors.As(err, &adapterRefusal), errors.As(err, &phaseScopeRefusal):
				// unknown-scope: the target does not resolve to a governed
				// context this evaluator can accept — recorded per-target,
				// never aborting the whole review, so one bad declared
				// target cannot hide every other target's real conflict
				// posture.
				conflicts = append(conflicts, TargetConflict{Target: target, Refusal: "unknown-scope: " + err.Error()})
				continue
			default:
				return nil, operational("io-failure", "evaluating conflicts for target "+target.Spec, err)
			}
		}
		report := result.Report
		conflicts = append(conflicts, TargetConflict{Target: target, Report: &report})
		affected = append(affected, target.Spec)
	}
	sort.Strings(affected)

	return &ImpactReviewResult{
		Schema:            ImpactReviewResultSchema,
		Identity:          identity,
		Accepted:          accepted,
		Proposed:          proposed,
		Layers:            layers,
		Conflicts:         conflicts,
		AffectedConsumers: affected,
	}, nil
}

// diffLayers compares accepted and proposed source-layer sets by
// (kind, id), reporting every addition, removal, and digest change. It
// never merges or reinterprets a layer's own content — a "changed" row
// carries both digests and nothing else, leaving the effective rule ledger
// itself (Snapshot.Effective) as the one place full content lives.
func diffLayers(accepted, proposed []SourceLayer) []LayerChange {
	type key struct{ kind, id string }
	acceptedByKey := make(map[key]string, len(accepted))
	for _, l := range accepted {
		acceptedByKey[key{l.Kind, l.ID}] = l.Digest
	}
	proposedByKey := make(map[key]string, len(proposed))
	for _, l := range proposed {
		proposedByKey[key{l.Kind, l.ID}] = l.Digest
	}

	changes := []LayerChange{}
	for k, acceptedDigest := range acceptedByKey {
		proposedDigest, stillPresent := proposedByKey[k]
		switch {
		case !stillPresent:
			changes = append(changes, LayerChange{Kind: k.kind, ID: k.id, Change: "removed", AcceptedDigest: acceptedDigest})
		case proposedDigest != acceptedDigest:
			changes = append(changes, LayerChange{Kind: k.kind, ID: k.id, Change: "changed", AcceptedDigest: acceptedDigest, ProposedDigest: proposedDigest})
		}
	}
	for k, proposedDigest := range proposedByKey {
		if _, present := acceptedByKey[k]; !present {
			changes = append(changes, LayerChange{Kind: k.kind, ID: k.id, Change: "added", ProposedDigest: proposedDigest})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].ID < changes[j].ID
	})
	return changes
}
