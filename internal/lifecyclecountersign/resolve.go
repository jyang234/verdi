// Package lifecyclecountersign owns the one read-only bridge from lifecycle
// targets and forge observations to the canonical countersign reducer.
package lifecyclecountersign

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/forge"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/store"
)

// ErrProfileUnavailable identifies an absent or incomplete selected sealed
// governance profile. It is a missing operand, not a malformed policy store.
var ErrProfileUnavailable = errors.New("lifecycle countersign: selected governance profile unavailable")

// Request names the repository and lifecycle target whose current forge
// approval facts must be resolved.
type Request struct {
	Root              string
	Manifest          *store.Manifest
	Model             *model.Model
	TargetClass       string
	DefaultBranch     string
	SourceBranch      string
	LocalCandidateSHA string
	// AcceptedBranch, AcceptedCommit, and AcceptedProfileSource pin the ONE
	// accepted default-branch Git tree this invocation authenticates
	// against. Governance authority is acceptance truth, never mutable
	// checkout state (I-121): AcceptedProfileSource is a read-only view of
	// the tree at AcceptedCommit, and AcceptedBranch is the default-branch
	// name that commit was resolved from, cross-matched here against
	// DefaultBranch — the same branch identity forge targeting uses — so a
	// profile can never arrive from some other branch than the one the
	// candidate is being countersigned into. Root remains the mutable
	// checkout's own identity; it is never a governance source.
	AcceptedBranch        string
	AcceptedCommit        string
	AcceptedProfileSource fs.FS
}

// Result preserves either a full canonical record or a stable unproven
// witness set when a required external operand does not exist.
type Result struct {
	Verdict   countersign.Verdict
	Record    *countersign.Record
	Witnesses []string
}

// Resolver owns MR discovery, approval observation, selected-profile loading,
// provider-fact authentication, candidate-author resolution, and countersign
// reduction. Clock is an explicit deterministic test seam; its nil value
// selects the live implementation. The selected profile has no seam at all:
// it is decoded from Request.AcceptedProfileSource through the ONE
// policyauthority decoder, so no caller can substitute profile bytes the
// accepted tree does not carry.
type Resolver struct {
	Forge forge.Forge
	Clock func() time.Time
}

