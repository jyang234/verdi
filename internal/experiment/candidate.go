package experiment

import "fmt"

// Candidate is one registered candidate patch entry inside a Definition's
// candidates list (AC-1).
type Candidate struct {
	ID     string `yaml:"id" json:"id"`
	Patch  string `yaml:"patch" json:"patch"`
	Digest string `yaml:"digest" json:"digest"`
	Base   string `yaml:"base" json:"base"`
}

// Validate checks c's own grammar and its relationship to the enclosing
// definition's base_commit: the patch path must be exactly
// "candidates/<id>.patch", and the candidate's base must equal baseCommit
// — a differing base names both values so the mismatch is legible.
func (c Candidate) Validate(baseCommit string) error {
	if err := ValidateID(c.ID); err != nil {
		return fmt.Errorf("experiment: candidate id: %w", err)
	}
	wantPatch := "candidates/" + c.ID + ".patch"
	if c.Patch != wantPatch {
		return fmt.Errorf("experiment: candidate %q: patch %q, want %q", c.ID, c.Patch, wantPatch)
	}
	if err := ValidateDigest(c.Digest); err != nil {
		return fmt.Errorf("experiment: candidate %q: digest: %w", c.ID, err)
	}
	if err := ValidateCommit(c.Base); err != nil {
		return fmt.Errorf("experiment: candidate %q: base: %w", c.ID, err)
	}
	if c.Base != baseCommit {
		return fmt.Errorf("experiment: candidate %q: base %q does not match definition base_commit %q", c.ID, c.Base, baseCommit)
	}
	return nil
}
