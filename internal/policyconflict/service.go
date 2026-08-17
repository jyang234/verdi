package policyconflict

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// ServiceDeps are the already-defined proof and authority seams the ordered
// evaluator composes. No dependency is stored outside this service value.
type ServiceDeps struct {
	Compiler   contextcompile.Compiler
	Refs       RefRelationResolver
	Primary    Judge
	Challenger Judge
	TreeHasher TreeHasher
	Dates      DateSource
	Actors     []governanceprincipal.PrincipalResolution
}

// TreeHasher supplies the current D4 corpus identity after the conflict
// snapshot has been resolved. It is required only for concrete local process
// adapters; injected non-process Judge ports remain filesystem-free.
type TreeHasher interface {
	TreeHash(context.Context, string) (string, error)
}

// Service derives one completed verdict/report or one typed no-report error.
type Service struct {
	Root string
	Deps ServiceDeps
}

func NewService(root string, deps ServiceDeps) *Service { return &Service{Root: root, Deps: deps} }

// ProbeAdoption distinguishes only an absent policy store from a valid loaded
// one. Every present-but-invalid store remains operational.
func ProbeAdoption(root string) (bool, error) {
	_, err := policyauthority.Load(root)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, policyauthority.ErrNotAdopted) {
		return false, nil
	}
	return false, operational("probe adoption", err)
}

// Evaluate follows the authority's ten stages in order and never returns a
// partial report on operational or not-adopted failure.
func (s *Service) Evaluate(ctx context.Context, request Request) (Result, error) {
	if s == nil {
		return Result{}, operational("evaluate", errors.New("nil service"))
	}
	if err := request.Validate(); err != nil {
		return Result{}, operational("validate request", err)
	}

	operands, err := ResolveOperands(ctx, s.Deps.Compiler, s.Root, request, contextcompile.ConflictFacts{Actors: s.Deps.Actors})
	if err != nil {
		if errors.Is(err, contextcompile.ErrNoConstitution) || errors.Is(err, policyauthority.ErrNotAdopted) {
			return Result{}, &NotAdoptedError{Err: err}
		}
		return Result{}, operational("resolve operands", err)
	}
	view, err := operands.View()
	if err != nil {
		return Result{}, operational("verify sealed operands", err)
	}
	if err := reverifyConflictView(view, request); err != nil {
		return Result{}, operational("reverify sealed operands", err)
	}

	mechanicalResult, err := EvaluateMechanical(ctx, MechanicalInput{
		Claims: view.TypedClaims, Profile: view.Profile, Actors: view.Actors, Refs: s.Deps.Refs,
	})
	if err != nil {
		return Result{}, operational("evaluate mechanical proofs", err)
	}

	if s.Deps.Dates == nil {
		return Result{}, operational("obtain evaluation date", errors.New("date source is nil"))
	}
	evaluatedOn, err := s.Deps.Dates.TodayUTC(ctx)
	if err != nil {
		return Result{}, operational("obtain evaluation date", err)
	}
	if err := validateEvaluatedOn("evaluated_on", evaluatedOn); err != nil {
		return Result{}, operational("obtain evaluation date", err)
	}

	targetSource, err := exactTargetSource(view.Snapshot, request)
	if err != nil {
		return Result{}, operational("derive target authority input", err)
	}
	authorityInput := AuthorityInput{
		EvaluatedOn:  evaluatedOn,
		TargetDigest: targetSource.ContentDigest,
		Profile:      view.Profile,
		Actors:       view.Actors,
		Exemptions:   view.Exemptions,
		Dispositions: dispositionsOf(view),
	}
	coverage, _ := s.Deps.Refs.(RefCoverageResolver)
	mechanical := make([]MechanicalEvaluation, 0, len(mechanicalResult.Evaluations))
	exemptionDisclosures := []Disclosure{}
	for _, row := range mechanicalResult.Evaluations {
		resolutions, disclosures, err := ResolveExemptionAuthority(ctx, authorityInput, row, coverage)
		if err != nil {
			return Result{}, operational("resolve exemption authority", err)
		}
		resolved, err := ApplyEffectiveExemptions(row, resolutions)
		if err != nil {
			return Result{}, operational("apply exemptions", err)
		}
		mechanical = append(mechanical, resolved)
		exemptionDisclosures = append(exemptionDisclosures, disclosures...)
	}

	semanticInput, err := BuildSemanticInput(view, mechanical)
	if err != nil {
		return Result{}, operational("build semantic input", err)
	}
	semantic := []SemanticEvaluation{}
	dispositionDisclosures := []Disclosure{}
	if semanticEvaluationRequired(semanticInput) {
		row, disclosures, err := s.evaluateSemantic(ctx, view, authorityInput, semanticInput)
		if err != nil {
			return Result{}, err
		}
		semantic = append(semantic, row)
		dispositionDisclosures = append(dispositionDisclosures, disclosures...)
	}

	inherited, err := compilerDisclosures(view.Snapshot.Disclosures)
	if err != nil {
		return Result{}, operational("derive compiler disclosures", err)
	}
	disclosures, err := mergeReportDisclosures(inherited, mechanicalResult.Disclosures, exemptionDisclosures, dispositionDisclosures)
	if err != nil {
		return Result{}, operational("merge report disclosures", err)
	}
	inputIdentity, _, err := reportInput(view, request, evaluatedOn)
	if err != nil {
		return Result{}, operational("derive report input", err)
	}
	report := Report{
		Schema:      ReportSchema,
		Input:       inputIdentity,
		Mechanical:  mechanical,
		Semantic:    semantic,
		Disclosures: disclosures,
	}
	report.Verdict = reportVerdict(report.Mechanical, report.Semantic, report.Disclosures)
	result, err := canonicalResult(report)
	if err != nil {
		return Result{}, operational("canonicalize report", err)
	}
	return result, nil
}