// Resolve reads current authority facts and returns a three-valued result.
func (r Resolver) Resolve(ctx context.Context, request Request) (Result, error) {
	if request.Manifest == nil || request.Manifest.Countersign == nil {
		return unproven("config", "verdi.yaml countersign block is absent"), nil
	}
	config := request.Manifest.Countersign
	if err := config.Validate(); err != nil {
		return Result{}, fmt.Errorf("lifecycle countersign: config: %w", err)
	}
	role, requiredCount, err := lifecycleObligation(request.Model, request.TargetClass)
	if err != nil {
		return Result{}, err
	}
	if r.Forge == nil {
		return unproven("forge", "configured forge is unavailable"), nil
	}
	if request.DefaultBranch == "" {
		// vocab:identity — forge provider resource name, not a Verdi lifecycle class.
		return unproven("default-branch", "default branch is unavailable for merge-request discovery"), nil
	}
	if request.SourceBranch == "" {
		// vocab:identity — forge provider resource name, not a Verdi lifecycle class.
		return unproven("source-branch", "candidate source branch is unavailable for merge-request discovery"), nil
	}

	changeID, err := forge.FindOpenMR(ctx, r.Forge, request.DefaultBranch, request.SourceBranch)
	if err != nil {
		if errors.Is(err, forge.ErrUnavailable) {
			// vocab:identity — forge provider resource name, not a Verdi lifecycle class.
			return unproven("forge", fmt.Sprintf("merge-request discovery is unavailable: %v", err)), nil
		}
		// vocab:identity — forge provider resource name, not a Verdi lifecycle class.
		return Result{}, fmt.Errorf("lifecycle countersign: discover merge request: %w", err)
	}
	if changeID == "" {
		return unproven("merge-request", fmt.Sprintf("no open change targets %q from %q", request.DefaultBranch, request.SourceBranch)), nil
	}

	snapshot, err := r.Forge.ListApprovals(ctx, changeID)
	if err != nil {
		if errors.Is(err, forge.ErrUnavailable) {
			return unproven("forge", fmt.Sprintf("approval observation is unavailable for change %q: %v", changeID, err)), nil
		}
		return Result{}, fmt.Errorf("lifecycle countersign: query approvals for change %q: %w", changeID, err)
	}
	if err := snapshot.Validate(); err != nil {
		return Result{}, fmt.Errorf("lifecycle countersign: provider snapshot: %w", err)
	}
	if snapshot.ChangeID != changeID {
		return Result{}, fmt.Errorf("lifecycle countersign: provider snapshot change_id %q does not match discovered change %q", snapshot.ChangeID, changeID)
	}

	if request.AcceptedProfileSource == nil || request.AcceptedCommit == "" || request.AcceptedBranch == "" {
		return unproven("accepted-tree", "accepted default-branch governance tree is unresolved"), nil
	}
	if request.AcceptedBranch != request.DefaultBranch {
		// vocab:identity — forge provider resource name, not a Verdi lifecycle class.
		return unproven("accepted-tree", fmt.Sprintf("pinned governance branch %q is not the forge target branch %q", request.AcceptedBranch, request.DefaultBranch)), nil
	}
	profile, err := loadSelectedProfile(request.AcceptedProfileSource)
	if err != nil {
		if errors.Is(err, ErrProfileUnavailable) {
			return unproven("profile", err.Error()), nil
		}
		return Result{}, fmt.Errorf("lifecycle countersign: load selected governance profile from accepted tree %s: %w", request.AcceptedCommit, err)
	}

	policy, err := countersign.NewFreshnessPolicy(config.FreshnessPolicyID, config.MaximumObservationAgeSeconds, config.MaximumApprovalAgeSeconds)
	if err != nil {
		return Result{}, fmt.Errorf("lifecycle countersign: freshness policy: %w", err)
	}
	if !profileHasTrustSource(profile, config.TrustSource) {
		return unproven("principal-authentication", fmt.Sprintf("configured trust source %q is absent from the selected governance profile", config.TrustSource)), nil
	}
	// SI-227's second finding: a local-operator identity is a bare
	// self-assertion (2026-09-05 local-operator disposition design §2.1)
	// and can never itself prove a close countersign, which needs a
	// forge-witnessed approval fact. Checked before any principal is
	// resolved, using only the kernel's own exported trust-source-kind
	// query — never internal/governanceprincipal edits.
	if localOnly, err := gp.HasTrustSourceKind(profile, config.TrustSource, gp.TrustSourceLocalOperator); err != nil {
		return Result{}, fmt.Errorf("lifecycle countersign: configured trust source kind: %w", err)
	} else if localOnly {
		return unproven("principal-authentication", fmt.Sprintf("configured trust source %q is local-operator only: a close countersign needs a forge-witnessed approval, never a bare self-assertion (SI-227)", config.TrustSource)), nil
	}
	facts := providerFacts{snapshot: snapshot}
	principalResolver := gp.NewResolver(facts)
	author, err := principalResolver.Resolve(ctx, profile, gp.PrincipalClaim{TrustSource: config.TrustSource, Subject: snapshot.CandidateAuthor.Subject})
	if err != nil {
		return Result{}, fmt.Errorf("lifecycle countersign: resolve candidate author: %w", err)
	}
	separationRule, separationWitnesses := kernelSeparationRule(profile, role, author)
	clock := r.Clock
	if clock == nil {
		clock = time.Now
	}
	// SeparationNone means the profile's own rules permit the author's
	// principal to also fill the approver role (solo collapse); the
	// candidate author is then meaningless to countersign.Resolve's
	// per-approval separation check and validateRequest requires it absent.
	var candidateAuthor *gp.PrincipalResolution
	if separationRule == countersign.SeparationDifferentFromAuthor {
		candidateAuthor = &author
	}
	record, err := countersign.Resolve(ctx, countersign.Request{
		Snapshot: snapshot, LocalCandidateSHA: request.LocalCandidateSHA,
		Profile: profile, TrustSourceID: config.TrustSource,
		Obligation: countersign.Obligation{
			Transition: "close", Scheme: countersign.SchemeAttestation,
			Kind: countersign.KindCountersign, Role: role, RequiredCount: requiredCount,
			SeparationRule: separationRule,
		},
		FreshnessPolicy:       policy,
		EvaluatedAt:           clock().UTC().Format(time.RFC3339Nano),
		CandidateAuthor:       candidateAuthor,
		Resolver:              principalResolver,
		SeparationDisclosures: separationWitnesses,
	})
	if err != nil {
		return Result{}, fmt.Errorf("lifecycle countersign: reduce approvals: %w", err)
	}
	copy := record
	return Result{Verdict: record.Verdict, Record: &copy, Witnesses: append([]string{}, record.Witnesses...)}, nil
}

