package constitutionapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// ImpactTarget is one caller-supplied supplemental preview target. Registered
// coverage is derived only from the exact-tree constitution consumer
// inventories; a target here can add presentation detail but cannot remove a
// registered consumer or improve submission readiness.
type ImpactTarget struct {
	Spec    string                    `json:"spec"`
	Phase   contextcompile.Phase      `json:"phase"`
	Adapter contextcompile.AdapterRef `json:"adapter"`
	Scope   policyartifact.Scope      `json:"scope"`
}

// ImpactReviewRequest is ImpactReview's strict request. Targets are
// supplemental only; the accepted/proposed inventory union is authoritative.
type ImpactReviewRequest struct {
	Schema  string         `json:"schema"`
	Targets []ImpactTarget `json:"targets"`
}

func (r ImpactReviewRequest) validate() error { return validateTargets(r.Targets) }

// LayerChange is one added, removed, or changed constitution source layer.
type LayerChange struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	Change         string `json:"change"`
	AcceptedDigest string `json:"accepted_digest,omitempty"`
	ProposedDigest string `json:"proposed_digest,omitempty"`
}

// TargetConflict is one supplemental target's conflict posture.
type TargetConflict struct {
	Target  ImpactTarget           `json:"target"`
	Report  *policyconflict.Report `json:"report,omitempty"`
	Refusal string                 `json:"refusal,omitempty"`
}

// ImpactReviewResult carries the exact accepted/proposed authority snapshots,
// canonical registered-consumer coverage, and supplemental caller previews.
// AffectedConsumers is the sorted registered consumer identity set; it no
// longer restates caller-selected specs.
type ImpactReviewResult struct {
	Schema            string                      `json:"schema"`
	Identity          Identity                    `json:"identity"`
	Accepted          Snapshot                    `json:"accepted"`
	Proposed          Snapshot                    `json:"proposed"`
	Layers            []LayerChange               `json:"layers"`
	Coverage          constitutionimpact.Coverage `json:"coverage"`
	Conflicts         []TargetConflict            `json:"conflicts"`
	AffectedConsumers []string                    `json:"affected_consumers"`
}

// MarshalJSON embeds Coverage through constitutionimpact's frozen canonical
// codec. Coverage is a domain struct whose wire grammar intentionally lives in
// constitutionimpact's private codec document; allowing encoding/json to walk
// the domain value directly would leak Go field names and bypass its nested
// request/manifest/report validation.
func (r ImpactReviewResult) MarshalJSON() ([]byte, error) {
	coverage, err := constitutionimpact.EncodeCoverage(r.Coverage)
	if err != nil {
		return nil, fmt.Errorf("constitutionapp: encoding impact coverage: %w", err)
	}
	type impactReviewWire struct {
		Schema            string           `json:"schema"`
		Identity          Identity         `json:"identity"`
		Accepted          Snapshot         `json:"accepted"`
		Proposed          Snapshot         `json:"proposed"`
		Layers            []LayerChange    `json:"layers"`
		Coverage          json.RawMessage  `json:"coverage"`
		Conflicts         []TargetConflict `json:"conflicts"`
		AffectedConsumers []string         `json:"affected_consumers"`
	}
	return json.Marshal(impactReviewWire{
		Schema: r.Schema, Identity: r.Identity, Accepted: r.Accepted, Proposed: r.Proposed,
		Layers: r.Layers, Coverage: json.RawMessage(bytes.TrimSpace(coverage)),
		Conflicts: r.Conflicts, AffectedConsumers: r.AffectedConsumers,
	})
}

// ImpactReview evaluates the complete exact-tree registered union through the
// existing accepted-context conflict path. Caller targets are supplemental.
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
	review, typed := s.impactReviewAt(ctx, root, identity, req)
	if typed != nil {
		return nil, typed
	}
	observed, typed := s.resolveIdentity(ctx, root)
	if typed != nil {
		return nil, typed
	}
	if identity != observed {
		return nil, operational("identity-shifted", fmt.Sprintf(
			"the repository moved during impact review: it was observed at %s before, and at %s after",
			describeIdentity(identity), describeIdentity(observed)), nil)
	}
	return review, nil
}

type impactState struct {
	acceptedTree     constitutionimpact.ExactTree
	proposedTree     constitutionimpact.ExactTree
	acceptedSnapshot Snapshot
	proposedSnapshot Snapshot
}

