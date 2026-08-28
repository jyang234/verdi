package experimenthuman

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// proofDigestDomain is the SI-147 evidence-digest domain separator. It is
// the SAME single-sourced literal as the v3 authentication_proof block's
// own `schema` field (controller pin P3): internal/experiment is the
// wire-schema home and owns the constant; this package consumes it
// rather than defining a second copy.
const proofDigestDomain = experiment.HumanProofSchema + "\x00"

const (
	ReasonHumanProofVerified      = "human-proof-verified"
	ReasonHumanProofMissing       = "human-proof-missing"
	ReasonHumanProofInvalid       = "human-proof-invalid"
	ReasonHumanChallengeStale     = "human-challenge-stale"
	ReasonHumanTrustSourceUnknown = "human-trust-source-unknown"
	ReasonHumanKeyUnmapped        = "human-key-unmapped"
)

// AcceptedAuthority pairs an exact accepted HEAD with its read-only tree.
type AcceptedAuthority struct {
	Head   string
	Source fs.FS
}

// Verification is either a stable non-operational verdict or the kernel's
// sealed authenticated resolution. Operational failures are returned as errors.
type Verification struct {
	State      governanceprincipal.ResolutionState
	Code       string
	Resolution governanceprincipal.PrincipalResolution
	// Retained is the sealed RetainedProof minted only alongside a
	// successful authenticated resolution (design §7, SI-150). Its zero
	// value fails every accessor — no proof was minted on a
	// non-authenticated verdict.
	Retained RetainedProof
}

// RetainedProof is the sealed, action-bound evidence a successful
// authenticated Verify mints (design §7; SI-150 narrows SI-147's
// no-durable-token rule only for this proof): the exact retained
// challenge bytes, detached Ed25519 signature, resolved principal claim,
// kernel-derived principal id, and the SI-147 evidence digest a
// ratification proposal projects into the v3 authentication_proof block.
// Mirrors governanceprincipal.PrincipalResolution's unexported-seal
// precedent — every field is unexported, only a successful Verify can
// populate them, and the value is never serialized: it is in-memory
// transport for one proposal, never a durable session, credential, or
// reusable identity token.
type RetainedProof struct {
	challengeBytes []byte
	signature      []byte
	claim          governanceprincipal.PrincipalClaim
	principalID    governanceprincipal.PrincipalID
	evidenceDigest string
	seal           string
}

// checkSeal proves the value was produced by verifyWith's authenticated
// success path and has not been altered since — mirroring
// governanceprincipal.PrincipalResolution.checkSeal. The zero value
// (never minted) always fails: seal is empty before any content check.
func (p RetainedProof) checkSeal() error {
	if p.seal == "" {
		return fmt.Errorf("experimenthuman: retained proof was not produced by a successful authenticated Verify")
	}
	want := retainedProofSeal(p.challengeBytes, p.signature, p.claim, p.principalID, p.evidenceDigest)
	if want != p.seal {
		return fmt.Errorf("experimenthuman: retained proof seal does not match its content")
	}
	return nil
}

// retainedProofSeal computes the content-bound seal minted alongside a
// successful authenticated Verify.
func retainedProofSeal(challengeBytes, signature []byte, claim governanceprincipal.PrincipalClaim, principalID governanceprincipal.PrincipalID, evidenceDigest string) string {
	h := sha256.New()
	_, _ = h.Write([]byte("verdi.experiment-retained-proof/v1\x00"))
	_, _ = h.Write(challengeBytes)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(signature)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(claim.TrustSource))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(claim.Subject))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(principalID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(evidenceDigest))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// mintRetainedProof seals a fresh RetainedProof. It is called ONLY from
// verifyWith's authenticated success path.
func mintRetainedProof(challengeBytes, signature []byte, claim governanceprincipal.PrincipalClaim, principalID governanceprincipal.PrincipalID, evidenceDigest string) RetainedProof {
	p := RetainedProof{
		challengeBytes: append([]byte(nil), challengeBytes...),
		signature:      append([]byte(nil), signature...),
		claim:          claim,
		principalID:    principalID,
		evidenceDigest: evidenceDigest,
	}
	p.seal = retainedProofSeal(p.challengeBytes, p.signature, p.claim, p.principalID, p.evidenceDigest)
	return p
}

// ChallengeBytes returns the exact retained challenge bytes a successful
// Verify authenticated. It fails on any value not minted by Verify.
func (p RetainedProof) ChallengeBytes() ([]byte, error) {
	if err := p.checkSeal(); err != nil {
		return nil, err
	}
	return append([]byte(nil), p.challengeBytes...), nil
}

