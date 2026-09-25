package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinessload"
)

// localOperatorConstitutionMD declares the one local-operator identity
// source under a solo profile, plus the disposition-approval transition
// the design assigns it (2026-09-05 local-operator disposition design
// §2.1-§2.2).
const localOperatorConstitutionMD = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Local-operator wiring fixture constitution"
owners: [platform-team]
selected_profile: solo-local
environments: [local, production]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close, policy-disposition-approval]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: [make-verify]
  configuration: [go-version]
  capability: []
  resource: [repo-tree]
  identity: [exemption-approval]
  evidence: [verify-receipt]
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Fixture-only constitution proving the wiring seam alone: no specs, no
dispositions — just enough for policyauthority.Load/SelectedProfile to
resolve a profile declaring a local-operator trust source.
`

const localOperatorWiringProfileMD = `---
schema: verdi.governance-profile/v1
id: solo-local
class: solo
applicable_transitions: [policy-disposition-approval]
identity_trust_sources:
  - {id: local, kind: local-operator}
role_mappings:
  - {role: policy-owner, trust_source: local, subjects: [wiring-fixture@example.com]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Fixture-only profile: binds "local" to policy-owner for exactly one
subject, matching the git identity fixturegit.Build configures.
`

// buildLocalOperatorWiringRepo builds a minimal adopted store whose
// selected profile declares a local-operator source — enough for
// readinessload.NewConflictProvider's factory-time profile resolution, with
// no specs/dispositions at all (nothing here ever calls .Evaluate).
func buildLocalOperatorWiringRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
		".verdi/verdi.yaml":                    "schema: verdi.layout/v1\n",
		".verdi/policy/constitution.md":        localOperatorConstitutionMD,
		".verdi/policy/profiles/solo-local.md": localOperatorWiringProfileMD,
	}, Message: "adopt a local-operator-declaring store"}})
}

// asPolicyConflictService type-asserts a VerdictProvider down to the one
// concrete production type the factory ever returns, so a test can inspect
// exactly which ServiceDeps.Actors the factory wired.
func asPolicyConflictService(t *testing.T, provider policyconflict.VerdictProvider) *policyconflict.Service {
	t.Helper()
	svc, ok := provider.(*policyconflict.Service)
	if !ok {
		t.Fatalf("provider is %T, want *policyconflict.Service", provider)
	}
	return svc
}

// TestNewLocalContextConflictProvider_NoLocalOperatorSource_NilActors
// proves the byte-identity guarantee at the narrowest possible seam: a
// profile that never declares a local-operator source resolves Actors to
// exactly nil, the same value context_conflict.go hardcoded before this
// wiring existed.
func TestNewLocalContextConflictProvider_NoLocalOperatorSource_NilActors(t *testing.T) {
	t.Parallel()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	provider, err := readinessload.NewConflictProvider(context.Background(), repo.Dir, policyconflict.Request{}, readinessload.JudgeRun, resolveConflictActors)
	if err != nil {
		t.Fatalf("NewConflictProvider: %v", err)
	}
	svc := asPolicyConflictService(t, provider)
	if svc.Deps.Actors != nil {
		t.Fatalf("Actors = %#v, want nil for a profile without a local-operator source", svc.Deps.Actors)
	}
}

// TestNewLocalContextConflictProvider_NotAdopted_NilActorsNoError proves
// the factory itself never fails on a not-yet-adopted store: Evaluate's
// own adoption probe must remain the sole place that reports
// NotAdoptedError (exit 1), never preempted here by an operational
// failure (exit 2) that would regress TestContextConflictBuiltBinary's
// "absent constitution is typed exit one" real-binary proof.
func TestNewLocalContextConflictProvider_NotAdopted_NilActorsNoError(t *testing.T) {
	t.Parallel()
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
	}, Message: "no policy store at all"}})
	provider, err := readinessload.NewConflictProvider(context.Background(), repo.Dir, policyconflict.Request{}, readinessload.JudgeRun, resolveConflictActors)
	if err != nil {
		t.Fatalf("NewConflictProvider: %v, want no error for a not-yet-adopted store", err)
	}
	svc := asPolicyConflictService(t, provider)
	if svc.Deps.Actors != nil {
		t.Fatalf("Actors = %#v, want nil for a not-yet-adopted store", svc.Deps.Actors)
	}
}

// TestNewLocalContextConflictProvider_LocalOperatorSource_ResolvesActor
// proves the positive wiring half: a selected profile declaring a
// local-operator source produces exactly the PrincipalResolution
// resolveLocalActors would produce standalone, through the same factory
// every lifecycle surface shares.
func TestNewLocalContextConflictProvider_LocalOperatorSource_ResolvesActor(t *testing.T) {
	t.Parallel()
	repo := buildLocalOperatorWiringRepo(t)
	// fixturegit.Build always configures this exact repo-local identity.
	provider, err := readinessload.NewConflictProvider(context.Background(), repo.Dir, policyconflict.Request{}, readinessload.JudgeRun, resolveConflictActors)
	if err != nil {
		t.Fatalf("NewConflictProvider: %v", err)
	}
	svc := asPolicyConflictService(t, provider)
	if len(svc.Deps.Actors) != 1 {
		t.Fatalf("Actors = %#v, want exactly one (fixturegit's identity is not role-mapped, so this is a violated resolution, not zero actors)", svc.Deps.Actors)
	}
	if svc.Deps.Actors[0].Claim.TrustSource != "local" {
		t.Fatalf("actor trust source = %q, want %q", svc.Deps.Actors[0].Claim.TrustSource, "local")
	}
	if svc.Deps.Actors[0].State != governanceprincipal.ResolutionViolated {
		t.Fatalf("state = %q, want violated-with-witness (fixturegit's identity is fixture@verdi.invalid, not the bound wiring-fixture@example.com)", svc.Deps.Actors[0].State)
	}
}

// TestContextConflict_NoLocalOperatorSource_ByteIdenticalReport is the
// CLI-level golden proof design §2.2 requires: for a profile without a
// local-operator source, the real production factory's report is
// byte-identical to a service built with the exact ServiceDeps this
// package hardcoded before this wiring existed (Actors: nil), everything
// else equal.
func TestContextConflict_NoLocalOperatorSource_ByteIdenticalReport(t *testing.T) {
	t.Parallel()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	request, err := contextcompile.DecodeRequest(contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	if err != nil {
		t.Fatalf("DecodeRequest fixture: %v", err)
	}
	conflictRequest := policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{Kind: policyconflict.TargetAcceptedContext, AcceptedContext: &request},
	}

	provider, err := readinessload.NewConflictProvider(context.Background(), repo.Dir, conflictRequest, readinessload.JudgeRun, resolveConflictActors)
	if err != nil {
		t.Fatalf("NewConflictProvider: %v", err)
	}
	// The comparison below is only valid between LIKE services: confirm the
	// wired factory actually left Primary nil for this fixture (no
	// manifest.align.judge_cmd declared), so a future fixture that DOES
	// declare one fails loudly here instead of silently comparing a
	// judge-carrying service against the judge-less baseline.
	if svc := asPolicyConflictService(t, provider); svc.Deps.Primary != nil {
		t.Fatalf("Deps.Primary = %#v, want nil for this fixture (no manifest.align.judge_cmd)", svc.Deps.Primary)
	}
	got, err := provider.Evaluate(context.Background(), conflictRequest)
	if err != nil {
		t.Fatalf("Evaluate (wired factory): %v", err)
	}

	// The same wired factory, but with an explicit nil actors resolver
	// (rather than resolveConflictActors) — the exact ServiceDeps.Actors
	// this package hardcoded before local-operator wiring existed. Every
	// other ServiceDeps field (compiler, ref resolver, tree hasher, date
	// source) is the SAME production construction readinessload.
	// NewConflictProvider used for the wired call above, so this
	// comparison isolates the Actors axis alone rather than also
	// re-implementing those adapters by hand here.
	baselineProvider, err := readinessload.NewConflictProvider(context.Background(), repo.Dir, conflictRequest, readinessload.JudgeRun, nil)
	if err != nil {
		t.Fatalf("NewConflictProvider (nil actors baseline): %v", err)
	}
	want, err := baselineProvider.Evaluate(context.Background(), conflictRequest)
	if err != nil {
		t.Fatalf("Evaluate (Actors: nil baseline): %v", err)
	}

	if !bytes.Equal(got.ReportBytes, want.ReportBytes) {
		t.Fatalf("report bytes differ for a profile without a local-operator source\nwired=%s\nbaseline=%s", got.ReportBytes, want.ReportBytes)
	}
}

// TestContextConflict_LocalOperatorSource_PassWithDisclosure and its
// sibling violated/unproven/build-start proofs live in
// context_conflict_localoperator_e2e_test.go, against the committed
// hermetic fixture under testdata/localoperator/.