func dispositionsOf(view contextcompile.ConflictView) []policyartifact.Disposition {
	out := make([]policyartifact.Disposition, len(view.EffectivePolicy.Dispositions))
	for i, disposition := range view.EffectivePolicy.Dispositions {
		out[i] = disposition.Disposition
	}
	return out
}

func semanticEvaluationRequired(input SemanticInput) bool {
	return len(input.Claims) >= 2 || len(input.UnknownMechanicals) != 0
}

func (s *Service) evaluateSemantic(ctx context.Context, view contextcompile.ConflictView, authorityInput AuthorityInput, input SemanticInput) (SemanticEvaluation, []Disclosure, error) {
	inputBytes, err := canonjson.Marshal(semanticInputWitnessDoc{
		Claims: input.Claims, UnknownMechanicals: input.UnknownMechanicals, Exemptions: input.Exemptions,
	})
	if err != nil {
		return SemanticEvaluation{}, nil, operational("encode semantic input", err)
	}

	cache, err := s.prepareJudgeCache(ctx)
	if err != nil {
		return SemanticEvaluation{}, nil, operational("prepare judgment cache", err)
	}
	primary, err := runValidatedJudge(ctx, s.Deps.Primary, JudgePrimary, input, inputBytes, cache, view)
	if err != nil {
		return SemanticEvaluation{}, nil, operational("run primary judge", err)
	}
	challenger, err := runValidatedJudge(ctx, s.Deps.Challenger, JudgeChallenger, input, inputBytes, cache, view)
	if err != nil {
		return SemanticEvaluation{}, nil, operational("run challenger judge", err)
	}

	dispositions, disclosures, err := ResolveDispositionAuthority(authorityInput, input, primary, challenger)
	if err != nil {
		return SemanticEvaluation{}, nil, operational("resolve disposition authority", err)
	}
	inputID, err := semanticInputDigest(input)
	if err != nil {
		return SemanticEvaluation{}, nil, operational("derive semantic input identity", err)
	}

	disagreement, err := judgesDisagree(primary, challenger)
	if err != nil {
		return SemanticEvaluation{}, nil, operational("compare judge results", err)
	}
	fallbackEligible := humanFallbackEligible(view.Profile.Class, primary, challenger, disagreement)

	origins := make(map[string]policyartifact.DispositionOrigin, len(authorityInput.Dispositions))
	for _, disposition := range authorityInput.Dispositions {
		origins[disposition.ID] = disposition.Origin
	}
	state, reasons := deriveSemanticProof(view.Profile.Class, primary, challenger, dispositions, origins, fallbackEligible, disagreement)
	row := SemanticEvaluation{
		ID: inputID, InputID: inputID,
		Claims:             reportSemanticClaims(input.Claims),
		UnknownMechanicals: input.UnknownMechanicals,
		Dispositions:       dispositions,
		State:              state, Reasons: reasons,
	}
	if primary != nil {
		exchange := primary.Exchange
		row.Primary = &exchange
	}
	if challenger != nil {
		exchange := challenger.Exchange
		row.Challenger = &exchange
	}
	return row, disclosures, nil
}