// impactReviewAt consumes an already-resolved identity. Both snapshots and
// both inventories/catalogs are loaded from the same two exact Git-tree
// sources, never from current-worktree bytes carrying committed labels.
func (s Service) impactReviewAt(ctx context.Context, root string, identity Identity, req ImpactReviewRequest) (*ImpactReviewResult, *Error) {
	state, typed := s.loadImpactState(ctx, root, identity)
	if typed != nil {
		return nil, typed
	}
	layers := []LayerChange{}
	if state.acceptedTree.FS != nil && state.proposedTree.FS != nil {
		layers = diffImpactLayers(state.acceptedSnapshot, state.proposedSnapshot)
	}
	plan, err := constitutionimpact.BuildPlan(ctx, state.acceptedTree, state.proposedTree, impactLayerChanges(layers))
	if err != nil {
		return nil, operational("impact-evidence-invalid", "building constitution impact coverage", err)
	}

	exactTreesAvailable := state.acceptedTree.FS != nil && state.proposedTree.FS != nil
	evaluations, conflicts, supplemental, typed := s.evaluateAtProposedCommit(ctx, root, identity, exactTreesAvailable, plan.Consumers(), req.Targets)
	if typed != nil {
		return nil, typed
	}
	coverage := plan.Complete(evaluations, supplemental)
	affected := make([]string, len(coverage.Evaluations))
	for i := range coverage.Evaluations {
		affected[i] = coverage.Evaluations[i].ConsumerIdentity
	}

	return &ImpactReviewResult{
		Schema:            ImpactReviewResultSchema,
		Identity:          identity,
		Accepted:          state.acceptedSnapshot,
		Proposed:          state.proposedSnapshot,
		Layers:            layers,
		Coverage:          coverage,
		Conflicts:         conflicts,
		AffectedConsumers: affected,
	}, nil
}

func (s Service) loadImpactState(ctx context.Context, root string, identity Identity) (impactState, *Error) {
	if !identity.AcceptedKnown {
		return impactState{}, operational("accepted-identity-unavailable", "the accepted default branch is unresolved", nil)
	}
	proposedTree, err := s.exactTreeAt(ctx, root, identity.Head)
	if err != nil {
		return impactState{}, operational("io-failure", "reading proposed constitution tree", err)
	}
	acceptedTree := proposedTree
	if identity.AcceptedHead != identity.Head {
		acceptedTree, err = s.exactTreeAt(ctx, root, identity.AcceptedHead)
		if err != nil {
			return impactState{}, operational("io-failure", "reading accepted constitution tree", err)
		}
	}
	proposed, typed := loadExactSnapshot(s.Authority, proposedTree, identity.Head, "corrupted-policy")
	if typed != nil {
		return impactState{}, typed
	}
	accepted, typed := loadExactSnapshot(s.Authority, acceptedTree, identity.AcceptedHead, "corrupted-policy")
	if typed != nil {
		return impactState{}, typed
	}
	return impactState{
		acceptedTree: acceptedTree, proposedTree: proposedTree,
		acceptedSnapshot: accepted, proposedSnapshot: proposed,
	}, nil
}

func loadExactSnapshot(store AuthorityStore, tree constitutionimpact.ExactTree, ref, corruptCode string) (Snapshot, *Error) {
	if tree.FS == nil {
		return Snapshot{Ref: ref, Reason: "exact Git tree bytes are unavailable", unavailable: true}, nil
	}
	return loadSnapshot(store, tree.FS, ref, corruptCode)
}

type cachedEvaluation struct {
	evidence ConflictEvidence
	err      error
}

func (s Service) evaluateAtProposedCommit(
	ctx context.Context,
	root string,
	identity Identity,
	exactTreesAvailable bool,
	consumers []constitutionimpact.Consumer,
	targets []ImpactTarget,
) ([]constitutionimpact.Evaluation, []TargetConflict, []constitutionimpact.SupplementalTarget, *Error) {
	if !exactTreesAvailable {
		conflicts, supplemental := unavailableSupplementalTargets(targets)
		return []constitutionimpact.Evaluation{}, conflicts, supplemental, nil
	}
	evaluationRoot := root
	var checkout *evaluationCheckout
	if !identity.Dirty && (len(consumers) != 0 || len(targets) != 0) {
		var err error
		checkout, err = s.materializeEvaluationCheckout(ctx, root, identity.Head)
		if err != nil {
			return nil, nil, nil, operational("io-failure", "materializing proposed constitution evaluation checkout", err)
		}
		evaluationRoot = checkout.root
	}

	evaluations, typed := s.evaluateRegisteredConsumers(ctx, evaluationRoot, identity, consumers)
	var conflicts []TargetConflict
	var supplemental []constitutionimpact.SupplementalTarget
	if typed == nil {
		conflicts, supplemental, typed = s.evaluateSupplementalTargets(ctx, evaluationRoot, identity, targets)
	}
	if checkout != nil {
		if err := checkout.Close(ctx); err != nil {
			cause := error(err)
			if typed != nil {
				cause = errors.Join(typed, err)
			}
			return nil, nil, nil, operational("io-failure", "removing proposed constitution evaluation checkout", cause)
		}
	}
	if typed != nil {
		return nil, nil, nil, typed
	}
	if evaluations == nil {
		evaluations = []constitutionimpact.Evaluation{}
	}
	if conflicts == nil {
		conflicts = []TargetConflict{}
	}
	if supplemental == nil {
		supplemental = []constitutionimpact.SupplementalTarget{}
	}
	return evaluations, conflicts, supplemental, nil
}

