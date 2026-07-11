package artifact

import "fmt"

// DiagramFrontmatter is the frontmatter schema for kind "diagram"
// (02 §Kind registry: active → superseded, authored-living — never
// frozen).
type DiagramFrontmatter struct {
	Base   `yaml:",inline"`
	Status Status `yaml:"status"`
}

// DecodeDiagram strict-decodes and validates diagram frontmatter.
func DecodeDiagram(data []byte) (*DiagramFrontmatter, error) {
	var fm DiagramFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, the status enum, and that Frozen is
// absent (authored-living artifacts are never frozen).
func (fm DiagramFrontmatter) Validate() error {
	if err := fm.validateBase(KindDiagram); err != nil {
		return err
	}
	if !diagramStatuses[fm.Status] {
		return fmt.Errorf("artifact: diagram status %q is not a known status", fm.Status)
	}
	return requireFrozen(fm.Frozen, false, "diagram", string(fm.Status))
}
