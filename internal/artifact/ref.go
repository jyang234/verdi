package artifact

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is a known artifact kind (02 §Kind registry).
type Kind string

const (
	KindSpec        Kind = "spec"
	KindADR         Kind = "adr"
	KindDiagram     Kind = "diagram"
	KindAttestation Kind = "attestation"
	KindWaiver      Kind = "waiver"
	KindConflict    Kind = "conflict"
)

// knownKinds is the closed set of artifact kinds. Any other value fails
// closed (CLAUDE.md: "unknown enum values fail closed").
var knownKinds = map[Kind]bool{
	KindSpec:        true,
	KindADR:         true,
	KindDiagram:     true,
	KindAttestation: true,
	KindWaiver:      true,
	KindConflict:    true,
}

// Valid reports whether k is one of the known kinds.
func (k Kind) Valid() bool { return knownKinds[k] }

const nameSegment = `[a-z0-9]+(?:-[a-z0-9]+)*`

var (
	// simpleNameRe matches an ordinary kebab-case name (02 §Identity:
	// "name is kebab-case, unique within its kind").
	simpleNameRe = regexp.MustCompile(`^` + nameSegment + `$`)

	// compoundNameRe matches the attestation/waiver name shape ratified by
	// I-6: "<story>--<ac-id>", a normative slug (each half independently
	// kebab-case), joined by a literal "--". The path stays nested
	// (attestations/<story>/<ac-id>.md, 01 §Directory layout); only the
	// ref's name field is compounded.
	compoundNameRe = regexp.MustCompile(`^` + nameSegment + `--` + nameSegment + `$`)

	// commitRe matches a short-or-full lowercase hex git commit sha.
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// Ref is a canonical artifact reference: "<kind>/<name>", optionally pinned
// as "<kind>/<name>@<commit>" (02 §Identity and references).
type Ref struct {
	Kind   Kind
	Name   string
	Commit string // empty when the ref is unpinned
}

// Pinned reports whether r carries a commit.
func (r Ref) Pinned() bool { return r.Commit != "" }

// String formats r back into canonical ref form. It does not validate —
// call Validate first if the caller cannot already trust r's fields.
func (r Ref) String() string {
	if r.Pinned() {
		return fmt.Sprintf("%s/%s@%s", r.Kind, r.Name, r.Commit)
	}
	return fmt.Sprintf("%s/%s", r.Kind, r.Name)
}

// Validate checks that r's kind is known, its name matches the shape
// required for that kind (kebab-case in general; the compound
// "<story>--<ac-id>" shape for attestation/waiver per I-6), and, if
// pinned, that the commit looks like a real (short or full) git sha.
func (r Ref) Validate() error {
	if !r.Kind.Valid() {
		return fmt.Errorf("artifact: unknown kind %q", r.Kind)
	}
	if r.Name == "" {
		return fmt.Errorf("artifact: %s ref has an empty name", r.Kind)
	}

	switch r.Kind {
	case KindAttestation, KindWaiver:
		if !compoundNameRe.MatchString(r.Name) {
			return fmt.Errorf("artifact: %s name %q must be <story>--<ac-id>, kebab-case on each side (I-6)", r.Kind, r.Name)
		}
	default:
		if !simpleNameRe.MatchString(r.Name) {
			return fmt.Errorf("artifact: %s name %q must be kebab-case", r.Kind, r.Name)
		}
	}

	if r.Commit != "" && !commitRe.MatchString(r.Commit) {
		return fmt.Errorf("artifact: ref %s/%s has an invalid commit %q (want 7-40 lowercase hex characters)", r.Kind, r.Name, r.Commit)
	}
	return nil
}

// ParseRef parses "kind/name" or "kind/name@commit" and validates the
// result. An "@" with nothing after it is a format error, not a silently
// unpinned ref.
func ParseRef(s string) (Ref, error) {
	kindPart, rest, ok := strings.Cut(s, "/")
	if !ok {
		return Ref{}, fmt.Errorf("artifact: ref %q is missing the '/' separating kind from name", s)
	}

	name, commit, hasAt := strings.Cut(rest, "@")
	if hasAt && commit == "" {
		return Ref{}, fmt.Errorf("artifact: ref %q has a trailing '@' with no commit", s)
	}

	r := Ref{Kind: Kind(kindPart), Name: name, Commit: commit}
	if err := r.Validate(); err != nil {
		return Ref{}, err
	}
	return r, nil
}

// ParsePinnedRef is ParseRef, additionally requiring the ref to be pinned
// (02 §Identity: "the only form permitted in context manifests, evidence
// records, and board pins").
func ParsePinnedRef(s string) (Ref, error) {
	r, err := ParseRef(s)
	if err != nil {
		return Ref{}, err
	}
	if !r.Pinned() {
		return Ref{}, fmt.Errorf("artifact: ref %q must be pinned (kind/name@commit)", s)
	}
	return r, nil
}
