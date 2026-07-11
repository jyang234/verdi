package artifact

// AttestationFrontmatter is the frontmatter schema for kind "attestation"
// (02 §Kind registry: "(none — existence is the record)" — no status
// field at all; frozen at commit, always). The struct deliberately has no
// Status field: a `status:` key present in an attestation's frontmatter is
// an unknown field and fails strict decode, matching the spec's "existence
// is the record" reading.
type AttestationFrontmatter struct {
	Base `yaml:",inline"`
}

// DecodeAttestation strict-decodes and validates attestation frontmatter.
func DecodeAttestation(data []byte) (*AttestationFrontmatter, error) {
	var fm AttestationFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields and that Frozen is always present
// (attestations are frozen at commit, unconditionally).
func (fm AttestationFrontmatter) Validate() error {
	if err := fm.validateBase(KindAttestation); err != nil {
		return err
	}
	return requireFrozen(fm.Frozen, true, "attestation", "")
}