func (s Service) evaluateRegisteredConsumers(ctx context.Context, root string, identity Identity, consumers []constitutionimpact.Consumer) ([]constitutionimpact.Evaluation, *Error) {
	out := make([]constitutionimpact.Evaluation, len(consumers))
	cache := make(map[string]cachedEvaluation, len(consumers))
	for i, consumer := range consumers {
		consumerIdentity, _ := consumer.Identity()
		out[i] = constitutionimpact.Evaluation{ConsumerIdentity: consumerIdentity, Consumer: consumer}
		if identity.Dirty {
			out[i].Refusal = unresolvedEvaluation("checkout-dirty")
			continue
		}
		key, err := consumerEvidenceKey(consumer)
		if err != nil {
			out[i].Refusal = unresolvedEvaluation(err.Error())
			continue
		}
		cached, ok := cache[key]
		if !ok {
			if s.Conflict == nil {
				cached.err = errors.New("conflict evaluator is not configured")
			} else {
				cached.evidence, cached.err = s.Conflict.Evaluate(ctx, root, acceptedContextRequest(consumer.Request))
			}
			cache[key] = cached
		}
		if cached.err != nil {
			if isContextFailure(ctx, cached.err) {
				return nil, operational("evaluation-canceled", "evaluating registered constitution consumer "+consumerIdentity, cached.err)
			}
			out[i].Refusal = unresolvedEvaluation(cached.err.Error())
			continue
		}
		result := cached.evidence.Result
		out[i].AcceptedManifestBytes = append([]byte(nil), cached.evidence.AcceptedManifestBytes...)
		out[i].Result = &result
	}
	return out, nil
}

func consumerEvidenceKey(consumer constitutionimpact.Consumer) (string, error) {
	encoded, err := contextcompile.EncodeRequest(consumer.Request)
	if err != nil {
		return "", err
	}
	return string(encoded) + "\x00" + consumer.Environment, nil
}

func unresolvedEvaluation(witness string) *constitutionimpact.EvaluationRefusal {
	return &constitutionimpact.EvaluationRefusal{
		Code: constitutionimpact.ReasonEvaluationUnresolved, Witnesses: []string{witness},
	}
}

func acceptedContextRequest(request contextcompile.Request) policyconflict.Request {
	requestCopy := request
	return policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{Kind: policyconflict.TargetAcceptedContext, AcceptedContext: &requestCopy},
	}
}

func (s Service) evaluateSupplementalTargets(ctx context.Context, root string, identity Identity, targets []ImpactTarget) ([]TargetConflict, []constitutionimpact.SupplementalTarget, *Error) {
	conflicts := make([]TargetConflict, 0, len(targets))
	supplemental := make([]constitutionimpact.SupplementalTarget, 0, len(targets))
	for _, target := range targets {
		request := supplementalRequest(target)
		row := constitutionimpact.SupplementalTarget{Request: request}
		if identity.Dirty {
			row.Refusal = unresolvedEvaluation("checkout-dirty")
			conflicts = append(conflicts, TargetConflict{Target: target, Refusal: "unproven: checkout-dirty"})
			supplemental = append(supplemental, row)
			continue
		}
		if s.Conflict == nil {
			row.Refusal = unresolvedEvaluation("conflict evaluator is not configured")
			conflicts = append(conflicts, TargetConflict{Target: target, Refusal: "unproven: conflict evaluator is not configured"})
			supplemental = append(supplemental, row)
			continue
		}
		evidence, err := s.Conflict.Evaluate(ctx, root, acceptedContextRequest(request))
		if err != nil {
			if isContextFailure(ctx, err) {
				return nil, nil, operational("evaluation-canceled", "evaluating supplemental constitution target "+target.Spec, err)
			}
			refusal := supplementalRefusal(err)
			row.Refusal = unresolvedEvaluation(refusal)
			conflicts = append(conflicts, TargetConflict{Target: target, Refusal: refusal})
			supplemental = append(supplemental, row)
			continue
		}
		result := evidence.Result
		report := result.Report
		row.AcceptedManifestBytes = append([]byte(nil), evidence.AcceptedManifestBytes...)
		row.Result = &result
		conflicts = append(conflicts, TargetConflict{Target: target, Report: &report})
		supplemental = append(supplemental, row)
	}
	return conflicts, supplemental, nil
}

