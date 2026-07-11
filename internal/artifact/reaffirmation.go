package artifact

import "fmt"

// HashPair is a re-affirmation's `hash: { old, new }` block: the
// (kind, id, text) content hash (ObjectContentHash) of the amended object
// before and after the feature supersession that triggered it
// (03 §The amendment ladder; 02 §Record schemas).
type HashPair struct {
	Old string `yaml:"old"`
	New string `yaml:"new"`
}

// Validate checks both Old and New are sha256:<hex> hashes and are
// distinct — a re-affirmation records a diff, so an unchanged hash pair
// would mean the object it names was never amended at all.
func (hp HashPair) Validate() error {
	if !sha256Re.MatchString(hp.Old) {
		return fmt.Errorf("artifact: hash.old %q is not sha256:<64 hex> form", hp.Old)
	}
	if !sha256Re.MatchString(hp.New) {
		return fmt.Errorf("artifact: hash.new %q is not sha256:<64 hex> form", hp.New)
	}
	if hp.Old == hp.New {
		return fmt.Errorf("artifact: hash.old and hash.new are identical %q — a re-affirmation records a content change", hp.Old)
	}
	return nil
}

// ReaffirmationFrontmatter is the frontmatter schema for kind
// "reaffirmation" (02 §Kind registry: "(none — existence is the record)" —
// no status field, same posture as attestation; frozen at commit,
// unconditionally; 03 §The amendment ladder rung 4, R4-I-4). Object is the
// pinned fragment ref to the amended feature object at the superseding
// revision's commit (e.g. "spec/loan-update@<v2-commit>#ac-2").
type ReaffirmationFrontmatter struct {
	Base   `yaml:",inline"`
	Object string   `yaml:"object"`
	Hash   HashPair `yaml:"hash"`
}

// DecodeReaffirmation strict-decodes and validates reaffirmation frontmatter.
func DecodeReaffirmation(data []byte) (*ReaffirmationFrontmatter, error) {
	var fm ReaffirmationFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, that Object is a pinned fragment ref
// to a spec object, that Hash is well-formed, and that Frozen is always
// present (frozen at commit, unconditionally).
func (fm ReaffirmationFrontmatter) Validate() error {
	if err := fm.validateBase(KindReaffirmation); err != nil {
		return err
	}
	ref, err := ParseRef(fm.Object)
	if err != nil {
		return fmt.Errorf("artifact: reaffirmation object: %w", err)
	}
	if !ref.Pinned() {
		return fmt.Errorf("artifact: reaffirmation object %q must be pinned (kind/name@commit#object-id)", fm.Object)
	}
	if !ref.Fragment() {
		return fmt.Errorf("artifact: reaffirmation object %q must carry an object-id fragment", fm.Object)
	}
	if err := fm.Hash.Validate(); err != nil {
		return err
	}
	return requireFrozen(fm.Frozen, true, "reaffirmation", "")
}
