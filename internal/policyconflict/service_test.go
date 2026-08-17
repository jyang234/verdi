package policyconflict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

type serviceDateSource string

func (d serviceDateSource) TodayUTC(context.Context) (string, error) { return string(d), nil }

type serviceJudge struct {
	role   JudgeRole
	result JudgeResult
	inputs [][]byte
	err    error
}

func (j *serviceJudge) Judge(_ context.Context, prompt, input []byte) (JudgmentExchange, error) {
	if j.err != nil {
		return JudgmentExchange{}, j.err
	}
	j.inputs = append(j.inputs, append([]byte(nil), input...))
	raw, err := EncodeJudgeResult(j.result)
	if err != nil {
		return JudgmentExchange{}, err
	}
	return JudgmentExchange{
		Role:          j.role,
		Adapter:       contextcompile.AdapterRef{ID: "fixture-judge", Version: "1"},
		Model:         "fixture-model",
		CommandDigest: rawContentDigest([]byte("fixture-judge")),
		PromptDigest:  rawContentDigest(prompt),
		InputDigest:   rawContentDigest(input),
		RawResult:     string(raw),
		RawDigest:     rawContentDigest(raw),
		Result:        j.result,
	}, nil
}

func serviceInconclusiveJudge(role JudgeRole) *serviceJudge {
	return &serviceJudge{role: role, result: JudgeResult{
		Schema: JudgeResultSchema, Recommendation: RecommendationInconclusive, Findings: []JudgeFinding{},
	}}
}

func serviceAcceptedRequest() Request {
	compileRequest := acceptedOperandRequest("spec/operand-feature")
	compileRequest.Scope.Paths = []string{"cmd/"}
	return Request{
		Schema: RequestSchema,
		Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &compileRequest},
	}
}

func serviceNoConflictJudge() *serviceJudge {
	return &serviceJudge{
		role: JudgePrimary,
		result: JudgeResult{
			Schema:         JudgeResultSchema,
			Recommendation: RecommendationNoConflict,
			Findings:       []JudgeFinding{},
		},
	}
}

func newServiceFixture(t *testing.T, primary Judge) (*Service, *fixturegit.Repo) {
	t.Helper()
	repo := operandFixtureRepo(t)
	return NewService(repo.Dir, ServiceDeps{
		Compiler: contextcompile.NewCompiler(),
		Refs:     noCallResolver{t: t},
		Primary:  primary,
		Dates:    serviceDateSource("2026-08-12"),
	}), repo
}

func TestServiceEvaluateSemanticNoConflictRequiresDisposition(t *testing.T) {
	judge := serviceNoConflictJudge()
	service, _ := newServiceFixture(t, judge)

	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Report.Verdict != VerdictBlockedUnproven {
		t.Fatalf("Verdict = %q, want %q", result.Report.Verdict, VerdictBlockedUnproven)
	}
	if len(result.Report.Semantic) != 1 {
		t.Fatalf("Semantic rows = %d, want 1", len(result.Report.Semantic))
	}
	row := result.Report.Semantic[0]
	if row.State != ProofUnproven || !containsReason(row.Reasons, ReasonDispositionRequired) {
		t.Fatalf("semantic row = %+v, want unproven disposition-required", row)
	}
	if row.Primary == nil || row.Primary.Result.Recommendation != RecommendationNoConflict {
		t.Fatalf("primary exchange = %+v, want validated no-conflict exchange", row.Primary)
	}
	if len(judge.inputs) != 1 {
		t.Fatalf("judge calls = %d, want 1", len(judge.inputs))
	}
	decoded, err := DecodeReport(result.ReportBytes)
	if err != nil {
		t.Fatalf("DecodeReport: %v", err)
	}
	if decoded.Digest == "" || decoded.Digest != result.Report.Digest {
		t.Fatalf("decoded digest = %q, report digest = %q", decoded.Digest, result.Report.Digest)
	}
	if decoded.Input.Target.Accepted == nil {
		t.Fatal("accepted target identity is absent")
	}
	for _, key := range [][]byte{[]byte(`"input"`), []byte(`"policy_entries"`), []byte(`"effective_policy_digest"`)} {
		if got := bytes.Count(result.ReportBytes, key); got != 1 {
			t.Fatalf("report key %s occurs %d times, want exactly one input ledger occurrence", key, got)
		}
	}
}