// humanFallbackEligible is SI-116's one orchestration-only legality seam. It
// deliberately does not alter any of Task 8's five authority states.
func humanFallbackEligible(class governanceprincipal.Class, primary, challenger *ValidatedExchange, disagreement bool) bool {
	missingConfigured := primary == nil || (class == governanceprincipal.ClassHighAssurance && challenger == nil)
	inconclusive := exchangeInconclusive(primary) || exchangeInconclusive(challenger)
	return missingConfigured || inconclusive || disagreement
}

type judgeCacheContext struct {
	enabled  bool
	root     string
	treeHash string
}

func (s *Service) prepareJudgeCache(ctx context.Context) (judgeCacheContext, error) {
	adapters := make([]JudgeAdapter, 0, 2)
	for _, judge := range []Judge{s.Deps.Primary, s.Deps.Challenger} {
		switch adapter := judge.(type) {
		case JudgeAdapter:
			adapters = append(adapters, adapter)
		case *JudgeAdapter:
			if adapter == nil {
				return judgeCacheContext{}, fmt.Errorf("concrete judge adapter is nil")
			}
			adapters = append(adapters, *adapter)
		}
	}
	if len(adapters) == 0 {
		return judgeCacheContext{}, nil
	}
	root, err := canonicalCheckoutRoot(s.Root)
	if err != nil {
		return judgeCacheContext{}, fmt.Errorf("service root: %w", err)
	}
	for _, adapter := range adapters {
		adapterRoot, err := canonicalCheckoutRoot(adapter.Root)
		if err != nil {
			return judgeCacheContext{}, fmt.Errorf("adapter root: %w", err)
		}
		if adapterRoot != root {
			return judgeCacheContext{}, fmt.Errorf("adapter root %q does not match service root %q", adapterRoot, root)
		}
	}
	if s.Deps.TreeHasher == nil {
		return judgeCacheContext{}, fmt.Errorf("tree hasher is nil for concrete judge adapter")
	}
	treeHash, err := s.Deps.TreeHasher.TreeHash(ctx, root)
	if err != nil {
		return judgeCacheContext{}, fmt.Errorf("compute D4 tree hash: %w", err)
	}
	if err := validateBareHex("D4 tree hash", treeHash); err != nil {
		return judgeCacheContext{}, err
	}
	return judgeCacheContext{enabled: true, root: root, treeHash: treeHash}, nil
}

func canonicalCheckoutRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		return "", fmt.Errorf("resolved root %q is not absolute", resolved)
	}
	return filepath.Clean(resolved), nil
}