func profileHasTrustSource(profile gp.Profile, trustSource string) bool {
	for _, source := range profile.IdentityTrustSources {
		if source.ID == trustSource {
			return true
		}
	}
	return false
}

func lifecycleObligation(mdl *model.Model, class string) (string, int, error) {
	role := ""
	switch class {
	case "story":
		role = "story-review"
	case "feature":
		role = "feature-uat"
	default:
		return "", 0, fmt.Errorf("lifecycle countersign: unsupported target class %q", class)
	}
	if mdl == nil {
		return "", 0, fmt.Errorf("lifecycle countersign: operating model is required")
	}
	lifecycle, ok := mdl.Lifecycle[class]
	if !ok {
		return "", 0, fmt.Errorf("lifecycle countersign: operating model has no lifecycle for class %q", class)
	}
	counts := []int{}
	for _, transition := range lifecycle.Transitions {
		if transition.Verb != "close" {
			continue
		}
		for _, obligation := range transition.Obligations {
			if obligation.Scheme == countersign.SchemeAttestation && obligation.Kind == countersign.KindCountersign {
				counts = append(counts, obligation.Count)
			}
		}
	}
	if len(counts) != 1 {
		// vocab:identity — operating-model transition verb, not display prose.
		return "", 0, fmt.Errorf("lifecycle countersign: class %q close transition must carry exactly one attestation/countersign obligation, got %d", class, len(counts))
	}
	if counts[0] <= 0 {
		// vocab:identity — operating-model transition verb, not display prose.
		return "", 0, fmt.Errorf("lifecycle countersign: class %q close countersign count must be positive", class)
	}
	return role, counts[0], nil
}

// loadSelectedProfile decodes the accepted tree's selected sealed
// governance profile through policyauthority's single decoder. An
// unadopted or incompletely adopted accepted tree is a MISSING operand
// (unproven); every other structural failure stays operational.
func loadSelectedProfile(source fs.FS) (gp.Profile, error) {
	policyStore, err := policyauthority.LoadFromSource(source)
	if err != nil {
		if errors.Is(err, policyauthority.ErrNotAdopted) || errors.Is(err, policyauthority.ErrIncompleteAdoption) {
			return gp.Profile{}, fmt.Errorf("%w: %v", ErrProfileUnavailable, err)
		}
		return gp.Profile{}, err
	}
	profile, err := policyStore.SelectedProfile()
	if err != nil {
		return gp.Profile{}, err
	}
	return profile, nil
}

type providerFacts struct {
	snapshot forge.ApprovalSnapshot
}

func (f providerFacts) ReadTrustFact(_ context.Context, source gp.TrustSource, claim gp.PrincipalClaim) (gp.TrustFact, error) {
	identity := struct {
		SourceID         string `json:"source_id"`
		ProviderSnapshot string `json:"provider_snapshot_id"`
		Subject          string `json:"subject"`
	}{source.ID, f.snapshot.ProviderSnapshotID, claim.Subject}
	digest, err := canonjson.Digest(identity)
	if err != nil {
		return gp.TrustFact{}, fmt.Errorf("lifecycle countersign: provider fact digest: %w", err)
	}
	present := f.snapshot.CandidateAuthor.Subject == claim.Subject
	for _, approval := range f.snapshot.Approvals {
		present = present || approval.Actor.Subject == claim.Subject
	}
	fact := gp.TrustFact{
		SourceID: source.ID, SourceKind: source.Kind,
		EvidenceDigest: digest, Available: true, Valid: present,
	}
	if present {
		fact.Subjects = []string{claim.Subject}
	} else {
		fact.Subjects = []string{}
		fact.Reason = "provider snapshot does not contain the claimed stable subject"
	}
	return fact, nil
}

func unproven(operand, detail string) Result {
	witnesses := []string{fmt.Sprintf("lifecycle-countersign:%s:unproven:%s", operand, detail)}
	sort.Strings(witnesses)
	return Result{Verdict: countersign.VerdictUnproven, Witnesses: witnesses}
}

// kernelCloseTransition is the fixed transition name every kernel
// separation probe evaluates — the same literal Resolve binds into
// countersign.Obligation.Transition above.
const kernelCloseTransition = "close"

