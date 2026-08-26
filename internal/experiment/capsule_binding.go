// Capsule binding (Wave 5C Task 9; design §§5, 9; AC-4, DC-8, SI-141,
// SI-146): the closed, deterministically derived retained inventory for a
// selecting ratification. The artifact-id vocabulary below is the ONE
// owner of the closed mapping: every id satisfies the existing capsule
// grammar (CapsuleArtifact.Validate → ValidateID: lowercase alphanumeric
// segments joined by single hyphens — the only namespace punctuation the
// ratified capsule protocol admits), and fixture ids are namespaced by
// the registered fixture id. The input-binding SLOT spelling
// `fixture:<id>` (design §8, SI-148) is a different, wire-only grammar
// and is deliberately not reused here: a colon is not a legal capsule
// artifact id.
package experiment

import (
	"errors"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
)

// The closed capsule artifact ids (design §9's retained set, in its
// order): each non-fixture member has exactly one fixed id; fixtures use
// CapsuleFixtureArtifactID.
const (
	CapsuleArtifactDefinition            = "definition"
	CapsuleArtifactCandidatePatch        = "candidate-patch"
	CapsuleArtifactEvaluatorCapabilities = "evaluator-capabilities"
	CapsuleArtifactContract              = "contract"
	CapsuleArtifactWorkload              = "workload"
	CapsuleArtifactExecutionReceipt      = "execution-receipt"
	CapsuleArtifactObservations          = "observations"
	CapsuleArtifactResult                = "result"
	CapsuleArtifactRatification          = "ratification"
	CapsuleArtifactRecommendation        = "recommendation"
)

// capsuleFixturePrefix namespaces retained fixtures by registered id.
const capsuleFixturePrefix = "fixture-"

// CapsuleFixtureArtifactID derives the namespaced capsule artifact id for
// one registered fixture.
func CapsuleFixtureArtifactID(fixtureID string) (string, error) {
	if err := ValidateID(fixtureID); err != nil {
		return "", fmt.Errorf("experiment: capsule fixture id: %w", err)
	}
	return capsuleFixturePrefix + fixtureID, nil
}

// SelectedCapsuleCandidate derives the selected candidate for a selecting
// ratification (design §9): the bound result's proven winner for
// select-recommended, or the ratification's explicit registered candidate
// for select-other. Reproduction posture is a status fact only and never
// participates. Non-selecting dispositions select nothing.
func SelectedCapsuleCandidate(def Definition, res Result, r Ratification) (string, error) {
	if err := ValidateRatificationBinding(def, res, r); err != nil {
		return "", err
	}
	switch r.Disposition {
	case DispositionSelectRecommended:
		if res.Schema != ResultSchemaV2 || res.Decision == nil {
			return "", fmt.Errorf("experiment: capsule selection requires a v2 result with its decision document")
		}
		if res.Decision.Winner == "" {
			return "", fmt.Errorf("experiment: %q requires a proven winner in the bound result", DispositionSelectRecommended)
		}
		return res.Decision.Winner, nil
	case DispositionSelectOther:
		return r.Candidate, nil
	}
	return "", fmt.Errorf("experiment: disposition %q does not select a candidate and binds no capsule", r.Disposition)
}

// CapsuleRetainedArtifact is one raw retained artifact presented for
// binding, identified by its closed capsule artifact id.
type CapsuleRetainedArtifact struct {
	ID    string
	Bytes []byte
}

// CapsuleBindingInput is the complete typed context BindCapsuleManifest
// judges: the locked definition, its canonical digest, the ratification
// and its bound result, every retained raw artifact, and the sealed
// effective per-artifact retention ceiling.
type CapsuleBindingInput struct {
	Definition            Definition
	DefinitionDigest      string
	Ratification          Ratification
	Result                Result
	Artifacts             []CapsuleRetainedArtifact
	RetainedArtifactBytes int64
}

// errCapsuleArtifactOversized marks the typed per-artifact retention
// refusal so callers can classify it as a policy verdict rather than an
// operational inventory failure.
var errCapsuleArtifactOversized = errors.New("retained artifact exceeds the effective retained_artifact_bytes ceiling")

// IsCapsuleArtifactOversized reports whether err is the typed
// retained-artifact ceiling refusal.
func IsCapsuleArtifactOversized(err error) bool {
	return errors.Is(err, errCapsuleArtifactOversized)
}

