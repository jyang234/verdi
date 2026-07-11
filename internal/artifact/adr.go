package artifact

import "fmt"

// ADRFrontmatter is the frontmatter schema for kind "adr"
// (02 §Kind registry: proposed → accepted → superseded, frozen at
// acceptance). `decided` is the load-bearing acceptance-moment stamp
// 02 §Identity carries as an exception to "no created/updated dates".
type ADRFrontmatter struct {
	Base    `yaml:",inline"`
	Status  Status `yaml:"status"`
	Decided string `yaml:"decided,omitempty"`
}

// DecodeADR strict-decodes and validates ADR frontmatter.
func DecodeADR(data []byte) (*ADRFrontmatter, error) {
	var fm ADRFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, the status enum, that Decided is
// present iff the ADR has been accepted (proposed→accepted→superseded),
// and that Frozen is present iff the temporal class requires it (frozen at
// acceptance: required once accepted or superseded, absent while
// proposed).
func (fm ADRFrontmatter) Validate() error {
	if err := fm.validateBase(KindADR); err != nil {
		return err
	}
	if !adrStatuses[fm.Status] {
		return fmt.Errorf("artifact: adr status %q is not a known status", fm.Status)
	}

	decided := fm.Status == "accepted" || fm.Status == "superseded"
	if decided && fm.Decided == "" {
		return fmt.Errorf("artifact: adr status %q requires a decided stamp", fm.Status)
	}
	if !decided && fm.Decided != "" {
		return fmt.Errorf("artifact: adr status %q must not carry a decided stamp", fm.Status)
	}
	if fm.Decided != "" && !dateRe.MatchString(fm.Decided) {
		return fmt.Errorf("artifact: adr decided %q is not a YYYY-MM-DD date", fm.Decided)
	}

	if err := requireFrozen(fm.Frozen, decided, "adr", string(fm.Status)); err != nil {
		return err
	}
	return nil
}

// requireFrozen enforces 01 §Temporal classes' "no third state" rule for a
// single kind/status combination: frozen must be present exactly when
// required, never present when not.
func requireFrozen(frozen *Frozen, required bool, kind, status string) error {
	if required && frozen == nil {
		return fmt.Errorf("artifact: %s status %q requires a frozen stamp", kind, status)
	}
	if !required && frozen != nil {
		return fmt.Errorf("artifact: %s status %q must not carry a frozen stamp", kind, status)
	}
	return nil
}
