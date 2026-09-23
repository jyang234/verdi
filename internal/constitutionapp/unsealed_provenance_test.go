package constitutionapp

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/unsealedprovenance"
)

const (
	acceptedCutoff = "0123456789abcdef0123456789abcdef01234567"
	otherCutoff    = "fedcba9876543210fedcba9876543210fedcba98"

	unsealedProvenancePolicyRel = "policies/unsealed-provenance.md"
)

// unsealedProvenancePolicy renders the one policy carrying the
// unsealed-provenance payload; an empty cutoff omits the cutoff record.
func unsealedProvenancePolicy(capacity int, cutoff string) string {
	cutoffLine := ""
	if cutoff != "" {
		cutoffLine = "    cutoff: {commit: " + cutoff + "}\n"
	}
	return `---
schema: verdi.policy/v1
id: policy/unsealed-provenance
kind: policy
title: "Unsealed-provenance exemption authority"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads:
  unsealed-provenance:
    permitted: true
    cap: ` + strconv.Itoa(capacity) + `
    inventory:
      - {id: vatc-machine-projections, story: spec/vatc-machine-projections, admitted_by: "unsealed-provenance exemption ratification (SI-244)"}
` + cutoffLine + `template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Permits the pilot unsealed-provenance exemption and records the cutoff.
`
}

// storeSourceWith returns the shared fixture store plus, when policy is
// non-empty, the unsealed-provenance policy.
func storeSourceWith(t *testing.T, policy string) fstest.MapFS {
	t.Helper()
	source := fstest.MapFS{}
	for rel, data := range storeFixtureFiles(t) {
		source[rel] = &fstest.MapFile{Data: []byte(data), Mode: 0o644}
	}
	if policy != "" {
		source[".verdi/policy/"+unsealedProvenancePolicyRel] = &fstest.MapFile{Data: []byte(policy), Mode: 0o644}
	}
	return source
}

func unsealedSnapshot(t *testing.T, policy string) Snapshot {
	t.Helper()
	snapshot, typed := loadSnapshot(policyauthorityStore{}, storeSourceWith(t, policy), "fixture", "corrupted-policy")
	if typed != nil {
		t.Fatalf("loadSnapshot: %v", typed)
	}
	return snapshot
}

func unsealedEffective(t *testing.T, policy string) *policyauthority.EffectivePolicy {
	t.Helper()
	return unsealedSnapshot(t, policy).Effective
}

func TestUnsealedProvenanceCutoffReason(t *testing.T) {
	withCutoff := unsealedSnapshot(t, unsealedProvenancePolicy(1, acceptedCutoff))
	withCapChange := unsealedSnapshot(t, unsealedProvenancePolicy(2, acceptedCutoff))
	withOther := unsealedSnapshot(t, unsealedProvenancePolicy(1, otherCutoff))
	withoutCutoff := unsealedSnapshot(t, unsealedProvenancePolicy(1, ""))
	withoutPayload := unsealedSnapshot(t, "")
	notAdopted := Snapshot{Ref: "fixture", Adopted: false, Reason: "not adopted"}
	unavailable := Snapshot{Ref: "fixture", Reason: "exact Git tree bytes are unavailable", unavailable: true}

	tests := []struct {
		name     string
		accepted Snapshot
		proposed Snapshot
		want     []string // nil means not blocked by this rule
	}{
		{"kept", withCutoff, withCutoff, nil},
		{"kept while the cap changes", withCutoff, withCapChange, nil},
		{"added to a payload without one", withoutCutoff, withCutoff, nil},
		{"added with a new payload", withoutPayload, withCutoff, nil},
		{"accepted not adopted", notAdopted, withCutoff, nil},
		{"removed from the payload", withCutoff, withoutCutoff, []string{unsealedprovenance.CutoffChangedCode + ": ", acceptedCutoff, "records none"}},
		{"removed with the payload", withCutoff, withoutPayload, []string{unsealedprovenance.CutoffChangedCode + ": ", acceptedCutoff, "records none"}},
		{"removed with the whole constitution", withCutoff, notAdopted, []string{unsealedprovenance.CutoffChangedCode + ": ", acceptedCutoff, "records none"}},
		{"replaced by another commit", withCutoff, withOther, []string{unsealedprovenance.CutoffChangedCode + ": ", acceptedCutoff, otherCutoff}},
		// An unreadable exact tree proves nothing either way; impact coverage
		// already discloses and blocks it (accepted-/proposed-tree-unavailable).
		{"accepted tree unavailable", unavailable, withoutCutoff, nil},
		{"proposed tree unavailable", withCutoff, unavailable, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, typed := unsealedProvenanceCutoffReason(tc.accepted, tc.proposed)
			if typed != nil {
				t.Fatalf("unsealedProvenanceCutoffReason: %v", typed)
			}
			if tc.want == nil {
				if got != "" {
					t.Fatalf("reason = %q, want not blocked by this rule", got)
				}
				return
			}
			if !strings.HasPrefix(got, tc.want[0]) {
				t.Fatalf("reason = %q, want it to start with %q", got, tc.want[0])
			}
			for _, fragment := range tc.want[1:] {
				if !strings.Contains(got, fragment) {
					t.Fatalf("reason = %q, want it to contain %q", got, fragment)
				}
			}
		})
	}
}