func TestServiceEvaluateMissingJudgeIsCompletedUnproven(t *testing.T) {
	service, _ := newServiceFixture(t, nil)
	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Report.Verdict != VerdictBlockedUnproven || len(result.Report.Semantic) != 1 {
		t.Fatalf("result = %+v, want one blocked-unproven semantic row", result.Report)
	}
	if !containsReason(result.Report.Semantic[0].Reasons, ReasonJudgeUnavailable) {
		t.Fatalf("reasons = %v, want %q", result.Report.Semantic[0].Reasons, ReasonJudgeUnavailable)
	}
}

func TestServiceEvaluateInconclusiveJudgeIsCompletedUnproven(t *testing.T) {
	service, _ := newServiceFixture(t, serviceInconclusiveJudge(JudgePrimary))
	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Report.Verdict != VerdictBlockedUnproven || !containsReason(result.Report.Semantic[0].Reasons, ReasonJudgeInconclusive) {
		t.Fatalf("result = %+v, want completed judge-inconclusive row", result.Report)
	}
}

func TestServiceEvaluateConstitutionAbsenceIsNotAdopted(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
		".verdi/specs/active/operand-feature/spec.md": operandFeatureSpec,
	}, Message: "spec only"}})
	service := NewService(repo.Dir, ServiceDeps{
		Compiler: contextcompile.NewCompiler(),
		Dates:    serviceDateSource("2026-08-12"),
	})

	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if !IsNotAdopted(err) {
		t.Fatalf("Evaluate error = %T %v, want typed not-adopted", err, err)
	}
	if len(result.ReportBytes) != 0 || result.Report.Schema != "" {
		t.Fatalf("result = %+v, want no report on refusal", result)
	}
	adopted, probeErr := ProbeAdoption(repo.Dir)
	if probeErr != nil || adopted {
		t.Fatalf("ProbeAdoption = (%v, %v), want (false, nil)", adopted, probeErr)
	}

	incomplete := t.TempDir()
	if err := os.MkdirAll(filepath.Join(incomplete, ".verdi", "policy"), 0o755); err != nil {
		t.Fatalf("MkdirAll incomplete policy store: %v", err)
	}
	adopted, probeErr = ProbeAdoption(incomplete)
	if adopted || !IsOperational(probeErr) {
		t.Fatalf("ProbeAdoption incomplete store = (%v, %T %v), want (false, operational)", adopted, probeErr, probeErr)
	}
}

