package countersign

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/forge"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

const (
	testSHA       = "0123456789abcdef0123456789abcdef01234567"
	otherSHA      = "89abcdef0123456789abcdef0123456789abcdef"
	testDigest    = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	evaluatedAt   = "2026-08-25T12:00:00Z"
	observedAt    = "2026-08-25T11:59:00Z"
	freshApproved = "2026-08-25T11:58:00Z"
)

type factReader struct {
	states map[string]gp.ResolutionState
	errors map[string]error
}

func (r factReader) ReadTrustFact(_ context.Context, source gp.TrustSource, claim gp.PrincipalClaim) (gp.TrustFact, error) {
	if err := r.errors[claim.Subject]; err != nil {
		return gp.TrustFact{}, err
	}
	state := r.states[claim.Subject]
	if state == "" {
		state = gp.ResolutionAuthenticated
	}
	switch state {
	case gp.ResolutionAuthenticated:
		return gp.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Subjects: []string{claim.Subject}, EvidenceDigest: testDigest, Available: true, Valid: true}, nil
	case gp.ResolutionViolated:
		return gp.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Subjects: []string{"different"}, EvidenceDigest: testDigest, Available: true, Valid: true}, nil
	case gp.ResolutionUnproven:
		return gp.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Reason: "forge identity unavailable"}, nil
	default:
		return gp.TrustFact{}, errors.New("test reader received unknown state")
	}
}

func testProfile(t *testing.T, trustKind gp.TrustSourceKind) gp.Profile {
	t.Helper()
	raw := []byte(`schema: verdi.governance-profile/v1
id: countersign-team
class: team
applicable_transitions: [accept, close]
identity_trust_sources:
  - {id: forge-live, kind: ` + string(trustKind) + `}
role_mappings:
  - {role: story-review, trust_source: forge-live, subjects: ["101", "202", "303"]}
  - {role: feature-uat, trust_source: forge-live, subjects: ["202", "303"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [accept], roles: [story-review], minimum: 1}
  - {transitions: [close], roles: [feature-uat], minimum: 1}
distinctness_rules:
  - {transitions: [accept, close], left_role: story-review, right_role: feature-uat, relation: different-principal}
evidence_source_restrictions: []
escalation_thresholds: []
`)
	catalog := gp.Catalog{Roles: []string{"story-review", "feature-uat"}, Transitions: []string{"accept", "close"}}
	profile, err := gp.DecodeProfile(raw, catalog)
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return profile
}

func approval(id, subject string) forge.Approval {
	return forge.Approval{
		ApprovalID: id, ApprovalRef: "review/" + id, State: forge.ApprovalActive,
		ApprovedAt: freshApproved, UpdatedAt: freshApproved, CandidateSHA: testSHA,
		Actor:             forge.ProviderActor{Scheme: "github-user-id", Subject: subject},
		ProviderWitnesses: []forge.ProviderWitness{{Name: "review_id", Value: id}},
	}
}