// kernelSeparationRule asks the governance kernel's authorization
// interpreter (gp.Authorize), using the selected profile's own rules for
// the close transition and the resolved candidate author's identity,
// whether the obligation's approverRole must be filled by a principal
// different from the author (SI-227, as narrowed by PR #345 finding F1).
// It never decides from profile.Class alone: gp.Authorize is always
// consulted, and Class only gates which POSITIVE answer (collapse) can be
// honored.
//
// Outcomes:
//   - required (team, high-assurance, or any profile whose own rules say
//     so): countersign.SeparationDifferentFromAuthor, today's behavior,
//     AC-3's self-approval refusal intact;
//   - permitted with collapse under a solo profile whose kernel decision
//     cleanly authorizes the author also filling approverRole, carrying a
//     solo role-collapse disclosure for the author's own principal:
//     countersign.SeparationNone, with that disclosure returned as a
//     witness for the caller to carry into the countersign record;
//   - unavailable or unproven kernel answer (an unauthenticated author, a
//     kernel error, or any decision that is not a clean authorized solo
//     collapse): countersign.SeparationDifferentFromAuthor (fail closed),
//     with a witness disclosing why.
func kernelSeparationRule(profile gp.Profile, approverRole string, author gp.PrincipalResolution) (countersign.SeparationRule, []string) {
	if author.State != gp.ResolutionAuthenticated {
		return countersign.SeparationDifferentFromAuthor, []string{
			`kernel-separation:unavailable:reason="candidate author principal is not authenticated"`,
		}
	}
	approvals := []gp.ApprovalRecord{{Role: approverRole, PrincipalID: author.PrincipalID}}
	for _, otherRole := range otherProfileRoles(profile, approverRole) {
		if holds, err := gp.HoldsRole(profile, author.Claim, otherRole); err == nil && holds {
			approvals = append(approvals, gp.ApprovalRecord{Role: otherRole, PrincipalID: author.PrincipalID})
		}
	}
	decision, err := gp.Authorize(profile, gp.AuthorizationRequest{
		Transition:  kernelCloseTransition,
		Posture:     gp.PostureAuthoritative,
		Resolutions: []gp.PrincipalResolution{author},
		Approvals:   approvals,
	})
	if err != nil {
		return countersign.SeparationDifferentFromAuthor, []string{
			fmt.Sprintf("kernel-separation:unavailable:reason=%q", err.Error()),
		}
	}
	if profile.Class == gp.ClassSolo && decision.State == gp.AuthorizationAuthorized {
		for _, d := range decision.Disclosures {
			if d.Code == gp.ReasonSoloRoleCollapse && d.PrincipalID == author.PrincipalID {
				return countersign.SeparationNone, []string{
					fmt.Sprintf("kernel-separation:solo-role-collapse:principal_id=%q:roles=%q", d.PrincipalID, d.Roles),
				}
			}
		}
	}
	return countersign.SeparationDifferentFromAuthor, kernelRequiredWitnesses(profile, decision)
}

// otherProfileRoles returns every distinct role name profile's own role
// mappings declare besides approverRole, sorted for determinism — the
// candidate "other" roles kernelSeparationRule probes the author's
// membership against. It reads only the already-decoded, sealed profile's
// public field; the actual membership question is answered by the
// kernel's own exported gp.HoldsRole, never reimplemented here.
func otherProfileRoles(profile gp.Profile, approverRole string) []string {
	seen := map[string]bool{approverRole: true}
	var roles []string
	for _, m := range profile.RoleMappings {
		if seen[m.Role] {
			continue
		}
		seen[m.Role] = true
		roles = append(roles, m.Role)
	}
	sort.Strings(roles)
	return roles
}

// kernelRequiredWitnesses discloses why kernelSeparationRule kept
// SeparationDifferentFromAuthor when the decision was not a clean
// authorized solo collapse: every applicable kernel finding, or — when the
// decision was itself authorized but collapse was not honored (not solo,
// or no matching disclosure for this author) — a single witness naming
// the reason.
func kernelRequiredWitnesses(profile gp.Profile, decision gp.AuthorizationDecision) []string {
	if decision.State != gp.AuthorizationAuthorized {
		witnesses := make([]string, 0, len(decision.Findings))
		for _, f := range decision.Findings {
			witnesses = append(witnesses, fmt.Sprintf(
				"kernel-separation:%s:code=%q:role=%q:principal_id=%q:detail=%q",
				f.State, f.Code, f.Role, f.PrincipalID, f.Detail,
			))
		}
		if len(witnesses) == 0 {
			witnesses = append(witnesses, fmt.Sprintf("kernel-separation:%s:reason=\"kernel decision was not authorized\"", decision.State))
		}
		return witnesses
	}
	if profile.Class != gp.ClassSolo {
		return []string{fmt.Sprintf("kernel-separation:required:class=%q", profile.Class)}
	}
	return []string{`kernel-separation:unavailable:reason="no solo role-collapse disclosure for this profile"`}
}