func runValidatedJudge(ctx context.Context, judge Judge, role JudgeRole, input SemanticInput, inputBytes []byte, cache judgeCacheContext, view contextcompile.ConflictView) (*ValidatedExchange, error) {
	if judge == nil {
		return nil, nil
	}
	var exchange JudgmentExchange
	var err error
	switch adapter := judge.(type) {
	case JudgeAdapter:
		if !cache.enabled {
			return nil, fmt.Errorf("concrete judge adapter has no prepared cache context")
		}
		adapter.Root = cache.root
		validated, cacheErr := CachedJudge(ctx, adapter, input, cache.treeHash, view.Profile.ID, view.Snapshot.ProfileDigest, view.Snapshot.EffectivePolicyDigest)
		exchange, err = validated.Exchange, cacheErr
	case *JudgeAdapter:
		if adapter == nil {
			return nil, fmt.Errorf("concrete judge adapter is nil")
		}
		if !cache.enabled {
			return nil, fmt.Errorf("concrete judge adapter has no prepared cache context")
		}
		copy := *adapter
		copy.Root = cache.root
		validated, cacheErr := CachedJudge(ctx, copy, input, cache.treeHash, view.Profile.ID, view.Snapshot.ProfileDigest, view.Snapshot.EffectivePolicyDigest)
		exchange, err = validated.Exchange, cacheErr
	default:
		exchange, err = judge.Judge(ctx, input.Prompt, inputBytes)
	}
	if err != nil {
		return nil, err
	}
	if err := exchange.validate(); err != nil {
		return nil, err
	}
	if exchange.Role != role {
		return nil, fmt.Errorf("judge returned role %q, want %q", exchange.Role, role)
	}
	if want := rawContentDigest(input.Prompt); exchange.PromptDigest != want {
		return nil, fmt.Errorf("judge prompt digest %s does not match exact prompt %s", exchange.PromptDigest, want)
	}
	if want := rawContentDigest(inputBytes); exchange.InputDigest != want {
		return nil, fmt.Errorf("judge input digest %s does not match exact input %s", exchange.InputDigest, want)
	}
	validated, err := ValidateJudgeResult(input, exchange.Result)
	if err != nil {
		return nil, err
	}
	validated.Exchange = exchange
	return &validated, nil
}

func exchangeInconclusive(exchange *ValidatedExchange) bool {
	return exchange != nil && exchange.Exchange.Result.Recommendation == RecommendationInconclusive
}

func judgesDisagree(primary, challenger *ValidatedExchange) (bool, error) {
	if primary == nil || challenger == nil {
		return false, nil
	}
	if primary.Exchange.Result.Recommendation != challenger.Exchange.Result.Recommendation {
		return true, nil
	}
	if exchangeInconclusive(primary) || exchangeInconclusive(challenger) {
		return true, nil
	}
	left, err := findingWitnessSet(primary.Exchange.Result.Findings)
	if err != nil {
		return false, err
	}
	right, err := findingWitnessSet(challenger.Exchange.Result.Findings)
	if err != nil {
		return false, err
	}
	return !reflect.DeepEqual(left, right), nil
}

func findingWitnessSet(findings []JudgeFinding) ([]string, error) {
	set := make(map[string]bool, len(findings))
	for _, finding := range findings {
		digest, err := canonjson.Digest(struct {
			Claims []ClaimWitness
		}{Claims: finding.Claims})
		if err != nil {
			return nil, err
		}
		set[digest] = true
	}
	out := make([]string, 0, len(set))
	for digest := range set {
		out = append(out, digest)
	}
	sort.Strings(out)
	return out, nil
}