func testRequest(t *testing.T, rows []forge.Approval, states map[string]gp.ResolutionState) Request {
	t.Helper()
	snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "42", testSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: "900"}, mustTime(t, observedAt), rows)
	if err != nil {
		t.Fatalf("NewApprovalSnapshot: %v", err)
	}
	policy, err := NewFreshnessPolicy("forge-live-policy", 300, 3600)
	if err != nil {
		t.Fatalf("NewFreshnessPolicy: %v", err)
	}
	return Request{
		Snapshot: snapshot, LocalCandidateSHA: testSHA, Profile: testProfile(t, gp.TrustSourceForge),
		TrustSourceID:   "forge-live",
		Obligation:      Obligation{Transition: "accept", Scheme: SchemeAttestation, Kind: KindCountersign, Role: "story-review", RequiredCount: 1, SeparationRule: SeparationNone},
		FreshnessPolicy: policy, EvaluatedAt: evaluatedAt,
		Resolver: gp.NewResolver(factReader{states: states}),
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	got, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func authorResolution(t *testing.T, req Request, subject string, state gp.ResolutionState) gp.PrincipalResolution {
	t.Helper()
	resolver := gp.NewResolver(factReader{states: map[string]gp.ResolutionState{subject: state}})
	got, err := resolver.Resolve(context.Background(), req.Profile, gp.PrincipalClaim{TrustSource: req.TrustSourceID, Subject: subject})
	if err != nil {
		t.Fatalf("resolve author: %v", err)
	}
	return got
}

func resolveRecord(t *testing.T, req Request) Record {
	t.Helper()
	record, err := Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return record
}

func TestCountersignWitnessContract_Static(t *testing.T) {
	t.Parallel()

	req := testRequest(t, []forge.Approval{approval("a-review", "202"), approval("z-review", "101")}, nil)
	req.Obligation.RequiredCount = 2
	record := resolveRecord(t, req)
	if record.Schema != SchemaID || record.Verdict != VerdictProven {
		t.Fatalf("record identity/verdict = %q/%q", record.Schema, record.Verdict)
	}
	if len(record.Approvals) != 2 || record.Approvals[0].PrincipalResolution.Claim.Subject != "101" || record.Approvals[1].PrincipalResolution.Claim.Subject != "202" {
		t.Fatalf("approvals not in canonical principal/id order: %+v", record.Approvals)
	}
	if record.Obligation.GovernanceProfileID != req.Profile.ID || record.Obligation.GovernanceProfileDigest == "" || record.Freshness.PolicyDigest != req.FreshnessPolicy.Digest {
		t.Fatalf("sealed input bindings missing: %+v %+v", record.Obligation, record.Freshness)
	}
	if record.Reduction.EligibleCount != 2 || len(record.Reduction.EligibleApprovalIDs) != 2 || len(record.Reduction.DistinctPrincipalIDs) != 2 {
		t.Fatalf("reduction = %+v", record.Reduction)
	}
	if !reflect.DeepEqual(record.Reduction.EligibleApprovalIDs, []string{"z-review", "a-review"}) {
		t.Fatalf("eligible approval ids = %v, want canonical approval-row order", record.Reduction.EligibleApprovalIDs)
	}
	if record.Approvals == nil || record.Reduction.EligibleApprovalIDs == nil || record.Reduction.DistinctPrincipalIDs == nil || record.Witnesses == nil {
		t.Fatal("record contains a null collection")
	}
	for _, row := range record.Approvals {
		if row.ProviderWitnesses == nil || row.PrincipalResolution.Witnesses == nil {
			t.Fatal("approval contains a null witness collection")
		}
	}

	encoded, err := EncodeRecord(record)
	if err != nil {
		t.Fatalf("EncodeRecord: %v", err)
	}
	if !bytes.HasSuffix(encoded, []byte("\n")) || bytes.HasSuffix(encoded, []byte("\n\n")) {
		t.Fatalf("canonical encoding must have exactly one trailing newline: %q", encoded)
	}
	decoded, err := DecodeRecord(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	if !reflect.DeepEqual(decoded, record) {
		t.Fatalf("round trip differs\n got: %#v\nwant: %#v", decoded, record)
	}

	var tree map[string]any
	if err := json.Unmarshal(encoded, &tree); err != nil {
		t.Fatal(err)
	}
	wantTop := []string{"approvals", "candidate_sha", "change_id", "digest", "forge", "freshness", "obligation", "reduction", "repository", "schema", "verdict", "witnesses"}
	assertExactKeys(t, tree, wantTop)
	assertExactKeys(t, tree["obligation"].(map[string]any), []string{"governance_profile_digest", "governance_profile_id", "kind", "required_count", "role", "scheme", "separation_rule", "transition"})
	assertExactKeys(t, tree["freshness"].(map[string]any), []string{"evaluated_at", "maximum_approval_age_seconds", "maximum_observation_age_seconds", "observed_at", "policy_digest", "policy_id", "provider_snapshot_id"})
	assertExactKeys(t, tree["reduction"].(map[string]any), []string{"distinct_principal_ids", "eligible_approval_ids", "eligible_count", "freshness_verdict", "required_count", "separation_verdict"})
	assertExactKeys(t, tree["approvals"].([]any)[0].(map[string]any), []string{"approval_id", "approval_ref", "approved_at", "candidate_sha", "principal_resolution", "provider_witnesses", "state", "updated_at"})

	semanticTwin := testRequest(t, []forge.Approval{approval("z-review", "101"), approval("a-review", "202")}, nil)
	semanticTwin.Obligation.RequiredCount = 2
	twinBytes, err := EncodeRecord(resolveRecord(t, semanticTwin))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, twinBytes) {
		t.Fatalf("semantic equivalents differ\nfirst: %s\nsecond: %s", encoded, twinBytes)
	}

	t.Run("strict decode failures", func(t *testing.T) {
		mutations := map[string]func(map[string]any){
			"unknown":                 func(doc map[string]any) { doc["surprise"] = true },
			"missing":                 func(doc map[string]any) { delete(doc, "forge") },
			"null approvals":          func(doc map[string]any) { doc["approvals"] = nil },
			"null top witnesses":      func(doc map[string]any) { doc["witnesses"] = nil },
			"null provider witnesses": func(doc map[string]any) { doc["approvals"].([]any)[0].(map[string]any)["provider_witnesses"] = nil },
			"null kernel witnesses": func(doc map[string]any) {
				doc["approvals"].([]any)[0].(map[string]any)["principal_resolution"].(map[string]any)["witnesses"] = nil
			},
			"unknown verdict":           func(doc map[string]any) { doc["verdict"] = "maybe" },
			"unknown freshness verdict": func(doc map[string]any) { doc["reduction"].(map[string]any)["freshness_verdict"] = "maybe" },
			"unknown separation":        func(doc map[string]any) { doc["obligation"].(map[string]any)["separation_rule"] = "same-as-author" },
		}
		for name, mutate := range mutations {
			t.Run(name, func(t *testing.T) {
				var doc map[string]any
				if err := json.Unmarshal(encoded, &doc); err != nil {
					t.Fatal(err)
				}
				mutate(doc)
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRecord(bytes.NewReader(raw)); err == nil {
					t.Fatal("DecodeRecord unexpectedly accepted invalid document")
				}
			})
		}
		if _, err := DecodeRecord(bytes.NewReader(append(append([]byte(nil), encoded...), []byte(`{}`)...))); err == nil {
			t.Fatal("DecodeRecord accepted trailing data")
		}
		if _, err := DecodeRecord(bytes.NewReader(bytes.TrimSuffix(encoded, []byte("\n")))); err == nil {
			t.Fatal("DecodeRecord accepted byte-noncanonical input")
		}
	})

	t.Run("digest tamper and recompute", func(t *testing.T) {
		tampered := record
		tampered.Repository = "acme/other"
		if _, err := EncodeRecord(tampered); err == nil {
			t.Fatal("EncodeRecord accepted stale digest")
		}
		digest, err := recordDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		tampered.Digest = digest
		if _, err := EncodeRecord(tampered); err != nil {
			t.Fatalf("EncodeRecord rejected recomputed digest: %v", err)
		}
	})

	t.Run("empty kernel witnesses cannot be redigested into validity", func(t *testing.T) {
		tampered := record
		tampered.Approvals = append([]ApprovalRecord(nil), record.Approvals...)
		tampered.Approvals[0].PrincipalResolution.Witnesses = []gp.Witness{}
		digest, err := recordDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		tampered.Digest = digest
		if _, err := EncodeRecord(tampered); err == nil {
			t.Fatal("EncodeRecord accepted a resolution without kernel witnesses")
		}
	})

	t.Run("cross-field contradictions cannot be redigested into validity", func(t *testing.T) {
		tests := map[string]func(Record) Record{
			"stale eligible approval": func(got Record) Record {
				got.Approvals = append([]ApprovalRecord(nil), got.Approvals...)
				got.Approvals[0].ApprovedAt = "2020-01-01T00:00:00Z"
				return got
			},
			"non-proven verdict over proven reduction": func(got Record) Record {
				got.Verdict = VerdictViolated
				return got
			},
		}
		for name, mutate := range tests {
			t.Run(name, func(t *testing.T) {
				tampered := mutate(record)
				digest, err := recordDigest(tampered)
				if err != nil {
					t.Fatal(err)
				}
				tampered.Digest = digest
				if _, err := EncodeRecord(tampered); err == nil {
					t.Fatal("EncodeRecord accepted a cross-field contradiction")
				}
			})
		}
	})

	t.Run("retained adverse eligibility witnesses cannot be redigested into proof", func(t *testing.T) {
		tests := map[string]func(*testing.T) Record{
			"role refused": func(t *testing.T) Record {
				return resolveRecord(t, testRequest(t, []forge.Approval{approval("r1", "999")}, nil))
			},
			"self approved": func(t *testing.T) Record {
				req := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
				req.Obligation.SeparationRule = SeparationDifferentFromAuthor
				author := authorResolution(t, req, "101", gp.ResolutionAuthenticated)
				req.CandidateAuthor = &author
				return resolveRecord(t, req)
			},
		}
		for name, build := range tests {
			t.Run(name, func(t *testing.T) {
				falseGreen := build(t)
				row := falseGreen.Approvals[0]
				falseGreen.Reduction.EligibleApprovalIDs = []string{row.ApprovalID}
				falseGreen.Reduction.DistinctPrincipalIDs = []gp.PrincipalID{row.PrincipalResolution.PrincipalID}
				falseGreen.Reduction.EligibleCount = 1
				falseGreen.Reduction.FreshnessVerdict = VerdictProven
				falseGreen.Reduction.SeparationVerdict = VerdictProven
				falseGreen.Verdict = VerdictProven
				digest, err := recordDigest(falseGreen)
				if err != nil {
					t.Fatal(err)
				}
				falseGreen.Digest = digest

				if _, err := EncodeRecord(falseGreen); err == nil {
					t.Fatal("EncodeRecord accepted a redigested false-green witness")
				}
				raw, err := canonjson.Marshal(falseGreen)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRecord(bytes.NewReader(raw)); err == nil {
					t.Fatal("DecodeRecord accepted a canonical redigested false-green witness")
				}
			})
		}
	})

	t.Run("decoded noncanonical order is rejected", func(t *testing.T) {
		var doc map[string]any
		if err := json.Unmarshal(encoded, &doc); err != nil {
			t.Fatal(err)
		}
		rows := doc["approvals"].([]any)
		rows[0], rows[1] = rows[1], rows[0]
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRecord(bytes.NewReader(raw)); err == nil {
			t.Fatal("DecodeRecord accepted noncanonical approval order")
		}
	})
}

func TestCountersignWitnessContract_Behavioral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		build      func(*testing.T) Request
		want       Verdict
		count      int
		freshness  Verdict
		separation Verdict
	}{
		{"story review", func(t *testing.T) Request { return testRequest(t, []forge.Approval{approval("r1", "101")}, nil) }, VerdictProven, 1, VerdictProven, VerdictProven},
		{"feature uat", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "202")}, nil)
			r.Obligation.Transition = "close"
			r.Obligation.Role = "feature-uat"
			return r
		}, VerdictProven, 1, VerdictProven, VerdictProven},
		{"multi principal", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101"), approval("r2", "202")}, nil)
			r.Obligation.RequiredCount = 2
			return r
		}, VerdictProven, 2, VerdictProven, VerdictProven},
		{"duplicate principal", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101"), approval("r2", "101")}, nil)
			r.Obligation.RequiredCount = 2
			return r
		}, VerdictViolated, 1, VerdictProven, VerdictProven},
		{"violated resolution", func(t *testing.T) Request {
			return testRequest(t, []forge.Approval{approval("r1", "101")}, map[string]gp.ResolutionState{"101": gp.ResolutionViolated})
		}, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"unproven resolution", func(t *testing.T) Request {
			return testRequest(t, []forge.Approval{approval("r1", "101")}, map[string]gp.ResolutionState{"101": gp.ResolutionUnproven})
		}, VerdictUnproven, 0, VerdictProven, VerdictProven},
		{"violated resolution with missing author", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, map[string]gp.ResolutionState{"101": gp.ResolutionViolated})
			r.Obligation.SeparationRule = SeparationDifferentFromAuthor
			return r
		}, VerdictViolated, 0, VerdictProven, VerdictUnproven},
		{"violated separation takes priority over unproven resolution", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, map[string]gp.ResolutionState{"101": gp.ResolutionUnproven})
			r.Obligation.SeparationRule = SeparationDifferentFromAuthor
			a := authorResolution(t, r, "999", gp.ResolutionViolated)
			r.CandidateAuthor = &a
			return r
		}, VerdictViolated, 0, VerdictProven, VerdictViolated},
		{"role refusal", func(t *testing.T) Request { return testRequest(t, []forge.Approval{approval("r1", "999")}, nil) }, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"revoked", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.State = forge.ApprovalRevoked
			return testRequest(t, []forge.Approval{a}, nil)
		}, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"dismissed", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.State = forge.ApprovalDismissed
			return testRequest(t, []forge.Approval{a}, nil)
		}, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"wrong head", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.CandidateSHA = otherSHA
			return testRequest(t, []forge.Approval{a}, nil)
		}, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"stale approval", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.ApprovedAt = "2026-08-25T10:00:00Z"
			a.UpdatedAt = a.ApprovedAt
			r := testRequest(t, []forge.Approval{a}, nil)
			p, _ := NewFreshnessPolicy("forge-live-policy", 300, 180)
			r.FreshnessPolicy = p
			return r
		}, VerdictViolated, 0, VerdictViolated, VerdictProven},
		{"future approval", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.ApprovedAt = "2026-08-25T12:01:00Z"
			a.UpdatedAt = a.ApprovedAt
			return testRequest(t, []forge.Approval{a}, nil)
		}, VerdictViolated, 0, VerdictViolated, VerdictProven},
		{"future approval update", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.UpdatedAt = "2026-08-25T12:01:00Z"
			return testRequest(t, []forge.Approval{a}, nil)
		}, VerdictViolated, 0, VerdictViolated, VerdictProven},
		{"stale observation", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Snapshot.ObservedAt = "2026-08-25T10:00:00Z"
			return r
		}, VerdictViolated, 0, VerdictViolated, VerdictProven},
		{"future observation", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Snapshot.ObservedAt = "2026-08-25T12:01:00Z"
			return r
		}, VerdictViolated, 0, VerdictViolated, VerdictProven},
		{"self approved", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Obligation.SeparationRule = SeparationDifferentFromAuthor
			a := authorResolution(t, r, "101", gp.ResolutionAuthenticated)
			r.CandidateAuthor = &a
			return r
		}, VerdictViolated, 0, VerdictProven, VerdictViolated},
		{"different author", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Obligation.SeparationRule = SeparationDifferentFromAuthor
			a := authorResolution(t, r, "999", gp.ResolutionAuthenticated)
			r.CandidateAuthor = &a
			return r
		}, VerdictProven, 1, VerdictProven, VerdictProven},
		{"author unproven", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Obligation.SeparationRule = SeparationDifferentFromAuthor
			a := authorResolution(t, r, "999", gp.ResolutionUnproven)
			r.CandidateAuthor = &a
			return r
		}, VerdictUnproven, 0, VerdictProven, VerdictUnproven},
		{"author violated", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Obligation.SeparationRule = SeparationDifferentFromAuthor
			a := authorResolution(t, r, "999", gp.ResolutionViolated)
			r.CandidateAuthor = &a
			return r
		}, VerdictViolated, 0, VerdictProven, VerdictViolated},
		{"insufficient", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.Obligation.RequiredCount = 2
			return r
		}, VerdictViolated, 1, VerdictProven, VerdictProven},
		{"empty approvals", func(t *testing.T) Request { return testRequest(t, []forge.Approval{}, nil) }, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"snapshot mismatch", func(t *testing.T) Request {
			r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
			r.LocalCandidateSHA = otherSHA
			return r
		}, VerdictViolated, 0, VerdictProven, VerdictProven},
		{"zero approval ceiling", func(t *testing.T) Request {
			a := approval("r1", "101")
			a.ApprovedAt = "2020-01-01T00:00:00Z"
			a.UpdatedAt = a.ApprovedAt
			r := testRequest(t, []forge.Approval{a}, nil)
			p, _ := NewFreshnessPolicy("forge-live-policy", 300, 0)
			r.FreshnessPolicy = p
			return r
		}, VerdictProven, 1, VerdictProven, VerdictProven},
		{"stale rejected row does not poison proof", func(t *testing.T) Request {
			stale := approval("r0", "202")
			stale.ApprovedAt = "2020-01-01T00:00:00Z"
			stale.UpdatedAt = stale.ApprovedAt
			r := testRequest(t, []forge.Approval{stale, approval("r1", "101")}, nil)
			p, _ := NewFreshnessPolicy("forge-live-policy", 300, 180)
			r.FreshnessPolicy = p
			return r
		}, VerdictProven, 1, VerdictProven, VerdictProven},
		{"irrelevant unproven row does not poison proof", func(t *testing.T) Request {
			return testRequest(t, []forge.Approval{approval("r1", "101"), approval("r2", "202")}, map[string]gp.ResolutionState{"202": gp.ResolutionUnproven})
		}, VerdictProven, 1, VerdictProven, VerdictProven},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			record := resolveRecord(t, tc.build(t))
			if record.Verdict != tc.want || record.Reduction.EligibleCount != tc.count || record.Reduction.FreshnessVerdict != tc.freshness || record.Reduction.SeparationVerdict != tc.separation {
				t.Fatalf("verdict/reduction = %q/%+v, want %q count=%d freshness=%q separation=%q; witnesses=%v", record.Verdict, record.Reduction, tc.want, tc.count, tc.freshness, tc.separation, record.Witnesses)
			}
		})
	}

	t.Run("author resolution facts are preserved", func(t *testing.T) {
		r := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
		r.Obligation.SeparationRule = SeparationDifferentFromAuthor
		a := authorResolution(t, r, "999", gp.ResolutionAuthenticated)
		r.CandidateAuthor = &a
		record := resolveRecord(t, r)
		for _, prefix := range []string{"candidate-author-resolution:", "candidate-author-witness:"} {
			found := false
			for _, witness := range record.Witnesses {
				if strings.HasPrefix(witness, prefix) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("top-level witnesses lack %q fact: %v", prefix, record.Witnesses)
			}
		}
	})

	t.Run("missing author remains an unproven operand", func(t *testing.T) {
		req := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
		req.Obligation.SeparationRule = SeparationDifferentFromAuthor

		record, err := Resolve(context.Background(), req)
		if err != nil {
			t.Fatalf("Resolve returned an operational error for a missing author: %v", err)
		}
		if record.Verdict != VerdictUnproven || record.Reduction.SeparationVerdict != VerdictUnproven {
			t.Fatalf("verdict/separation = %q/%q, want unproven/unproven", record.Verdict, record.Reduction.SeparationVerdict)
		}
		if record.Reduction.EligibleCount != 0 || len(record.Reduction.EligibleApprovalIDs) != 0 || len(record.Reduction.DistinctPrincipalIDs) != 0 {
			t.Fatalf("missing author contributed eligible approvals: %+v", record.Reduction)
		}
		for _, want := range []string{
			`candidate-author-resolution:operand="missing":state="unproven"`,
			`approval-separation:approval_id="r1":state="unproven"`,
		} {
			found := false
			for _, witness := range record.Witnesses {
				if witness == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("witnesses lack %q: %v", want, record.Witnesses)
			}
		}
	})

	t.Run("operational input and port failures", func(t *testing.T) {
		base := testRequest(t, []forge.Approval{approval("r1", "101")}, nil)
		cases := map[string]func(Request) Request{
			"tampered policy binding":   func(r Request) Request { r.FreshnessPolicy.MaximumApprovalAgeSeconds++; return r },
			"tampered profile binding":  func(r Request) Request { r.Profile.ID = "mutated"; return r },
			"non forge trust source":    func(r Request) Request { r.Profile = testProfile(t, gp.TrustSourceIdentityProvider); return r },
			"unconfigured trust source": func(r Request) Request { r.TrustSourceID = "missing"; return r },
			"resolver error": func(r Request) Request {
				r.Resolver = gp.NewResolver(factReader{errors: map[string]error{"101": errors.New("reader broke")}})
				return r
			},
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				if _, err := Resolve(context.Background(), mutate(base)); err == nil {
					t.Fatal("Resolve unexpectedly accepted invalid operational input")
				}
			})
		}
	})
}

