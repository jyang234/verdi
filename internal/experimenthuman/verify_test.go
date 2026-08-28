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

	// Task 10 correction (SI-150, design §7): a successful authenticated
	// Verify additionally mints the sealed RetainedProof a proposal
	// projects into ratification v3 — the exact challenge/signature bytes,
	// resolved claim, kernel principal id, and the same SI-147 evidence
	// digest.
	retainedChallenge, err := got.Retained.ChallengeBytes()
	if err != nil || string(retainedChallenge) != string(challengeBytes) {
		t.Fatalf("got.Retained.ChallengeBytes() = %q/%v, want %q", retainedChallenge, err, challengeBytes)
	}
	retainedSignature, err := got.Retained.Signature()
	if err != nil || string(retainedSignature) != string(signature) {
		t.Fatalf("got.Retained.Signature() = %q/%v, want %q", retainedSignature, err, signature)
	}
	retainedClaim, err := got.Retained.Claim()
	if err != nil || retainedClaim.TrustSource != facts.TrustSource || retainedClaim.Subject != subject {
		t.Fatalf("got.Retained.Claim() = %+v/%v, want trust_source %q subject %q", retainedClaim, err, facts.TrustSource, subject)
	}
	retainedPrincipal, err := got.Retained.PrincipalID()
	if err != nil || retainedPrincipal != wantPrincipal {
		t.Fatalf("got.Retained.PrincipalID() = %q/%v, want %q", retainedPrincipal, err, wantPrincipal)
	}
	retainedDigest, err := got.Retained.EvidenceDigest()
	if err != nil || retainedDigest != wantDigest {
		t.Fatalf("got.Retained.EvidenceDigest() = %q/%v, want %q", retainedDigest, err, wantDigest)
	}
}

// TestVerifyMintsRetainedProofOnlyOnAuthenticatedSuccess pins the seal:
// RetainedProof is mintable only inside a successful authenticated
// Verify (design §7). Every non-authenticated path — verdict or the zero
// value itself — must fail every accessor rather than silently returning
// hollow data.
func TestVerifyMintsRetainedProofOnlyOnAuthenticatedSuccess(t *testing.T) {
	var zero RetainedProof
	if _, err := zero.ChallengeBytes(); err == nil {
		t.Fatalf("zero-value RetainedProof.ChallengeBytes() = nil error, want unsealed refusal")
	}
	if _, err := zero.Signature(); err == nil {
		t.Fatalf("zero-value RetainedProof.Signature() = nil error, want unsealed refusal")
	}
	if _, err := zero.Claim(); err == nil {
		t.Fatalf("zero-value RetainedProof.Claim() = nil error, want unsealed refusal")
	}
	if _, err := zero.PrincipalID(); err == nil {
		t.Fatalf("zero-value RetainedProof.PrincipalID() = nil error, want unsealed refusal")
	}
	if _, err := zero.EvidenceDigest(); err == nil {
		t.Fatalf("zero-value RetainedProof.EvidenceDigest() = nil error, want unsealed refusal")
	}

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
	tampered := append([]byte(nil), signature...)
	tampered[0] ^= 0xff
	nonAuthenticated := []struct {
		name  string
		facts ChallengeFacts
		proof []byte
	}{
		{"missing proof", facts, nil},
		{"invalid signature", facts, tampered},
		{"stale challenge", stale, signature},
	}
	for _, tt := range nonAuthenticated {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Verify(context.Background(), tt.facts, challengeBytes, tt.proof, authority)
			if err != nil {
				t.Fatalf("Verify() error = %v, want verdict", err)
			}
			if got.State == governanceprincipal.ResolutionAuthenticated {
				t.Fatalf("Verify() = %+v, want a non-authenticated verdict", got)
			}
			if _, err := got.Retained.ChallengeBytes(); err == nil {
				t.Fatalf("%s: Retained.ChallengeBytes() = nil error, want no proof minted on a non-authenticated verdict", tt.name)
			}
		})
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