// TestUnsealedProvenanceCutoffReason_RefusesUnreadablePayload covers the
// defensive paths: an adopted snapshot with no effective policy, and an
// effective policy whose payload cannot be read, are corrupted authority —
// never read as "no cutoff".
func TestUnsealedProvenanceCutoffReason_RefusesUnreadablePayload(t *testing.T) {
	withCutoff := unsealedSnapshot(t, unsealedProvenancePolicy(1, acceptedCutoff))
	payload, _, err := unsealedprovenance.Effective(withCutoff.Effective)
	if err != nil {
		t.Fatal(err)
	}
	twoCarriers := Snapshot{Ref: "fixture", Adopted: true, Effective: &policyauthority.EffectivePolicy{Policies: []policyauthority.EffectivePolicyEntry{
		{PolicyID: "policy/a", Payloads: map[string]policyartifact.Payload{unsealedprovenance.PayloadKind: payload}},
		{PolicyID: "policy/b", Payloads: map[string]policyartifact.Payload{unsealedprovenance.PayloadKind: payload}},
	}}}
	adoptedWithoutEffective := Snapshot{Ref: "fixture", Adopted: true}

	tests := []struct {
		name     string
		accepted Snapshot
		proposed Snapshot
		want     string
	}{
		{"accepted adopted without an effective policy", adoptedWithoutEffective, withCutoff, "carries no effective policy"},
		{"proposed adopted without an effective policy", withCutoff, adoptedWithoutEffective, "carries no effective policy"},
		{"accepted payload unreadable", twoCarriers, withCutoff, "carried by both policy"},
		{"proposed payload unreadable", withCutoff, twoCarriers, "carried by both policy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, typed := unsealedProvenanceCutoffReason(tc.accepted, tc.proposed)
			if typed == nil {
				t.Fatalf("reason = %q, want a typed corrupted-policy refusal", got)
			}
			if typed.Classification != ClassificationVerdict || typed.Code != "corrupted-policy" || !strings.Contains(typed.Error(), tc.want) {
				t.Fatalf("refusal = %+v (%v), want verdict corrupted-policy containing %q", typed, typed, tc.want)
			}
		})
	}
}

// effectiveSwappingAuthority is the real policyauthority port except that
// the nth Resolve call returns a preconfigured effective policy. On a
// checkout whose HEAD is the accepted head, SubmitPreparation resolves
// exactly three times — 1: the worktree validation, 2: the proposed exact
// tree, 3: the accepted exact tree — while the source layers still come from
// the real, unchanged store. That isolates the cutoff rule as the only thing
// that can move readiness: no layer delta exists, so coverage is proven.
type effectiveSwappingAuthority struct {
	AuthorityStore
	calls     *int
	effective map[int]*policyauthority.EffectivePolicy
}

func (a effectiveSwappingAuthority) Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error) {
	*a.calls++
	if effective, ok := a.effective[*a.calls]; ok {
		return effective, nil
	}
	return a.AuthorityStore.Resolve(store)
}