func deriveSemanticProof(class governanceprincipal.Class, primary, challenger *ValidatedExchange, dispositions []DispositionResolution, origins map[string]policyartifact.DispositionOrigin, fallbackEligible, disagreement bool) (ProofState, []ReasonCode) {
	effectiveNoConflict := false
	effectiveConflict := false
	for _, disposition := range dispositions {
		legal := allProven(disposition.Resolution)
		if origins[disposition.ID] == policyartifact.DispositionHumanFallback && !fallbackEligible {
			legal = false
		}
		if !legal {
			continue
		}
		switch disposition.Conclusion {
		case policyartifact.DispositionConflict:
			effectiveConflict = true
		case policyartifact.DispositionNoConflict:
			effectiveNoConflict = true
		}
	}
	if effectiveConflict {
		return ProofViolatedWithWitness, []ReasonCode{ReasonDispositionEffectiveConflict}
	}
	if effectiveNoConflict {
		reasons := []ReasonCode{ReasonDispositionEffectiveNoConflict}
		if class == governanceprincipal.ClassExperimental {
			return ProofUnproven, addReason(reasons, ReasonProfileExperimental)
		}
		return ProofProven, reasons
	}

	reasons := []ReasonCode{}
	if primary == nil {
		reasons = addReason(reasons, ReasonJudgeUnavailable)
	}
	if exchangeInconclusive(primary) || exchangeInconclusive(challenger) {
		reasons = addReason(reasons, ReasonJudgeInconclusive)
	}
	if class == governanceprincipal.ClassHighAssurance && challenger == nil {
		reasons = addReason(reasons, ReasonChallengerUnavailable)
	}
	if disagreement {
		reasons = addReason(reasons, ReasonJudgmentDisagreement)
	}
	if len(dispositions) == 0 {
		reasons = addReason(reasons, ReasonDispositionRequired)
	} else {
		reasons = addReason(reasons, ReasonDispositionIneffective)
	}
	if class == governanceprincipal.ClassExperimental {
		reasons = addReason(reasons, ReasonProfileExperimental)
	}
	return ProofUnproven, reasons
}