func TestServiceEvaluateCandidateUsesSealedIdentity(t *testing.T) {
	files := operandPolicyStoreFiles(t)
	files[".verdi/specs/active/operand-candidate/spec.md"] = operandCandidateSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "candidate"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	request := Request{
		Schema: RequestSchema,
		Target: Target{Kind: TargetAcceptanceCandidate, AcceptanceCandidate: &AcceptanceCandidate{
			Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Expected: contextcompile.Expected{Branch: "main", Head: repo.Head},
			Grants:   execworkspace.GrantSet{}, Scope: universalScope(), Spec: "spec/operand-candidate",
		}},
	}
	service := NewService(repo.Dir, ServiceDeps{
		Compiler: contextcompile.NewCompiler(), Refs: noCallResolver{t: t},
		Primary: serviceNoConflictJudge(), Dates: serviceDateSource("2026-08-12"),
	})

	result, err := service.Evaluate(context.Background(), request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	candidate := result.Report.Input.Target.Candidate
	if candidate == nil {
		t.Fatal("candidate target identity is absent")
	}
	if candidate.Ref != "spec/operand-candidate" || candidate.Path != ".verdi/specs/active/operand-candidate/spec.md" {
		t.Fatalf("candidate source = %q %q, want exact sealed spec source", candidate.Ref, candidate.Path)
	}
	if candidate.Branch != "main" || candidate.Head != repo.Head || candidate.Blob == "" || candidate.ContentDigest == "" {
		t.Fatalf("candidate Git/content identity is incomplete: %+v", candidate)
	}
}

func TestServiceEvaluateJudgeFailureIsOperationalWithoutReport(t *testing.T) {
	service, _ := newServiceFixture(t, &serviceJudge{err: errors.New("judge unavailable")})
	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if !IsOperational(err) {
		t.Fatalf("Evaluate error = %T %v, want operational", err, err)
	}
	if result.Report.Schema != "" || len(result.ReportBytes) != 0 {
		t.Fatalf("result = %+v, want no report on judge failure", result)
	}
}

func TestServiceEvaluateMalformedDateIsOperationalWithoutReport(t *testing.T) {
	service, _ := newServiceFixture(t, serviceNoConflictJudge())
	service.Deps.Dates = serviceDateSource("not-a-date")
	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if !IsOperational(err) {
		t.Fatalf("Evaluate error = %T %v, want operational", err, err)
	}
	if result.Report.Schema != "" || len(result.ReportBytes) != 0 {
		t.Fatalf("result = %+v, want no report on malformed date", result)
	}
}

func TestServiceEvaluateActorPermutationIsDeterministic(t *testing.T) {
	repo := operandFixtureRepo(t)
	alice := authorityResolve(t, "alice", authenticatedFact("alice"))
	bob := authorityResolve(t, "bob", authenticatedFact("bob"))
	evaluate := func(actors []governanceprincipal.PrincipalResolution) Result {
		t.Helper()
		service := NewService(repo.Dir, ServiceDeps{
			Compiler: contextcompile.NewCompiler(), Refs: noCallResolver{t: t},
			Primary: serviceNoConflictJudge(), Dates: serviceDateSource("2026-08-12"), Actors: actors,
		})
		result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		return result
	}
	first := evaluate([]governanceprincipal.PrincipalResolution{alice, bob})
	second := evaluate([]governanceprincipal.PrincipalResolution{bob, alice})
	assertSameReportBytes(t, first.ReportBytes, second.ReportBytes)
}

func TestServiceEvaluateDispositionAndExemptionUseOneSemanticIdentity(t *testing.T) {
	repo, request, actors, baselineInputID, baselineAuthorityDigest := serviceDispositionRepo(t)
	judge := serviceNoConflictJudge()
	service := NewService(repo.Dir, ServiceDeps{
		Compiler: contextcompile.NewCompiler(), Refs: noCallResolver{t: t}, Primary: judge,
		Dates: serviceDateSource("2026-08-12"), Actors: actors,
	})

	result, err := service.Evaluate(context.Background(), request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Report.Verdict != VerdictPass || len(result.Report.Semantic) != 1 {
		t.Fatalf("report = %+v, want pass with one semantic row", result.Report)
	}
	row := result.Report.Semantic[0]
	if row.InputID != baselineInputID || row.State != ProofProven || !containsReason(row.Reasons, ReasonDispositionEffectiveNoConflict) {
		t.Fatalf("semantic row = %+v, want baseline identity and effective no-conflict disposition", row)
	}
	if len(row.Dispositions) != 1 || !allProven(row.Dispositions[0].Resolution) {
		t.Fatalf("dispositions = %+v, want one all-proven resolution", row.Dispositions)
	}
	if len(judge.inputs) != 1 {
		t.Fatalf("judge calls = %d, want 1", len(judge.inputs))
	}
	var shown semanticInputWitnessDoc
	if err := json.Unmarshal(judge.inputs[0], &shown); err != nil {
		t.Fatalf("decode judge input: %v", err)
	}
	if len(shown.Exemptions) != 1 || shown.Exemptions[0].ID != "policy-exemption/legacy-service-go" {
		t.Fatalf("judge exemptions = %+v, want the effective exemption in the sole input", shown.Exemptions)
	}
	if result.Report.Input.EffectivePolicyDigest == baselineAuthorityDigest {
		t.Fatal("effective policy digest did not change after adding the disposition")
	}
	if result.Report.Input.Target.Accepted == nil || result.Report.Input.Target.Accepted.ManifestDigest == "" {
		t.Fatal("accepted manifest digest is not separately bound")
	}
	if len(result.Report.Input.PolicyEntries) == 0 {
		t.Fatal("applicable sealed policy-entry ledger is empty")
	}
}

func serviceDispositionRepo(t *testing.T) (*fixturegit.Repo, Request, []governanceprincipal.PrincipalResolution, string, string) {
	t.Helper()
	files := operandPolicyStoreFiles(t)
	profilePath := ".verdi/policy/profiles/solo-default.md"
	files[profilePath] = strings.Replace(files[profilePath],
		"applicable_transitions: [accept]",
		"applicable_transitions: [accept, policy-disposition-approval, policy-exemption-approval]", 1)
	files[".verdi/policy/constitution.md"] = strings.Replace(files[".verdi/policy/constitution.md"],
		"transitions: [accept, close]",
		"transitions: [accept, close, policy-disposition-approval, policy-exemption-approval]", 1)
	exemptionPath := ".verdi/policy/exemptions/legacy-service-go.md"
	files[exemptionPath] = strings.Replace(files[exemptionPath], `paths: ["services/legacy/"]`, "paths: []", 1)
	files[".verdi/specs/active/operand-feature/spec.md"] = operandFeatureSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "authority"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	runOperandGit(t, repo.Dir, "add", "-A")
	runOperandGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "projection")
	repo.Head = strings.TrimSpace(runOperandGit(t, repo.Dir, "rev-parse", "HEAD"))

	compileRequest := acceptedOperandRequest("spec/operand-feature")
	request := Request{Schema: RequestSchema, Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &compileRequest}}
	operands, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, request, contextcompile.ConflictFacts{})
	if err != nil {
		t.Fatalf("ResolveOperands profile pass: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View profile pass: %v", err)
	}
	fact := governanceprincipal.TrustFact{
		SourceID: "github-org", SourceKind: governanceprincipal.TrustSourceForge,
		Available: true, Valid: true, Subjects: []string{"alice"}, EvidenceDigest: testDigest64,
	}
	resolver := governanceprincipal.NewResolver(staticFact(fact))
	actor, err := resolver.Resolve(context.Background(), view.Profile, governanceprincipal.PrincipalClaim{TrustSource: "github-org", Subject: "alice"})
	if err != nil {
		t.Fatalf("resolve actor: %v", err)
	}
	actors := []governanceprincipal.PrincipalResolution{actor}

	operands, err = ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, request, contextcompile.ConflictFacts{Actors: actors})
	if err != nil {
		t.Fatalf("ResolveOperands witness pass: %v", err)
	}
	view, err = operands.View()
	if err != nil {
		t.Fatalf("View witness pass: %v", err)
	}
	for _, claim := range view.ProseClaims {
		if claim.AuthorityDigest != claim.SourceDigest {
			t.Fatalf("claim %q authority digest %q does not equal authoring source digest %q", claim.ID, claim.AuthorityDigest, claim.SourceDigest)
		}
	}
	mechanical, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: view.TypedClaims, Profile: view.Profile, Actors: actors, Refs: noCallResolver{t: t}})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	target, err := exactTargetSource(view.Snapshot, request)
	if err != nil {
		t.Fatalf("exactTargetSource: %v", err)
	}
	authorityInput := AuthorityInput{EvaluatedOn: "2026-08-12", TargetDigest: target.ContentDigest, Profile: view.Profile, Actors: actors, Exemptions: view.Exemptions}
	rows := make([]MechanicalEvaluation, 0, len(mechanical.Evaluations))
	for _, row := range mechanical.Evaluations {
		resolutions, _, err := ResolveExemptionAuthority(context.Background(), authorityInput, row, nil)
		if err != nil {
			t.Fatalf("ResolveExemptionAuthority: %v", err)
		}
		applied, err := ApplyEffectiveExemptions(row, resolutions)
		if err != nil {
			t.Fatalf("ApplyEffectiveExemptions: %v", err)
		}
		rows = append(rows, applied)
	}
	semanticInput, err := BuildSemanticInput(view, rows)
	if err != nil {
		t.Fatalf("BuildSemanticInput: %v", err)
	}
	inputID, err := semanticInputDigest(semanticInput)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	if len(semanticInput.Exemptions) != 1 || semanticInput.Exemptions[0].ID != "policy-exemption/legacy-service-go" {
		t.Fatalf("semantic exemptions = %+v, want the effective fixture exemption", semanticInput.Exemptions)
	}

	principal, err := governanceprincipal.CanonicalPrincipalID("github-org", "alice")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	disposition := serviceDispositionDocument(inputID, target.ContentDigest, witnessClaimsFrom(semanticInput.Claims), semanticInput.Exemptions, string(principal))
	dispositionPath := filepath.Join(repo.Dir, ".verdi", "policy", "dispositions", "current-no-conflict.md")
	if err := os.MkdirAll(filepath.Dir(dispositionPath), 0o755); err != nil {
		t.Fatalf("MkdirAll dispositions: %v", err)
	}
	if err := os.WriteFile(dispositionPath, []byte(disposition), 0o644); err != nil {
		t.Fatalf("write disposition: %v", err)
	}
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate after disposition: %v", err)
	}
	runOperandGit(t, repo.Dir, "add", "-A")
	runOperandGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "add disposition")
	repo.Head = strings.TrimSpace(runOperandGit(t, repo.Dir, "rev-parse", "HEAD"))
	return repo, request, actors, inputID, view.Snapshot.EffectivePolicyDigest
}