// Signature returns the exact retained detached Ed25519 signature.
func (p RetainedProof) Signature() ([]byte, error) {
	if err := p.checkSeal(); err != nil {
		return nil, err
	}
	return append([]byte(nil), p.signature...), nil
}

// Claim returns the resolved principal claim Verify authenticated.
func (p RetainedProof) Claim() (governanceprincipal.PrincipalClaim, error) {
	if err := p.checkSeal(); err != nil {
		return governanceprincipal.PrincipalClaim{}, err
	}
	return p.claim, nil
}

// PrincipalID returns the kernel-derived canonical principal id Verify's
// resolution carried.
func (p RetainedProof) PrincipalID() (governanceprincipal.PrincipalID, error) {
	if err := p.checkSeal(); err != nil {
		return "", err
	}
	return p.principalID, nil
}

// EvidenceDigest returns the SI-147 evidence digest over the retained
// challenge and signature bytes.
func (p RetainedProof) EvidenceDigest() (string, error) {
	if err := p.checkSeal(); err != nil {
		return "", err
	}
	return p.evidenceDigest, nil
}

// Verify checks one raw detached signature against accepted profile authority
// and delegates principal minting to the governance-principal resolver.
func Verify(ctx context.Context, current ChallengeFacts, challengeBytes, signature []byte, accepted AcceptedAuthority) (Verification, error) {
	return verifyWith(ctx, current, challengeBytes, signature, accepted, ed25519.Verify)
}

func verifyWith(ctx context.Context, current ChallengeFacts, challengeBytes, signature []byte, accepted AcceptedAuthority, verify verifyFunc) (Verification, error) {
	if ctx == nil {
		return Verification{}, fmt.Errorf("experimenthuman: context is nil")
	}
	if verify == nil {
		return Verification{}, fmt.Errorf("experimenthuman: signature verifier is nil")
	}
	challenge, err := DecodeChallenge(challengeBytes)
	if err != nil {
		return Verification{}, err
	}
	expected, err := NewChallenge(current)
	if err != nil {
		return Verification{}, err
	}
	if len(signature) == 0 {
		return verdict(governanceprincipal.ResolutionUnproven, ReasonHumanProofMissing), nil
	}
	if len(signature) != ed25519.SignatureSize {
		return Verification{}, fmt.Errorf("experimenthuman: detached Ed25519 signature is %d bytes, want %d", len(signature), ed25519.SignatureSize)
	}
	if challenge != expected {
		return verdict(governanceprincipal.ResolutionViolated, ReasonHumanChallengeStale), nil
	}
	if accepted.Source == nil {
		return Verification{}, fmt.Errorf("experimenthuman: accepted authority source is nil")
	}
	if accepted.Head != current.AcceptedHEAD {
		return Verification{}, fmt.Errorf("experimenthuman: accepted authority HEAD %q does not match challenge HEAD %q", accepted.Head, current.AcceptedHEAD)
	}
	store, err := loadPolicyStore(accepted.Source)
	if err != nil {
		return Verification{}, fmt.Errorf("experimenthuman: loading accepted policy authority: %w", err)
	}
	profile, err := store.SelectedProfile()
	if err != nil {
		return Verification{}, fmt.Errorf("experimenthuman: selecting accepted governance profile: %w", err)
	}
	sourceKnown := false
	for _, source := range profile.IdentityTrustSources {
		if source.ID == current.TrustSource && source.Kind == governanceprincipal.TrustSourceIdentityProvider {
			sourceKnown = true
			break
		}
	}
	if !sourceKnown {
		return verdict(governanceprincipal.ResolutionViolated, ReasonHumanTrustSourceUnknown), nil
	}
	candidates := candidateSubjects(profile, current.TrustSource)
	if len(candidates) == 0 {
		return verdict(governanceprincipal.ResolutionViolated, ReasonHumanKeyUnmapped), nil
	}
	matches, err := matchingSubjects(profile, current.TrustSource, challengeBytes, signature, verify)
	if err != nil {
		return Verification{}, err
	}
	switch len(matches) {
	case 0:
		return verdict(governanceprincipal.ResolutionViolated, ReasonHumanProofInvalid), nil
	case 1:
	default:
		return Verification{}, fmt.Errorf("experimenthuman: %d mapped Ed25519 keys verified one proof, want exactly one", len(matches))
	}

	subject := matches[0]
	digest := proofEvidenceDigest(challengeBytes, signature)
	reader := staticFactReader{fact: governanceprincipal.TrustFact{
		SourceID: current.TrustSource, SourceKind: governanceprincipal.TrustSourceIdentityProvider,
		Subjects: []string{subject}, EvidenceDigest: digest, Available: true, Valid: true,
	}}
	resolution, err := governanceprincipal.NewResolver(reader).Resolve(ctx, profile, governanceprincipal.PrincipalClaim{
		TrustSource: current.TrustSource,
		Subject:     subject,
	})
	if err != nil {
		return Verification{}, fmt.Errorf("experimenthuman: resolving verified human fact: %w", err)
	}
	if resolution.State != governanceprincipal.ResolutionAuthenticated {
		return Verification{}, fmt.Errorf("experimenthuman: verified human fact resolved to unexpected state %q", resolution.State)
	}
	retained := mintRetainedProof(challengeBytes, signature, resolution.Claim, resolution.PrincipalID, digest)
	return Verification{State: resolution.State, Code: ReasonHumanProofVerified, Resolution: resolution, Retained: retained}, nil
}

