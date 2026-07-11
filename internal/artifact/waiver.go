package artifact

import "fmt"

// WaiverFrontmatter is the frontmatter schema for kind "waiver"
// (02 §Kind registry: active → expired, frozen at commit always;
// VL-011: "waiver has owner + reason, expiry optional"). Owner reuses the
// common `owners:` field; Reason and Expiry are waiver-specific.
type WaiverFrontmatter struct {
	Base   `yaml:",inline"`
	Status Status `yaml:"status"`
	Reason string `yaml:"reason"`
	Expiry string `yaml:"expiry,omitempty"`
}

// DecodeWaiver strict-decodes and validates waiver frontmatter.
func DecodeWaiver(data []byte) (*WaiverFrontmatter, error) {
	var fm WaiverFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, the status enum, Reason is present,
// Expiry (if present) is a date, and Frozen is always present.
func (fm WaiverFrontmatter) Validate() error {
	if err := fm.validateBase(KindWaiver); err != nil {
		return err
	}
	if !waiverStatuses[fm.Status] {
		return fmt.Errorf("artifact: waiver status %q is not a known status", fm.Status)
	}
	if fm.Reason == "" {
		return fmt.Errorf("artifact: waiver reason is required (VL-011)")
	}
	if fm.Expiry != "" && !dateRe.MatchString(fm.Expiry) {
		return fmt.Errorf("artifact: waiver expiry %q is not a YYYY-MM-DD date", fm.Expiry)
	}
	return requireFrozen(fm.Frozen, true, "waiver", string(fm.Status))
}