func serviceDispositionDocument(inputID, targetDigest string, claims []policyartifact.SemanticClaimWitness, exemptions []policyartifact.SemanticExemptionWitness, principal string) string {
	return fmt.Sprintf(`---
schema: verdi.policy-disposition/v1
id: policy-disposition/current-no-conflict
kind: policy-disposition
title: "Current no conflict"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
witness:
  input_id: %q
  target_digest: %q
  claims:
%s%sconclusion: no-conflict
origin: judge-result
judgment:
  primary_digest: %q
approvals:
  - role: policy-owner
    principal: %s
template: {identity: "test", digest: %q}
---
Current semantic input is approved as non-conflicting.
`, inputID, targetDigest, witnessClaimsYAML(claims), witnessExemptionsYAML(exemptions), testDigest64, principal, testDigest64)
}

func TestServiceEvaluateHumanFallbackLegality(t *testing.T) {
	conclusive := &ValidatedExchange{Exchange: JudgmentExchange{Result: JudgeResult{Recommendation: RecommendationNoConflict}}}
	inconclusive := &ValidatedExchange{Exchange: JudgmentExchange{Result: JudgeResult{Recommendation: RecommendationInconclusive}}}
	tests := []struct {
		name         string
		class        governanceprincipal.Class
		primary      *ValidatedExchange
		challenger   *ValidatedExchange
		disagreement bool
		want         bool
	}{
		{"primary absent", governanceprincipal.ClassTeam, nil, nil, false, true},
		{"well-formed inconclusive", governanceprincipal.ClassTeam, inconclusive, nil, false, true},
		{"judges disagree", governanceprincipal.ClassHighAssurance, conclusive, conclusive, true, true},
		{"required challenger absent", governanceprincipal.ClassHighAssurance, conclusive, nil, false, true},
		{"conclusive configured result", governanceprincipal.ClassTeam, conclusive, nil, false, false},
		{"conclusive agreeing judges", governanceprincipal.ClassHighAssurance, conclusive, conclusive, false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := humanFallbackEligible(test.class, test.primary, test.challenger, test.disagreement); got != test.want {
				t.Fatalf("humanFallbackEligible = %v, want %v", got, test.want)
			}
		})
	}

	all := AuthorityResolution{Match: ProofProven, Freshness: ProofProven, Scope: ProofProven, Bound: ProofProven, Authorization: ProofProven}
	disposition := DispositionResolution{ID: "policy-disposition/fallback", Conclusion: policyartifact.DispositionNoConflict, Resolution: all}
	origins := map[string]policyartifact.DispositionOrigin{disposition.ID: policyartifact.DispositionHumanFallback}
	state, reasons := deriveSemanticProof(governanceprincipal.ClassTeam, conclusive, nil, []DispositionResolution{disposition}, origins, false, false)
	if state != ProofUnproven || !containsReason(reasons, ReasonDispositionIneffective) {
		t.Fatalf("conclusive fallback result = (%q, %v), want unproven/ineffective", state, reasons)
	}
	state, reasons = deriveSemanticProof(governanceprincipal.ClassTeam, nil, nil, []DispositionResolution{disposition}, origins, true, false)
	if state != ProofProven || !containsReason(reasons, ReasonDispositionEffectiveNoConflict) {
		t.Fatalf("eligible fallback result = (%q, %v), want proven/effective", state, reasons)
	}
}