// BindCapsuleManifest builds the deterministic capsule manifest for a
// selecting ratification from the exact closed retained set. It refuses
// missing, extra, or duplicate inventory members, recomputes every
// artifact digest from exact raw bytes, requires the definition-,
// result-, and ratification-bound digests to match where the references
// exist, and applies the retention ceiling independently to every member
// (equality passes; one byte over fails naming the exact artifact and
// observed size). The manifest itself is not an inventory member and is
// never measured against the ceiling.
func BindCapsuleManifest(in CapsuleBindingInput) (CapsuleManifest, error) {
	selected, err := SelectedCapsuleCandidate(in.Definition, in.Result, in.Ratification)
	if err != nil {
		return CapsuleManifest{}, err
	}
	if err := ValidateDigest(in.DefinitionDigest); err != nil {
		return CapsuleManifest{}, fmt.Errorf("experiment: capsule definition digest: %w", err)
	}
	if in.RetainedArtifactBytes <= 0 {
		return CapsuleManifest{}, fmt.Errorf("experiment: capsule retained_artifact_bytes ceiling must be positive, got %d", in.RetainedArtifactBytes)
	}

	var selectedCandidate *Candidate
	for i := range in.Definition.Candidates {
		if in.Definition.Candidates[i].ID == selected {
			selectedCandidate = &in.Definition.Candidates[i]
			break
		}
	}
	if selectedCandidate == nil {
		return CapsuleManifest{}, fmt.Errorf("experiment: selected candidate %q is not registered by the definition", selected)
	}

	// required maps each closed inventory id to the digest the retained
	// bytes must recompute to; "" means the raw digest itself is the
	// binding (recorded, not pre-constrained).
	required := map[string]string{
		CapsuleArtifactDefinition:            "",
		CapsuleArtifactCandidatePatch:        selectedCandidate.Digest,
		CapsuleArtifactEvaluatorCapabilities: in.Definition.Evaluator.CapabilitiesDigest,
		CapsuleArtifactContract:              in.Definition.Contract.Digest,
		CapsuleArtifactWorkload:              in.Definition.Workload.Digest,
		CapsuleArtifactExecutionReceipt:      "",
		CapsuleArtifactObservations:          "",
		CapsuleArtifactResult:                in.Ratification.ResultDigest,
		CapsuleArtifactRatification:          "",
	}
	for _, fixture := range in.Definition.Fixtures {
		id, err := CapsuleFixtureArtifactID(fixture.ID)
		if err != nil {
			return CapsuleManifest{}, err
		}
		required[id] = fixture.Digest
	}
	optional := map[string]bool{CapsuleArtifactRecommendation: true}

	presented := make(map[string][]byte, len(in.Artifacts))
	for _, artifact := range in.Artifacts {
		if _, duplicate := presented[artifact.ID]; duplicate {
			return CapsuleManifest{}, fmt.Errorf("experiment: capsule inventory presents duplicate artifact %q", artifact.ID)
		}
		if _, known := required[artifact.ID]; !known && !optional[artifact.ID] {
			return CapsuleManifest{}, fmt.Errorf("experiment: capsule inventory presents %q, which is outside the closed retained set", artifact.ID)
		}
		presented[artifact.ID] = artifact.Bytes
	}
	for id := range required {
		if _, ok := presented[id]; !ok {
			return CapsuleManifest{}, fmt.Errorf("experiment: capsule inventory is missing required artifact %q", id)
		}
	}

	artifacts := make([]CapsuleArtifact, 0, len(presented))
	for id, data := range presented {
		size := int64(len(data))
		if size > in.RetainedArtifactBytes {
			return CapsuleManifest{}, fmt.Errorf("experiment: capsule artifact %q is %d bytes, exceeding retained_artifact_bytes %d: %w",
				id, size, in.RetainedArtifactBytes, errCapsuleArtifactOversized)
		}
		digest := rawSHA256Digest(data)
		if want := required[id]; want != "" && digest != want {
			return CapsuleManifest{}, fmt.Errorf("experiment: capsule artifact %q raw digest %s does not match its bound reference %s", id, digest, want)
		}
		artifacts = append(artifacts, CapsuleArtifact{ID: id, Digest: digest})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].ID < artifacts[j].ID })

	manifest := CapsuleManifest{
		Schema: CapsuleManifestSchema, Experiment: in.Definition.ID,
		DefinitionDigest: in.DefinitionDigest, ResultDigest: in.Ratification.ResultDigest,
		Selected: selected, Artifacts: artifacts,
	}
	if err := manifest.Validate(); err != nil {
		return CapsuleManifest{}, err
	}
	return manifest, nil
}

// EncodeCapsuleManifest renders the exact deterministic canonical bytes
// for a valid manifest.
func EncodeCapsuleManifest(m CapsuleManifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	data, err := canonjson.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("experiment: encoding capsule manifest: %w", err)
	}
	return data, nil
}

// rawSHA256Digest is the exact raw-byte digest every retained artifact is
// recomputed with (the same sha256Digest the state algorithm uses).
func rawSHA256Digest(data []byte) string { return sha256Digest(data) }