// TestVerifyRetainedMatrix is the accepted-use signature re-verification
// matrix (Task 10 correction, SI-150, design §7; controller pin P1/P4):
// VerifyRetained decodes a retained challenge, loads the HISTORICAL
// accepted profile from a caller-supplied fs.FS, and verifies the
// signature against its mapped candidate keys — performing no kernel
// resolution itself.
func TestVerifyRetainedMatrix(t *testing.T) {
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

	t.Run("historical verify pass", func(t *testing.T) {
		got, err := VerifyRetained(challengeBytes, signature, authority)
		if err != nil {
			t.Fatalf("VerifyRetained() error = %v", err)
		}
		if !got.Verified || got.Code != ReasonHumanProofVerified || got.Subject != subject {
			t.Fatalf("VerifyRetained() = %+v, want verified/%q/subject %q", got, ReasonHumanProofVerified, subject)
		}
		if got.Challenge != facts {
			t.Fatalf("VerifyRetained().Challenge = %+v, want %+v", got.Challenge, facts)
		}
		if !got.Fact.Available || !got.Fact.Valid || len(got.Fact.Subjects) != 1 || got.Fact.Subjects[0] != subject {
			t.Fatalf("VerifyRetained().Fact = %+v, want an available/valid fact naming only the verified subject", got.Fact)
		}
		wantDigest := proofEvidenceDigest(challengeBytes, signature)
		if got.Fact.EvidenceDigest != wantDigest {
			t.Fatalf("VerifyRetained().Fact.EvidenceDigest = %q, want %q", got.Fact.EvidenceDigest, wantDigest)
		}
	})

	// F3 (lane review, controller-adjudicated remedy): the historical-tree
	// substitution hazard is closed STRUCTURALLY inside VerifyRetained,
	// not by caller discipline — accepted.Head must equal the retained
	// challenge's own signed accepted_head, the same shape Verify already
	// enforces for the proposal-time accepted authority.
	t.Run("accepted authority head mismatches the retained challenge", func(t *testing.T) {
		mismatched := AcceptedAuthority{Head: "dddddddddddddddddddddddddddddddddddddddd", Source: humanPolicySource(subject)}
		if _, err := VerifyRetained(challengeBytes, signature, mismatched); err == nil {
			t.Fatalf("VerifyRetained(mismatched accepted HEAD) error = nil, want operational refusal")
		}
	})

	t.Run("accepted authority head matches the retained challenge", func(t *testing.T) {
		got, err := VerifyRetained(challengeBytes, signature, authority)
		if err != nil {
			t.Fatalf("VerifyRetained() error = %v", err)
		}
		if !got.Verified || got.Code != ReasonHumanProofVerified {
			t.Fatalf("VerifyRetained(matching accepted HEAD) = %+v, want verified/%q", got, ReasonHumanProofVerified)
		}
	})

	t.Run("unknown source", func(t *testing.T) {
		unknownFacts := withTrustSource(facts, "unknown-human")
		unknownChallengeBytes := challengeBytesFor(t, unknownFacts)
		unknownSignature := ed25519.Sign(privateKey, unknownChallengeBytes)
		got, err := VerifyRetained(unknownChallengeBytes, unknownSignature, AcceptedAuthority{Head: unknownFacts.AcceptedHEAD, Source: humanPolicySource(subject)})
		if err != nil {
			t.Fatalf("VerifyRetained() error = %v, want a verdict", err)
		}
		if got.Verified || got.Code != ReasonHumanTrustSourceUnknown {
			t.Fatalf("VerifyRetained(unknown source) = %+v, want unverified/%q", got, ReasonHumanTrustSourceUnknown)
		}
	})

	t.Run("unmapped subject (garbage subjects only)", func(t *testing.T) {
		got, err := VerifyRetained(challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: humanPolicySourceWithSubjects([]string{"ed25519:not-base64", "employee:alice"})})
		if err != nil {
			t.Fatalf("VerifyRetained() error = %v, want a verdict", err)
		}
		if got.Verified || got.Code != ReasonHumanKeyUnmapped {
			t.Fatalf("VerifyRetained(unmapped subject) = %+v, want unverified/%q", got, ReasonHumanKeyUnmapped)
		}
	})

	t.Run("zero keys (no role mapping at all)", func(t *testing.T) {
		got, err := VerifyRetained(challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: humanPolicySourceWithNoRoleMapping()})
		if err != nil {
			t.Fatalf("VerifyRetained() error = %v, want a verdict", err)
		}
		if got.Verified || got.Code != ReasonHumanKeyUnmapped {
			t.Fatalf("VerifyRetained(zero keys) = %+v, want unverified/%q", got, ReasonHumanKeyUnmapped)
		}
	})

	t.Run("two keys (ambiguous)", func(t *testing.T) {
		publicKey2, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		subject2 := "ed25519:" + base64.RawURLEncoding.EncodeToString(publicKey2)
		_, err = verifyRetainedWith(challengeBytes, make([]byte, ed25519.SignatureSize), AcceptedAuthority{Head: facts.AcceptedHEAD, Source: humanPolicySourceWithSubjects([]string{subject, subject2})},
			func(ed25519.PublicKey, []byte, []byte) bool { return true })
		if err == nil {
			t.Fatalf("verifyRetainedWith(two matching mapped keys) error = nil, want operational ambiguity")
		}
	})

	t.Run("bad signature", func(t *testing.T) {
		tampered := append([]byte(nil), signature...)
		tampered[0] ^= 0xff
		got, err := VerifyRetained(challengeBytes, tampered, authority)
		if err != nil {
			t.Fatalf("VerifyRetained() error = %v, want a verdict", err)
		}
		if got.Verified || got.Code != ReasonHumanProofInvalid {
			t.Fatalf("VerifyRetained(bad signature) = %+v, want unverified/%q", got, ReasonHumanProofInvalid)
		}
	})

	t.Run("malformed challenge", func(t *testing.T) {
		if _, err := VerifyRetained(append([]byte(" "), challengeBytes...), signature, authority); err == nil {
			t.Fatalf("VerifyRetained(malformed challenge) error = nil, want operational refusal")
		}
	})

	t.Run("wrong signature length is operational", func(t *testing.T) {
		if _, err := VerifyRetained(challengeBytes, signature[:63], authority); err == nil {
			t.Fatalf("VerifyRetained(63-byte signature) error = nil, want operational refusal")
		}
	})

	t.Run("policy load failure is operational", func(t *testing.T) {
		if _, err := VerifyRetained(challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: fstest.MapFS{}}); err == nil {
			t.Fatalf("VerifyRetained(broken policy source) error = nil, want operational refusal")
		}
	})

	t.Run("nil policy source is operational", func(t *testing.T) {
		if _, err := VerifyRetained(challengeBytes, signature, AcceptedAuthority{Head: facts.AcceptedHEAD, Source: nil}); err == nil {
			t.Fatalf("VerifyRetained(nil source) error = nil, want operational refusal")
		}
	})
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

// humanPolicySourceWithNoRoleMapping registers the offline-human identity
// source (so it is KNOWN) but maps no role to it at all — distinct from
// humanPolicySourceWithSubjects's "subjects present but none decode as a
// valid Ed25519 key" case: here there is no role-mapping entry to read a
// subject from in the first place.
func humanPolicySourceWithNoRoleMapping() fstest.MapFS {
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
role_mappings: []
ownership_sources: []
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