func verdict(state governanceprincipal.ResolutionState, code string) Verification {
	return Verification{State: state, Code: code}
}

// RetainedVerification is VerifyRetained's result: either a stable
// non-operational classification code (Verified == false), or — only on
// a verified signature — the decoded challenge facts, the exactly one
// verified subject, and the normalized trust fact the caller resolves
// through governanceprincipal.Resolver against the CURRENT accepted
// profile. VerifyRetained performs NO kernel resolution itself (design
// §7; Task 10 correction controller pins P1/P4): role-mapping membership
// is never itself reported as evidence, and only the caller decides
// principal equality against the current profile.
//
// Challenge is populated whenever decode and length checks succeed,
// including every unverified (Verified == false) outcome — those facts
// are UNAUTHENTICATED challenge content at that point (the signature has
// not verified, or verified against the wrong tree), fit only for
// witness/diagnostic prose, never for an authorization decision (lane
// review F4).
type RetainedVerification struct {
	Verified  bool
	Code      string
	Challenge ChallengeFacts
	Subject   string
	Fact      governanceprincipal.TrustFact
}

// VerifyRetained re-verifies one action-bound retained ratification proof
// at accepted use (design §7, SI-150): it strict-decodes challengeBytes
// as a verdi.experiment-human-challenge/v1 document, requires
// accepted.Head to equal the retained challenge's own signed
// accepted_head (closing the historical-tree substitution hazard
// STRUCTURALLY inside this helper rather than by caller discipline —
// lane review F3, the same shape Verify already enforces for the
// proposal-time accepted authority), loads the selected governance
// profile from accepted.Source — the caller's resolved read-only view of
// that EXACT historical tree (same loadPolicyStore path as Verify),
// never the current worktree or current profile — requires the
// challenge's trust source to be a known identity-provider source of
// that historical profile, and verifies signature against exactly one of
// its mapped Ed25519 candidate subjects (reusing candidateSubjects /
// matchingSubjects). It never resolves against the governance kernel:
// that is the caller's job, against the CURRENT accepted profile.
//
// Malformed retained challenge bytes, a wrong-length signature, a
// mismatched or nil accepted authority, or a broken policy/profile
// contract are operational errors (controller pin P1). Two or more
// historical keys verifying is also operational — the existing
// experimenthuman ambiguity rule. An unknown trust source, zero mapped
// candidate keys, and a signature that matches none of them are stable
// non-operational classifications, not errors.
func VerifyRetained(challengeBytes, signature []byte, accepted AcceptedAuthority) (RetainedVerification, error) {
	return verifyRetainedWith(challengeBytes, signature, accepted, ed25519.Verify)
}