func assertExactKeys(t *testing.T, got map[string]any, want []string) {
	t.Helper()
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sort.Strings(want)
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
}

func TestFreshnessPolicyValidation(t *testing.T) {
	t.Parallel()
	policy, err := NewFreshnessPolicy("forge-live-policy", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Digest == "" {
		t.Fatal("policy digest is empty")
	}
	for _, mutate := range []func(FreshnessPolicy) FreshnessPolicy{
		func(p FreshnessPolicy) FreshnessPolicy { p.ID = "changed"; return p },
		func(p FreshnessPolicy) FreshnessPolicy { p.MaximumObservationAgeSeconds = 0; return p },
		func(p FreshnessPolicy) FreshnessPolicy { p.MaximumApprovalAgeSeconds = -1; return p },
	} {
		if err := mutate(policy).Validate(); err == nil {
			t.Fatal("tampered/invalid policy passed validation")
		}
	}

	a, _ := NewFreshnessPolicy("forge-live-policy", 1, 0)
	b, _ := NewFreshnessPolicy("forge-live-policy", 1, 0)
	if a != b {
		t.Fatalf("policy construction not deterministic: %+v %+v", a, b)
	}
}

func TestRecordDigestUsesCanonicalEmptyDigestProjection(t *testing.T) {
	t.Parallel()
	record := resolveRecord(t, testRequest(t, []forge.Approval{approval("r1", "101")}, nil))
	want := record.Digest
	record.Digest = strings.Repeat("x", len(want))
	got, err := recordDigest(record)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("recordDigest = %q, want %q", got, want)
	}
	bytes, err := canonjson.Marshal(record)
	if err != nil || len(bytes) == 0 {
		t.Fatalf("canonical marshal failed: %v", err)
	}
}