func TestServiceEvaluateHighAssuranceAndExperimentalProofs(t *testing.T) {
	all := AuthorityResolution{Match: ProofProven, Freshness: ProofProven, Scope: ProofProven, Bound: ProofProven, Authorization: ProofProven}
	judgeDisposition := DispositionResolution{ID: "policy-disposition/current", Conclusion: policyartifact.DispositionNoConflict, Resolution: all}
	origins := map[string]policyartifact.DispositionOrigin{judgeDisposition.ID: policyartifact.DispositionJudgeResult}
	state, reasons := deriveSemanticProof(governanceprincipal.ClassExperimental, nil, nil, []DispositionResolution{judgeDisposition}, origins, true, false)
	if state != ProofUnproven || !containsReason(reasons, ReasonProfileExperimental) {
		t.Fatalf("experimental result = (%q, %v), want unproven/profile-experimental", state, reasons)
	}
	state, reasons = deriveSemanticProof(governanceprincipal.ClassHighAssurance, &ValidatedExchange{}, nil, nil, nil, true, false)
	if state != ProofUnproven || !containsReason(reasons, ReasonChallengerUnavailable) {
		t.Fatalf("high-assurance result = (%q, %v), want challenger-unavailable", state, reasons)
	}

	conflict := judgeDisposition
	conflict.ID = "policy-disposition/conflict"
	conflict.Conclusion = policyartifact.DispositionConflict
	state, reasons = deriveSemanticProof(governanceprincipal.ClassExperimental, nil, nil, []DispositionResolution{conflict}, map[string]policyartifact.DispositionOrigin{conflict.ID: policyartifact.DispositionJudgeResult}, true, false)
	if state != ProofViolatedWithWitness || !containsReason(reasons, ReasonDispositionEffectiveConflict) {
		t.Fatalf("effective conflict result = (%q, %v), want violated/effective-conflict", state, reasons)
	}

	stale := judgeDisposition
	stale.Resolution.Freshness = ProofViolatedWithWitness
	state, reasons = deriveSemanticProof(governanceprincipal.ClassTeam, nil, nil, []DispositionResolution{stale}, origins, true, false)
	if state != ProofUnproven || !containsReason(reasons, ReasonDispositionIneffective) {
		t.Fatalf("stale disposition result = (%q, %v), want unproven/ineffective", state, reasons)
	}
	unauthorized := judgeDisposition
	unauthorized.Resolution.Authorization = ProofUnproven
	state, reasons = deriveSemanticProof(governanceprincipal.ClassTeam, nil, nil, []DispositionResolution{unauthorized}, origins, true, false)
	if state != ProofUnproven || !containsReason(reasons, ReasonDispositionIneffective) {
		t.Fatalf("unauthorized disposition result = (%q, %v), want unproven/ineffective", state, reasons)
	}
}

func TestServiceEvaluateJudgeDisagreementUsesWitnessSets(t *testing.T) {
	base := JudgeResult{Recommendation: RecommendationNoConflict, Findings: []JudgeFinding{}}
	primary := &ValidatedExchange{Exchange: JudgmentExchange{Result: base}}
	challenger := &ValidatedExchange{Exchange: JudgmentExchange{Result: base}}
	disagree, err := judgesDisagree(primary, challenger)
	if err != nil || disagree {
		t.Fatalf("identical judgments disagree = %v, err = %v", disagree, err)
	}
	challenger.Exchange.Result.Recommendation = RecommendationInconclusive
	disagree, err = judgesDisagree(primary, challenger)
	if err != nil || !disagree {
		t.Fatalf("different recommendations disagree = %v, err = %v", disagree, err)
	}
}

func containsReason(reasons []ReasonCode, want ReasonCode) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func assertSameReportBytes(t *testing.T, left, right []byte) {
	t.Helper()
	if !bytes.Equal(left, right) {
		t.Fatalf("reports differ:\nleft:  %s\nright: %s", left, right)
	}
}
