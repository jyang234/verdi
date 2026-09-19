package governanceprincipal

import (
	"fmt"
	"regexp"
)

var sha256Re = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// TemplateRecord is the resolved-scaffold provenance a created human
// artifact records (AC-1: "A created artifact records the resolved
// template identity and digest"). Identity is the shared scaffold
// resolver's source identity (internal/humanartifact); Digest is the
// sha256 of the resolved template bytes.
//
// Declared here, below policyartifact, so the governance profile can
// carry the same record; policyartifact aliases it.
type TemplateRecord struct {
	Identity string `yaml:"identity" json:"identity"`
	Digest   string `yaml:"digest" json:"digest"`
}

// Validate checks the record's grammar.
func (t TemplateRecord) Validate() error {
	if t.Identity == "" {
		return fmt.Errorf("governanceprincipal: template.identity is required")
	}
	if !sha256Re.MatchString(t.Digest) {
		return fmt.Errorf("governanceprincipal: template.digest %q is not sha256:<64 hex> form", t.Digest)
	}
	return nil
}