func unavailableSupplementalTargets(targets []ImpactTarget) ([]TargetConflict, []constitutionimpact.SupplementalTarget) {
	conflicts := make([]TargetConflict, 0, len(targets))
	supplemental := make([]constitutionimpact.SupplementalTarget, 0, len(targets))
	for _, target := range targets {
		refusal := "unproven: exact-tree-unavailable"
		conflicts = append(conflicts, TargetConflict{Target: target, Refusal: refusal})
		supplemental = append(supplemental, constitutionimpact.SupplementalTarget{
			Request: supplementalRequest(target), Refusal: unresolvedEvaluation("exact-tree-unavailable"),
		})
	}
	return conflicts, supplemental
}

func supplementalRequest(target ImpactTarget) contextcompile.Request {
	return contextcompile.Request{
		Schema: contextcompile.RequestSchema, Adapter: target.Adapter,
		Grants: execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		Phase:  target.Phase, Scope: target.Scope, Spec: target.Spec,
	}
}

func isContextFailure(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func supplementalRefusal(err error) string {
	if policyconflict.IsNotAdopted(err) {
		return "not-adopted: " + err.Error()
	}
	var scopeRefusal *contextcompile.DeclaredScopeRefusal
	var specRefusal *contextcompile.AcceptedSpecRefusal
	var adapterRefusal *contextcompile.AdapterMismatchRefusal
	var phaseScopeRefusal *contextcompile.PhaseScopeRefusal
	if errors.As(err, &scopeRefusal) || errors.As(err, &specRefusal) || errors.As(err, &adapterRefusal) || errors.As(err, &phaseScopeRefusal) {
		return "unknown-scope: " + err.Error()
	}
	return "unproven: " + err.Error()
}

func diffImpactLayers(accepted, proposed Snapshot) []LayerChange {
	changes := diffLayers(accepted.Layers, proposed.Layers)
	if accepted.ConstitutionDigest != proposed.ConstitutionDigest {
		change := LayerChange{Kind: policyartifact.KindConstitution, ID: policyartifact.KindConstitution + "/" + policyartifact.ConstitutionName}
		switch {
		case accepted.ConstitutionDigest == "":
			change.Change = "added"
			change.ProposedDigest = proposed.ConstitutionDigest
		case proposed.ConstitutionDigest == "":
			change.Change = "removed"
			change.AcceptedDigest = accepted.ConstitutionDigest
		default:
			change.Change = "changed"
			change.AcceptedDigest = accepted.ConstitutionDigest
			change.ProposedDigest = proposed.ConstitutionDigest
		}
		changes = append(changes, change)
		sort.Slice(changes, func(i, j int) bool {
			if changes[i].Kind != changes[j].Kind {
				return changes[i].Kind < changes[j].Kind
			}
			return changes[i].ID < changes[j].ID
		})
	}
	return changes
}

func impactLayerChanges(in []LayerChange) []constitutionimpact.LayerChange {
	out := make([]constitutionimpact.LayerChange, len(in))
	for i, row := range in {
		out[i] = constitutionimpact.LayerChange{
			Kind: row.Kind, ID: row.ID, Change: row.Change,
			AcceptedDigest: row.AcceptedDigest, ProposedDigest: row.ProposedDigest,
		}
	}
	return out
}

// diffLayers compares accepted and proposed source-layer sets by (kind, id).
func diffLayers(accepted, proposed []SourceLayer) []LayerChange {
	type key struct{ kind, id string }
	acceptedByKey := make(map[key]string, len(accepted))
	for _, layer := range accepted {
		acceptedByKey[key{layer.Kind, layer.ID}] = layer.Digest
	}
	proposedByKey := make(map[key]string, len(proposed))
	for _, layer := range proposed {
		proposedByKey[key{layer.Kind, layer.ID}] = layer.Digest
	}
	changes := []LayerChange{}
	for identity, acceptedDigest := range acceptedByKey {
		proposedDigest, present := proposedByKey[identity]
		switch {
		case !present:
			changes = append(changes, LayerChange{Kind: identity.kind, ID: identity.id, Change: "removed", AcceptedDigest: acceptedDigest})
		case proposedDigest != acceptedDigest:
			changes = append(changes, LayerChange{Kind: identity.kind, ID: identity.id, Change: "changed", AcceptedDigest: acceptedDigest, ProposedDigest: proposedDigest})
		}
	}
	for identity, proposedDigest := range proposedByKey {
		if _, present := acceptedByKey[identity]; !present {
			changes = append(changes, LayerChange{Kind: identity.kind, ID: identity.id, Change: "added", ProposedDigest: proposedDigest})
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
