package experiment

import "fmt"

// CapsuleManifestSchema is the only accepted capsule-manifest.json schema
// identifier.
const CapsuleManifestSchema = "verdi.experiment-capsule/v1"

// CapsuleArtifact is one retained, content-addressed artifact a capsule
// manifest names.
type CapsuleArtifact struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// Validate checks a's id and digest.
func (a CapsuleArtifact) Validate() error {
	if err := ValidateID(a.ID); err != nil {
		return fmt.Errorf("experiment: capsule.artifacts: %w", err)
	}
	if err := ValidateDigest(a.Digest); err != nil {
		return fmt.Errorf("experiment: capsule artifact %q: digest: %w", a.ID, err)
	}
	return nil
}

// CapsuleManifest is one verdi.experiment-capsule/v1 record (AC-4,
// DC-8): the sealed complete reproduction set for the selected candidate.
type CapsuleManifest struct {
	Schema           string            `json:"schema"`
	Experiment       string            `json:"experiment"`
	DefinitionDigest string            `json:"definition_digest"`
	ResultDigest     string            `json:"result_digest"`
	Selected         string            `json:"selected"`
	Artifacts        []CapsuleArtifact `json:"artifacts"`
}

// DecodeCapsuleManifest strict-decodes raw as a capsule-manifest.json
// document and fully validates it (decodeStrictJSON: the shared strict
// seam plus this package's duplicate-key guard).
func DecodeCapsuleManifest(raw []byte) (CapsuleManifest, error) {
	var m CapsuleManifest
	if err := decodeStrictJSON(raw, &m); err != nil {
		return CapsuleManifest{}, fmt.Errorf("experiment: decoding capsule manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return CapsuleManifest{}, err
	}
	return m, nil
}

// Validate checks every field's grammar, that artifacts is nonempty with
// unique ids.
func (m CapsuleManifest) Validate() error {
	if m.Schema != CapsuleManifestSchema {
		return fmt.Errorf("experiment: unknown capsule manifest schema %q, want %q", m.Schema, CapsuleManifestSchema)
	}
	if err := ValidateID(m.Experiment); err != nil {
		return fmt.Errorf("experiment: capsule.experiment: %w", err)
	}
	if err := ValidateDigest(m.DefinitionDigest); err != nil {
		return fmt.Errorf("experiment: capsule.definition_digest: %w", err)
	}
	if err := ValidateDigest(m.ResultDigest); err != nil {
		return fmt.Errorf("experiment: capsule.result_digest: %w", err)
	}
	if err := ValidateID(m.Selected); err != nil {
		return fmt.Errorf("experiment: capsule.selected: %w", err)
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("experiment: capsule.artifacts must be nonempty")
	}
	seen := make(map[string]bool, len(m.Artifacts))
	for i, a := range m.Artifacts {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("experiment: artifacts[%d]: %w", i, err)
		}
		if seen[a.ID] {
			return fmt.Errorf("experiment: capsule.artifacts: duplicate id %q", a.ID)
		}
		seen[a.ID] = true
	}
	return nil
}
