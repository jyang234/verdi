package governanceprincipal

import (
	"context"
	"strings"
	"testing"
)

// forgedProfile is a hand-built team profile with no approval or
// distinctness rules — exactly the value that must never authorize.
func forgedProfile() Profile {
	return Profile{
		Schema:                "verdi.governance-profile/v1",
		ID:                    "forged-team",
		Class:                 ClassTeam,
		ApplicableTransitions: []string{"accept"},
		IdentityTrustSources:  []TrustSource{{ID: "github", Kind: TrustSourceForge}},
		RoleMappings: []RoleMapping{
			{Role: "author", TrustSource: "github", Subjects: []string{"user-123"}},
			{Role: "reviewer", TrustSource: "github", Subjects: []string{"user-456"}},
		},
		OwnershipSources:           []OwnershipSource{},
		SignatureRequirements:      []SignatureRequirement{},
		RequiredApprovers:          []ApproverRequirement{},
		DistinctnessRules:          []DistinctnessRule{},
		EvidenceSourceRestrictions: []EvidenceSourceRestriction{},
		EscalationThresholds:       []EscalationThreshold{},
	}
}

// TestProfileSealRejectsForgery: a manually constructed Profile is
// refused by every public kernel operation.
func TestProfileSealRejectsForgery(t *testing.T) {
	forged := forgedProfile()

	if _, err := forged.Digest(); err == nil || !strings.Contains(err.Error(), "DecodeProfile") {
		t.Errorf("Digest on forged profile = %v, want DecodeProfile provenance error", err)
	}

	if _, err := Authorize(forged, AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: mustPID(t, "user-123")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
		},
	}); err == nil || !strings.Contains(err.Error(), "DecodeProfile") {
		t.Errorf("Authorize on forged profile = %v, want DecodeProfile provenance error", err)
	}

	r := NewResolver(staticFact(githubFact("user-123")))
	if _, err := r.Resolve(context.Background(), forged, PrincipalClaim{TrustSource: "github", Subject: "user-123"}); err == nil || !strings.Contains(err.Error(), "DecodeProfile") {
		t.Errorf("Resolve on forged profile = %v, want DecodeProfile provenance error", err)
	}
}

// TestProfileSealDetectsMutation: post-decode mutation of every
// authority-bearing family is detected; an unchanged decoded profile
// keeps working.
func TestProfileSealDetectsMutation(t *testing.T) {
	for _, tt := range profileMutators {
		t.Run(tt.name, func(t *testing.T) {
			p := mustDecode(t, profileYAML())
			tt.mutate(&p)
			if _, err := p.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
				t.Errorf("Digest on mutated profile = %v, want modification error", err)
			}
			if _, err := Authorize(p, AuthorizationRequest{Transition: "accept", Posture: PostureAuthoritative}); err == nil || !strings.Contains(err.Error(), "modified") {
				t.Errorf("Authorize on mutated profile = %v, want modification error", err)
			}
			r := NewResolver(staticFact(githubFact("user-123")))
			if _, err := r.Resolve(context.Background(), p, PrincipalClaim{TrustSource: "github", Subject: "user-123"}); err == nil || !strings.Contains(err.Error(), "modified") {
				t.Errorf("Resolve on mutated profile = %v, want modification error", err)
			}
		})
	}

	// A copied-but-unchanged decoded profile continues to work everywhere.
	p := mustDecode(t, profileYAML())
	q := p
	if _, err := q.Digest(); err != nil {
		t.Errorf("Digest on unchanged profile: %v", err)
	}
	if _, err := Authorize(q, AuthorizationRequest{Transition: "accept", Posture: PostureAuthoritative}); err != nil {
		t.Errorf("Authorize on unchanged profile: %v", err)
	}
	r := NewResolver(staticFact(githubFact("user-123")))
	if _, err := r.Resolve(context.Background(), q, PrincipalClaim{TrustSource: "github", Subject: "user-123"}); err != nil {
		t.Errorf("Resolve on unchanged profile: %v", err)
	}
}