func reverifyConflictView(view contextcompile.ConflictView, request Request) error {
	snapshot := view.Snapshot
	if err := snapshot.Repository.Validate(); err != nil {
		return err
	}
	effectiveDigest, err := view.EffectivePolicy.Digest()
	if err != nil {
		return err
	}
	if effectiveDigest != snapshot.EffectivePolicyDigest || view.EffectivePolicy.ConstitutionDigest != snapshot.ConstitutionDigest || view.EffectivePolicy.ProfileID != snapshot.ProfileID || view.EffectivePolicy.ProfileDigest != snapshot.ProfileDigest {
		return fmt.Errorf("effective policy identity does not match sealed snapshot")
	}
	profileDigest, err := view.Profile.Digest()
	if err != nil {
		return err
	}
	if view.Profile.ID != snapshot.ProfileID || profileDigest != snapshot.ProfileDigest {
		return fmt.Errorf("governance profile identity does not match sealed snapshot")
	}
	for _, source := range snapshot.Sources {
		if source.Ref == "" || source.Path == "" {
			return fmt.Errorf("sealed source has empty ref/path")
		}
		if err := validateDigest("sealed source digest", source.ContentDigest); err != nil {
			return err
		}
	}
	if _, err := exactTargetSource(snapshot, request); err != nil {
		return err
	}

	var adapter contextcompile.AdapterRef
	var scope policyartifact.Scope
	var grants execworkspace.GrantSet
	switch request.Target.Kind {
	case TargetAcceptedContext:
		if snapshot.TargetKind != string(TargetAcceptedContext) || snapshot.CandidateDigest != "" || snapshot.CandidateBlob != "" {
			return fmt.Errorf("sealed snapshot does not match accepted target arm")
		}
		if err := validateDigest("sealed accepted manifest digest", snapshot.ManifestDigest); err != nil {
			return err
		}
		accepted := request.Target.AcceptedContext
		adapter, scope, grants = accepted.Adapter, accepted.Scope, accepted.Grants
		if snapshot.Phase != accepted.Phase {
			return fmt.Errorf("accepted phase %q does not match sealed phase %q", accepted.Phase, snapshot.Phase)
		}
		if accepted.Expected != nil {
			if !snapshot.Repository.Branch.Known || snapshot.Repository.Branch.Value != accepted.Expected.Branch || !snapshot.Repository.Head.Known || snapshot.Repository.Head.Value != accepted.Expected.Head {
				return fmt.Errorf("accepted expected repository identity does not match sealed snapshot")
			}
		}
	case TargetAcceptanceCandidate:
		if snapshot.TargetKind != string(TargetAcceptanceCandidate) || snapshot.ManifestDigest != "" {
			return fmt.Errorf("sealed snapshot does not match candidate target arm")
		}
		if snapshot.Phase != contextcompile.PhaseDesign {
			return fmt.Errorf("candidate snapshot phase %q is not design", snapshot.Phase)
		}
		if err := validateDigest("sealed candidate digest", snapshot.CandidateDigest); err != nil {
			return err
		}
		if !fullHexRe.MatchString(snapshot.CandidateBlob) {
			return fmt.Errorf("sealed candidate blob is not a full Git object ID")
		}
		candidate := request.Target.AcceptanceCandidate
		adapter, scope, grants = candidate.Adapter, candidate.Scope, candidate.Grants
		if !snapshot.Repository.Branch.Known || snapshot.Repository.Branch.Value != candidate.Expected.Branch || !snapshot.Repository.Head.Known || snapshot.Repository.Head.Value != candidate.Expected.Head {
			return fmt.Errorf("candidate expected repository identity does not match sealed snapshot")
		}
	default:
		return fmt.Errorf("unknown target kind %q", request.Target.Kind)
	}
	if snapshot.Adapter != adapter || !reflect.DeepEqual(snapshot.Scope, scope) {
		return fmt.Errorf("request adapter/scope does not match sealed snapshot")
	}
	grantBytes, err := execworkspace.EncodeGrantSet(grants)
	if err != nil {
		return err
	}
	if rawContentDigest(grantBytes) != snapshot.GrantDigest {
		return fmt.Errorf("request grants do not match sealed snapshot")
	}

	entries := make(map[string]contextcompile.ConflictPolicyIdentity, len(snapshot.PolicyEntries))
	for _, entry := range snapshot.PolicyEntries {
		if err := validateDigest("sealed policy entry digest", entry.Digest); err != nil {
			return err
		}
		key := entry.Kind + "\x00" + entry.ID
		if _, exists := entries[key]; exists {
			return fmt.Errorf("duplicate sealed policy entry %q", entry.ID)
		}
		entries[key] = entry
	}
	for _, claim := range view.TypedClaims {
		entry, ok := entries[contextcompile.PolicyEntryPolicy+"\x00"+claim.PolicyID]
		if !ok || entry.Digest != claim.PolicyDigest {
			return fmt.Errorf("typed claim policy %q is absent or digest-mismatched in sealed ledger", claim.PolicyID)
		}
	}
	sourceSet := make(map[contextcompile.ConflictSourceIdentity]bool, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		sourceSet[source] = true
	}
	for _, claim := range view.ProseClaims {
		identity := contextcompile.ConflictSourceIdentity{Ref: claim.SourceRef, Path: claim.SourcePath, ContentDigest: claim.SourceDigest}
		if !sourceSet[identity] {
			return fmt.Errorf("prose claim %q source is absent from sealed source ledger", claim.ID)
		}
		if claim.AuthorityDigest != claim.SourceDigest {
			return fmt.Errorf("prose claim %q authority digest does not match its exact authoring source", claim.ID)
		}
	}
	for _, exemption := range view.Exemptions {
		digest, err := exemption.Digest()
		if err != nil {
			return err
		}
		entry, ok := entries[contextcompile.PolicyEntryExemption+"\x00"+exemption.ID]
		if !ok || entry.Digest != digest {
			return fmt.Errorf("exemption %q is absent or digest-mismatched in sealed ledger", exemption.ID)
		}
	}
	for _, disposition := range view.EffectivePolicy.Dispositions {
		digest, err := disposition.Disposition.Digest()
		if err != nil {
			return err
		}
		if disposition.ID != disposition.Disposition.ID || disposition.Digest != digest {
			return fmt.Errorf("effective disposition %q identity is inconsistent", disposition.ID)
		}
	}
	for _, code := range snapshot.Disclosures {
		if err := validateDisclosureCode(code); err != nil {
			return err
		}
	}
	return nil
}
