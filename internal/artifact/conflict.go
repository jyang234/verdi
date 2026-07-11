package artifact

import "fmt"

// ConflictFrontmatter is the frontmatter schema for kind "conflict"
// (02 §Kind registry: open → superseded | dismissed, frozen at
// resolution; 03 §Challenging closed decisions: "File a conflict:
// .verdi/conflicts/<name>.md with a challenges: link to the disputed
// artifact").
type ConflictFrontmatter struct {
	Base   `yaml:",inline"`
	Status Status `yaml:"status"`
}

// DecodeConflict strict-decodes and validates conflict frontmatter.
func DecodeConflict(data []byte) (*ConflictFrontmatter, error) {
	var fm ConflictFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, the status enum, that at least one
// `challenges` link is present (filing is mandatory per 03 §Challenging
// closed decisions), and that Frozen is present iff the conflict is
// resolved (superseded or dismissed) — open conflicts are not yet frozen.
func (fm ConflictFrontmatter) Validate() error {
	if err := fm.validateBase(KindConflict); err != nil {
		return err
	}
	if !conflictStatuses[fm.Status] {
		return fmt.Errorf("artifact: conflict status %q is not a known status", fm.Status)
	}

	hasChallenges := false
	for _, l := range fm.Links {
		if l.Type == LinkChallenges {
			hasChallenges = true
			break
		}
	}
	if !hasChallenges {
		return fmt.Errorf("artifact: conflict must carry at least one 'challenges' link (03 §Challenging closed decisions)")
	}

	resolved := fm.Status == "superseded" || fm.Status == "dismissed"
	return requireFrozen(fm.Frozen, resolved, "conflict", string(fm.Status))
}
