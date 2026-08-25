package experimenthuman

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

func TestHumanProofDetachedSignatureAndEvidenceDigest(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	subject := "ed25519:" + base64.RawURLEncoding.EncodeToString(publicKey)
	source := humanPolicySource(subject)
	facts := testChallengeFacts()
	challenge, err := NewChallenge(facts)
	if err != nil {
		t.Fatal(err)
	}
	challengeBytes, err := challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, challengeBytes)

	got, err := Verify(context.Background(), facts, challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: source})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.State != governanceprincipal.ResolutionAuthenticated || got.Code != ReasonHumanProofVerified {
		t.Fatalf("Verify() = %+v, want authenticated/%q", got, ReasonHumanProofVerified)
	}
	wantPrincipal, err := governanceprincipal.CanonicalPrincipalID(facts.TrustSource, subject)
	if err != nil {
		t.Fatal(err)
	}
	if got.Resolution.PrincipalID != wantPrincipal {
		t.Fatalf("principal = %q, want %q", got.Resolution.PrincipalID, wantPrincipal)
	}
	preimage := append([]byte("verdi.experiment-human-proof/v1\x00"), challengeBytes...)
	preimage = append(preimage, signature...)
	sum := sha256.Sum256(preimage)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if len(got.Resolution.Witnesses) != 1 || got.Resolution.Witnesses[0].EvidenceDigest != wantDigest {
		t.Fatalf("witnesses = %+v, want evidence digest %q", got.Resolution.Witnesses, wantDigest)
	}
}

func TestHumanProofVerdictAndOperationalClassification(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	subject := "ed25519:" + base64.RawURLEncoding.EncodeToString(publicKey)
	facts := testChallengeFacts()
	challenge, err := NewChallenge(facts)
	if err != nil {
		t.Fatal(err)
	}
	challengeBytes, err := challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, challengeBytes)
	authority := AcceptedAuthority{Head: facts.AcceptedHEAD, Source: humanPolicySource(subject)}

	stale := facts
	stale.ProposalHEAD = "cccccccccccccccccccccccccccccccccccccccc"
	verdicts := []struct {
		name      string
		facts     ChallengeFacts
		challenge []byte
		proof     []byte
		authority AcceptedAuthority
		wantCode  string
	}{
		{"missing proof", facts, challengeBytes, nil, authority, ReasonHumanProofMissing},
		{"invalid signature", facts, challengeBytes, append([]byte(nil), signature...), authority, ReasonHumanProofInvalid},
		{"stale challenge", stale, challengeBytes, signature, authority, ReasonHumanChallengeStale},
		{"unknown source", withTrustSource(facts, "unknown-human"), challengeBytesFor(t, withTrustSource(facts, "unknown-human")), signature, authority, ReasonHumanTrustSourceUnknown},
		{"unmapped key", facts, challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: humanPolicySource("employee:alice")}, ReasonHumanKeyUnmapped},
	}
	verdicts[1].proof[0] ^= 0xff
	for _, tt := range verdicts {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Verify(context.Background(), tt.facts, tt.challenge, tt.proof, tt.authority)
			if err != nil {
				t.Fatalf("Verify() error = %v, want verdict", err)
			}
			if got.State == governanceprincipal.ResolutionAuthenticated || got.Code != tt.wantCode {
				t.Fatalf("Verify() = %+v, want non-authenticated code %q", got, tt.wantCode)
			}
		})
	}

	operations := []struct {
		name      string
		facts     ChallengeFacts
		challenge []byte
		proof     []byte
		authority AcceptedAuthority
	}{
		{"wrong proof length", facts, challengeBytes, signature[:63], authority},
		{"wrong proof length with stale challenge", stale, challengeBytes, signature[:63], authority},
		{"noncanonical challenge", facts, append([]byte(" "), challengeBytes...), signature, authority},
		{"accepted tree mismatch", facts, challengeBytes, signature, AcceptedAuthority{Head: "dddddddddddddddddddddddddddddddddddddddd", Source: authority.Source}},
		{"policy load failure", facts, challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: fstest.MapFS{}}},
	}
	for _, tt := range operations {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Verify(context.Background(), tt.facts, tt.challenge, tt.proof, tt.authority); err == nil {
				t.Fatal("Verify() error = nil, want operational error")
			}
		})
	}
}

func TestHumanProofMappedSubjectGrammarAndMultipleMatches(t *testing.T) {
	publicKey1, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey2, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	valid1 := "ed25519:" + base64.RawURLEncoding.EncodeToString(publicKey1)
	valid2 := "ed25519:" + base64.RawURLEncoding.EncodeToString(publicKey2)
	profile, err := loadHumanProfile(humanPolicySourceWithSubjects([]string{
		"ed25519:" + base64.URLEncoding.EncodeToString(publicKey1),
		"ed25519:not-base64",
		"employee:alice",
		valid1,
		valid2,
	}))
	if err != nil {
		t.Fatal(err)
	}
	matches, err := matchingSubjects(profile, "offline-human", []byte("challenge"), make([]byte, ed25519.SignatureSize), func(ed25519.PublicKey, []byte, []byte) bool { return true })
	if err != nil {
		t.Fatalf("matchingSubjects() error = %v", err)
	}
	want := []string{valid1, valid2}
	sort.Strings(want)
	if len(matches) != 2 || matches[0] != want[0] || matches[1] != want[1] {
		t.Fatalf("matchingSubjects() = %v, want exact RawURL Ed25519 subjects %v", matches, want)
	}

	facts := testChallengeFacts()
	challengeBytes := challengeBytesFor(t, facts)
	_, err = verifyWith(
		context.Background(),
		facts,
		challengeBytes,
		make([]byte, ed25519.SignatureSize),
		AcceptedAuthority{Head: facts.AcceptedHEAD, Source: humanPolicySourceWithSubjects([]string{valid1, valid2})},
		func(ed25519.PublicKey, []byte, []byte) bool { return true },
	)
	if err == nil {
		t.Fatal("verifyWith(two matching mapped keys) error = nil, want operational ambiguity")
	}
}

func withTrustSource(facts ChallengeFacts, source string) ChallengeFacts {
	facts.TrustSource = source
	return facts
}

func challengeBytesFor(t *testing.T, facts ChallengeFacts) []byte {
	t.Helper()
	challenge, err := NewChallenge(facts)
	if err != nil {
		t.Fatal(err)
	}
	data, err := challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func humanPolicySource(subject string) fstest.MapFS {
	return humanPolicySourceWithSubjects([]string{subject})
}

func humanPolicySourceWithSubjects(subjects []string) fstest.MapFS {
	profileSubjects := ""
	for _, subject := range subjects {
		profileSubjects += "      - \"" + subject + "\"\n"
	}
	return fstest.MapFS{
		".verdi/policy/constitution.md": {Data: []byte(`---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Human proof fixture"
owners: [platform-team]
selected_profile: human-default
environments: []
catalog:
  roles: [operator]
  transitions: [accept]
  evidence_sources: []
  escalation_metrics: []
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters: []
---
Human proof fixture constitution.
`)},
		".verdi/policy/profiles/human-default.md": {Data: []byte(`---
schema: verdi.governance-profile/v1
id: human-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: offline-human, kind: identity-provider}
role_mappings:
  - role: operator
    trust_source: offline-human
    subjects:
` + profileSubjects + `ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Human proof fixture profile.
`)},
	}
}

func loadHumanProfile(source fstest.MapFS) (governanceprincipal.Profile, error) {
	store, err := loadPolicyStore(source)
	if err != nil {
		return governanceprincipal.Profile{}, err
	}
	return store.SelectedProfile()
}
