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

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyauthority"
)

const proofDigestDomain = "verdi.experiment-human-proof/v1\x00"

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
	if challenge != expected {
		return verdict(governanceprincipal.ResolutionViolated, ReasonHumanChallengeStale), nil
	}
	if signature == nil || len(signature) == 0 {
		return verdict(governanceprincipal.ResolutionUnproven, ReasonHumanProofMissing), nil
	}
	if len(signature) != ed25519.SignatureSize {
		return Verification{}, fmt.Errorf("experimenthuman: detached Ed25519 signature is %d bytes, want %d", len(signature), ed25519.SignatureSize)
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
	return Verification{State: resolution.State, Code: ReasonHumanProofVerified, Resolution: resolution}, nil
}

func verdict(state governanceprincipal.ResolutionState, code string) Verification {
	return Verification{State: state, Code: code}
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