func verifyRetainedWith(challengeBytes, signature []byte, accepted AcceptedAuthority, verify verifyFunc) (RetainedVerification, error) {
	if verify == nil {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: signature verifier is nil")
	}
	challenge, err := DecodeChallenge(challengeBytes)
	if err != nil {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: retained challenge: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: retained signature is %d bytes, want %d", len(signature), ed25519.SignatureSize)
	}
	if accepted.Source == nil {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: historical accepted policy source is nil")
	}
	if accepted.Head != challenge.AcceptedHEAD {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: historical accepted authority HEAD %q does not match the retained challenge's signed accepted_head %q", accepted.Head, challenge.AcceptedHEAD)
	}
	store, err := loadPolicyStore(accepted.Source)
	if err != nil {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: loading historical accepted policy authority: %w", err)
	}
	profile, err := store.SelectedProfile()
	if err != nil {
		return RetainedVerification{}, fmt.Errorf("experimenthuman: selecting historical accepted governance profile: %w", err)
	}

	facts := ChallengeFacts{
		Operation: challenge.Operation, Spike: challenge.Spike, ExperimentID: challenge.ExperimentID,
		AcceptedHEAD: challenge.AcceptedHEAD, ProposalHEAD: challenge.ProposalHEAD,
		TrustSource: challenge.TrustSource, InputDigest: challenge.InputDigest, ProposalDigest: challenge.ProposalDigest,
	}

	sourceKnown := false
	for _, ts := range profile.IdentityTrustSources {
		if ts.ID == challenge.TrustSource && ts.Kind == governanceprincipal.TrustSourceIdentityProvider {
			sourceKnown = true
			break
		}
	}
	if !sourceKnown {
		return RetainedVerification{Code: ReasonHumanTrustSourceUnknown, Challenge: facts}, nil
	}
	candidates := candidateSubjects(profile, challenge.TrustSource)
	if len(candidates) == 0 {
		return RetainedVerification{Code: ReasonHumanKeyUnmapped, Challenge: facts}, nil
	}
	matches, err := matchingSubjects(profile, challenge.TrustSource, challengeBytes, signature, verify)
	if err != nil {
		return RetainedVerification{}, err
	}
	switch len(matches) {
	case 0:
		return RetainedVerification{Code: ReasonHumanProofInvalid, Challenge: facts}, nil
	case 1:
	default:
		return RetainedVerification{}, fmt.Errorf("experimenthuman: %d mapped Ed25519 keys verified one retained proof, want exactly one", len(matches))
	}

	subject := matches[0]
	digest := proofEvidenceDigest(challengeBytes, signature)
	fact := governanceprincipal.TrustFact{
		SourceID: challenge.TrustSource, SourceKind: governanceprincipal.TrustSourceIdentityProvider,
		Subjects: []string{subject}, EvidenceDigest: digest, Available: true, Valid: true,
	}
	return RetainedVerification{Verified: true, Code: ReasonHumanProofVerified, Challenge: facts, Subject: subject, Fact: fact}, nil
}

func loadPolicyStore(source fs.FS) (*policyauthority.Store, error) {
	return policyauthority.LoadFromSource(source)
}

type verifyFunc func(ed25519.PublicKey, []byte, []byte) bool

func matchingSubjects(profile governanceprincipal.Profile, trustSource string, challenge, signature []byte, verify verifyFunc) ([]string, error) {
	if verify == nil {
		return nil, fmt.Errorf("experimenthuman: signature verifier is nil")
	}
	var matches []string
	for _, subject := range candidateSubjects(profile, trustSource) {
		publicKey, ok := publicKeyFromSubject(subject)
		if ok && verify(publicKey, challenge, signature) {
			matches = append(matches, subject)
		}
	}
	return matches, nil
}

func candidateSubjects(profile governanceprincipal.Profile, trustSource string) []string {
	unique := map[string]bool{}
	for _, mapping := range profile.RoleMappings {
		if mapping.TrustSource != trustSource {
			continue
		}
		for _, subject := range mapping.Subjects {
			if _, ok := publicKeyFromSubject(subject); ok {
				unique[subject] = true
			}
		}
	}
	result := make([]string, 0, len(unique))
	for subject := range unique {
		result = append(result, subject)
	}
	sort.Strings(result)
	return result
}

func publicKeyFromSubject(subject string) (ed25519.PublicKey, bool) {
	const prefix = "ed25519:"
	if !strings.HasPrefix(subject, prefix) {
		return nil, false
	}
	encoded := strings.TrimPrefix(subject, prefix)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != ed25519.PublicKeySize || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return nil, false
	}
	return ed25519.PublicKey(decoded), true
}

func proofEvidenceDigest(challenge, signature []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(proofDigestDomain))
	_, _ = hash.Write(challenge)
	_, _ = hash.Write(signature)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

type staticFactReader struct {
	fact governanceprincipal.TrustFact
}

func (r staticFactReader) ReadTrustFact(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	fact := r.fact
	fact.Subjects = append([]string(nil), r.fact.Subjects...)
	return fact, nil
}