func TestSubmitPreparation_CutoffRuleAloneDecidesReadiness(t *testing.T) {
	kept := func(t *testing.T) *policyauthority.EffectivePolicy {
		return unsealedEffective(t, unsealedProvenancePolicy(1, acceptedCutoff))
	}
	tests := []struct {
		name      string
		accepted  func(*testing.T) *policyauthority.EffectivePolicy // nil: the real store, no payload
		proposed  func(*testing.T) *policyauthority.EffectivePolicy // nil: the real store, no payload
		wantReady bool
	}{
		{"kept", kept, kept, true},
		{"removed", kept, nil, false},
		{"replaced", kept, func(t *testing.T) *policyauthority.EffectivePolicy {
			return unsealedEffective(t, unsealedProvenancePolicy(1, otherCutoff))
		}, false},
		{"added where none existed", nil, kept, true},
		{"added to a payload without one", func(t *testing.T) *policyauthority.EffectivePolicy {
			return unsealedEffective(t, unsealedProvenancePolicy(1, ""))
		}, kept, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := buildFixtureRepo(t)
			swapped := map[int]*policyauthority.EffectivePolicy{}
			if tc.proposed != nil {
				proposed := tc.proposed(t)
				swapped[1], swapped[2] = proposed, proposed
			}
			if tc.accepted != nil {
				swapped[3] = tc.accepted(t)
			}
			calls := 0
			svc := testService()
			svc.Authority = effectiveSwappingAuthority{AuthorityStore: svc.Authority, calls: &calls, effective: swapped}

			prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
			if typed != nil {
				t.Fatalf("SubmitPreparation: %v", typed)
			}
			if calls != 3 {
				t.Fatalf("Resolve was called %d times, want exactly 3 (validation, proposed, accepted)", calls)
			}
			if prep.ImpactReview.Coverage.State != constitutionimpact.StateProven || len(prep.ImpactReview.Layers) != 0 {
				t.Fatalf("test setup: coverage %q with layers %v, want proven with no delta", prep.ImpactReview.Coverage.State, prep.ImpactReview.Layers)
			}
			if prep.ReadyForSubmission != tc.wantReady {
				t.Fatalf("ReadyForSubmission = %t, want %t (blocking reasons %v)", prep.ReadyForSubmission, tc.wantReady, prep.BlockingReasons)
			}
			if tc.wantReady {
				if len(prep.BlockingReasons) != 0 {
					t.Fatalf("ready packet carries blocking reasons %v", prep.BlockingReasons)
				}
				return
			}
			if len(prep.BlockingReasons) != 1 || !strings.HasPrefix(prep.BlockingReasons[0], unsealedprovenance.CutoffChangedCode+": ") {
				t.Fatalf("blocking reasons = %v, want exactly the %s reason", prep.BlockingReasons, unsealedprovenance.CutoffChangedCode)
			}
		})
	}
}

// TestSubmitPreparation_UnreadableCutoffPayloadRefuses proves a payload the
// rule cannot read surfaces from SubmitPreparation as a typed
// corrupted-policy refusal instead of a packet that reads "no cutoff".
func TestSubmitPreparation_UnreadableCutoffPayloadRefuses(t *testing.T) {
	payload, _, err := unsealedprovenance.Effective(unsealedEffective(t, unsealedProvenancePolicy(1, acceptedCutoff)))
	if err != nil {
		t.Fatal(err)
	}
	twoCarriers := &policyauthority.EffectivePolicy{Policies: []policyauthority.EffectivePolicyEntry{
		{PolicyID: "policy/a", Payloads: map[string]policyartifact.Payload{unsealedprovenance.PayloadKind: payload}},
		{PolicyID: "policy/b", Payloads: map[string]policyartifact.Payload{unsealedprovenance.PayloadKind: payload}},
	}}
	for _, call := range []int{2, 3} {
		t.Run("resolve call "+strconv.Itoa(call), func(t *testing.T) {
			root := buildFixtureRepo(t)
			calls := 0
			svc := testService()
			swapped := map[int]*policyauthority.EffectivePolicy{call: twoCarriers}
			if call == 2 {
				// Validation and the proposed exact tree must agree, or the
				// packet refuses earlier as identity-shifted.
				swapped[1] = twoCarriers
			}
			svc.Authority = effectiveSwappingAuthority{AuthorityStore: svc.Authority, calls: &calls, effective: swapped}

			prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
			if typed == nil {
				t.Fatalf("SubmitPreparation returned a packet (ready = %t, reasons %v) over an unreadable payload", prep.ReadyForSubmission, prep.BlockingReasons)
			}
			if typed.Classification != ClassificationVerdict || typed.Code != "corrupted-policy" || !strings.Contains(typed.Error(), "carried by both policy") {
				t.Fatalf("refusal = %v, want verdict corrupted-policy naming the duplicate carriers", typed)
			}
		})
	}
}

