package artifact

import "fmt"

// SupersessionNote is one `{id, note}` entry in a feature spec's
// `supersession:` block — used by the amended/amended_advisory/removed
// buckets, each of which requires a reason (03 §The amendment ladder).
type SupersessionNote struct {
	ID   string `yaml:"id"`
	Note string `yaml:"note"`
}

// Supersession is a superseding feature spec revision's structured object
// manifest (02 §Kind registry, R4-I-4; 03 §The amendment ladder rung 4):
// classifies every predecessor object exactly once across
// carried/amended/amended_advisory/removed, plus added for wholly new
// objects. This package validates entry shape only — completeness
// (every predecessor object classified exactly once) and carried-content
// byte-identity are VL-015's job (V1-P2), not decoded/checked here per this
// phase's "pure types" posture.
type Supersession struct {
	Carried         []string           `yaml:"carried,omitempty"`
	Amended         []SupersessionNote `yaml:"amended,omitempty"`
	AmendedAdvisory []SupersessionNote `yaml:"amended_advisory,omitempty"`
	Removed         []SupersessionNote `yaml:"removed,omitempty"`
	Added           []string           `yaml:"added,omitempty"`
}

// Validate checks every id (in every bucket) looks like a real object id
// and every note-carrying entry (amended/amended_advisory/removed) carries
// a non-empty note.
func (s Supersession) Validate() error {
	for i, id := range s.Carried {
		if !objectIDRe.MatchString(id) {
			return fmt.Errorf("carried[%d]: %q is not a valid object id", i, id)
		}
	}
	for i, n := range s.Amended {
		if err := n.validate(); err != nil {
			return fmt.Errorf("amended[%d]: %w", i, err)
		}
	}
	for i, n := range s.AmendedAdvisory {
		if err := n.validate(); err != nil {
			return fmt.Errorf("amended_advisory[%d]: %w", i, err)
		}
	}
	for i, n := range s.Removed {
		if err := n.validate(); err != nil {
			return fmt.Errorf("removed[%d]: %w", i, err)
		}
	}
	for i, id := range s.Added {
		if !objectIDRe.MatchString(id) {
			return fmt.Errorf("added[%d]: %q is not a valid object id", i, id)
		}
	}
	return nil
}

func (n SupersessionNote) validate() error {
	if !objectIDRe.MatchString(n.ID) {
		return fmt.Errorf("%q is not a valid object id", n.ID)
	}
	if n.Note == "" {
		return fmt.Errorf("entry %q has no note", n.ID)
	}
	return nil
}