// TestResolutionSealRejectsForgery: a hand-built, internally consistent
// authenticated resolution must not be trusted by Authorize or
// AttributionFromResolution.
func TestResolutionSealRejectsForgery(t *testing.T) {
	forged := PrincipalResolution{
		Claim:       PrincipalClaim{TrustSource: "github", Subject: "user-123"},
		PrincipalID: mustPID(t, "user-123"),
		State:       ResolutionAuthenticated,
		Witnesses:   []Witness{{Code: ReasonTrustSubjectVerified, SourceID: "github", EvidenceDigest: testDigest}},
	}
	forgedReviewer := PrincipalResolution{
		Claim:       PrincipalClaim{TrustSource: "github", Subject: "user-456"},
		PrincipalID: mustPID(t, "user-456"),
		State:       ResolutionAuthenticated,
		Witnesses:   []Witness{{Code: ReasonTrustSubjectVerified, SourceID: "github", EvidenceDigest: testDigest}},
	}

	profile := authzProfile(t, nil)
	if _, err := Authorize(profile, AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{forged, forgedReviewer},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: forged.PrincipalID},
			{Role: "reviewer", PrincipalID: forgedReviewer.PrincipalID},
		},
	}); err == nil || !strings.Contains(err.Error(), "Resolver.Resolve") {
		t.Errorf("Authorize with forged resolutions = %v, want Resolver.Resolve provenance error", err)
	}

	if _, err := AttributionFromResolution(forged); err == nil || !strings.Contains(err.Error(), "Resolver.Resolve") {
		t.Errorf("AttributionFromResolution(forged) = %v, want Resolver.Resolve provenance error", err)
	}
}

// TestResolutionSealDetectsMutation: a genuine resolution that is copied
// and mutated — including witness replacement, alteration, or removal —
// is rejected; unchanged resolver output stays accepted.
func TestResolutionSealDetectsMutation(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*PrincipalResolution)
	}{
		{"witness digest altered", func(r *PrincipalResolution) {
			r.Witnesses[0].EvidenceDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{"witness code replaced", func(r *PrincipalResolution) { r.Witnesses[0].Code = ReasonTrustEvidenceInvalid }},
		{"witnesses removed", func(r *PrincipalResolution) { r.Witnesses = nil }},
		{"witness appended", func(r *PrincipalResolution) {
			r.Witnesses = append(r.Witnesses, Witness{Code: ReasonTrustSubjectVerified, SourceID: "github"})
		}},
		{"claim subject rewritten", func(r *PrincipalResolution) {
			r.Claim.Subject = "user-456"
			r.PrincipalID = mustPID(t, "user-456")
		}},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			res := authedRes(t, "user-123")
			tt.mutate(&res)
			if _, err := AttributionFromResolution(res); err == nil || !strings.Contains(err.Error(), "modified") {
				t.Errorf("AttributionFromResolution(mutated) = %v, want modification error", err)
			}
			profile := authzProfile(t, nil)
			if _, err := Authorize(profile, AuthorizationRequest{
				Transition:  "accept",
				Posture:     PostureAuthoritative,
				Resolutions: []PrincipalResolution{res},
				Approvals:   []ApprovalRecord{{Role: "author", PrincipalID: res.PrincipalID}},
			}); err == nil || !strings.Contains(err.Error(), "modified") {
				t.Errorf("Authorize with mutated resolution = %v, want modification error", err)
			}
		})
	}

	// Unchanged resolver output stays accepted in both consumers.
	res := authedRes(t, "user-123")
	if _, err := AttributionFromResolution(res); err != nil {
		t.Errorf("AttributionFromResolution(genuine): %v", err)
	}
	profile := authzProfile(t, nil)
	if _, err := Authorize(profile, AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{res},
		Approvals:   []ApprovalRecord{{Role: "author", PrincipalID: res.PrincipalID}},
	}); err != nil {
		t.Errorf("Authorize with genuine resolution: %v", err)
	}
}