// buildUnsealedProvenanceRepo builds the shared fixture store plus the
// unsealed-provenance policy as the accepted default-branch state.
func buildUnsealedProvenanceRepo(t *testing.T, policy string) string {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	files := storeFixtureFiles(t)
	files[".verdi/policy/"+unsealedProvenancePolicyRel] = policy
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt constitution with unsealed-provenance payload"}})
	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

func proposeUnsealedProvenancePolicy(t *testing.T, root string, svc Service, branch, content string) {
	t.Helper()
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindPolicy, Name: "unsealed-provenance",
		Content: []byte(content), Expected: Expected{Branch: branch},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
}

func cutoffReasons(reasons []string) []string {
	var out []string
	for _, reason := range reasons {
		if strings.Contains(reason, unsealedprovenance.CutoffChangedCode) {
			out = append(out, reason)
		}
	}
	return out
}

// TestSubmitPreparation_CutoffImmutabilityThroughProposals drives the real
// governed change path: the accepted default branch carries the payload, a
// proposal branch is committed through Propose, and SubmitPreparation reads
// both exact Git trees.
func TestSubmitPreparation_CutoffImmutabilityThroughProposals(t *testing.T) {
	tests := []struct {
		name        string
		accepted    string
		proposed    string // empty: no proposal; HEAD stays on the accepted head
		wantCutoff  []string
		wantReady   bool
		wantReasons int
	}{
		{"kept with no change stays ready", unsealedProvenancePolicy(1, acceptedCutoff), "", nil, true, 0},
		{"kept while the cap changes", unsealedProvenancePolicy(1, acceptedCutoff), unsealedProvenancePolicy(2, acceptedCutoff), nil, false, 1},
		{"added where none existed", unsealedProvenancePolicy(1, ""), unsealedProvenancePolicy(1, acceptedCutoff), nil, false, 1},
		{"removed", unsealedProvenancePolicy(1, acceptedCutoff), unsealedProvenancePolicy(1, ""), []string{acceptedCutoff, "records none"}, false, 2},
		{"replaced by another commit", unsealedProvenancePolicy(1, acceptedCutoff), unsealedProvenancePolicy(1, otherCutoff), []string{acceptedCutoff, otherCutoff}, false, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := buildUnsealedProvenanceRepo(t, tc.accepted)
			svc := testService()
			if tc.proposed != "" {
				proposeUnsealedProvenancePolicy(t, root, svc, "policy/unsealed-provenance-cutoff", tc.proposed)
			}

			prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
			if typed != nil {
				t.Fatalf("SubmitPreparation: %v", typed)
			}
			if prep.ReadyForSubmission != tc.wantReady || len(prep.BlockingReasons) != tc.wantReasons {
				t.Fatalf("ready = %t with reasons %v, want ready = %t with %d reasons", prep.ReadyForSubmission, prep.BlockingReasons, tc.wantReady, tc.wantReasons)
			}
			got := cutoffReasons(prep.BlockingReasons)
			if tc.wantCutoff == nil {
				if len(got) != 0 {
					t.Fatalf("unexpected cutoff blocking reasons %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.HasPrefix(got[0], unsealedprovenance.CutoffChangedCode+": ") {
				t.Fatalf("cutoff blocking reasons = %v, want exactly one starting with the code", got)
			}
			for _, fragment := range tc.wantCutoff {
				if !strings.Contains(got[0], fragment) {
					t.Fatalf("cutoff reason %q does not name %q", got[0], fragment)
				}
			}
		})
	}
}

// TestSubmitPreparation_CutoffRuleKeepsUnavailableAcceptedTreeBlocking proves
// an unreadable accepted tree keeps today's behavior: disclosed and blocking
// through impact coverage, with no cutoff claim invented from bytes that
// were never read.
func TestSubmitPreparation_CutoffRuleKeepsUnavailableAcceptedTreeBlocking(t *testing.T) {
	root := buildUnsealedProvenanceRepo(t, unsealedProvenancePolicy(1, acceptedCutoff))
	acceptedHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	svc := testService()
	proposeUnsealedProvenancePolicy(t, root, svc, "policy/unsealed-provenance-unavailable", unsealedProvenancePolicy(1, ""))
	svc.Git = unavailableExactTreeGitReader{
		GitReader: svc.Git, ref: acceptedHead,
		unavailable:    errors.New("accepted exact tree deliberately unavailable"),
		materializeErr: errors.New("an unavailable tree must not be materialized"),
	}

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
	if typed != nil {
		t.Fatalf("SubmitPreparation classified accepted-tree unavailability as a refusal: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("ready over an unreadable accepted tree")
	}
	if prep.ImpactReview.Coverage.State != constitutionimpact.StateDisclosedUnproven ||
		!hasImpactReason(prep.ImpactReview.Coverage, constitutionimpact.ReasonAcceptedTreeUnavailable) {
		t.Fatalf("accepted-tree unavailability not disclosed: %+v", prep.ImpactReview.Coverage)
	}
	if got := cutoffReasons(prep.BlockingReasons); len(got) != 0 {
		t.Fatalf("cutoff rule claimed a change it could not read: %v", got)
	}
}
