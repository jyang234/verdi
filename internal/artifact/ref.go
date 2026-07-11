package artifact

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is a known artifact kind (02 §Kind registry).
type Kind string

const (
	KindSpec          Kind = "spec"
	KindADR           Kind = "adr"
	KindDiagram       Kind = "diagram"
	KindAttestation   Kind = "attestation"
	KindWaiver        Kind = "waiver"
	KindConflict      Kind = "conflict"
	KindReaffirmation Kind = "reaffirmation" // R4-I-4, 02 §Kind registry
)

// knownKinds is the closed set of artifact kinds. Any other value fails
// closed (CLAUDE.md: "unknown enum values fail closed").
var knownKinds = map[Kind]bool{
	KindSpec:          true,
	KindADR:           true,
	KindDiagram:       true,
	KindAttestation:   true,
	KindWaiver:        true,
	KindConflict:      true,
	KindReaffirmation: true,
}

// Valid reports whether k is one of the known kinds.
func (k Kind) Valid() bool { return knownKinds[k] }

const nameSegment = `[a-z0-9]+(?:-[a-z0-9]+)*`

var (
	// simpleNameRe matches an ordinary kebab-case name (02 §Identity:
	// "name is kebab-case, unique within its kind").
	simpleNameRe = regexp.MustCompile(`^` + nameSegment + `$`)

	// compoundNameRe matches the attestation/waiver/reaffirmation name shape
	// ratified by I-6 and extended to reaffirmation (R4-I-4):
	// "<story>--<ac-id>" / "<story>--<object-id>", a normative slug (each
	// half independently kebab-case), joined by a literal "--". The path
	// stays nested (attestations/<story>/<ac-id>.md,
	// reaffirmations/<story>/<object-id>.md, 01 §Directory layout); only
	// the ref's name field is compounded.
	compoundNameRe = regexp.MustCompile(`^` + nameSegment + `--` + nameSegment + `$`)

	// commitRe matches a short-or-full lowercase hex git commit sha.
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

	// objectIDRe matches a frontmatter-declared object or attribute's stable
	// id — "ac-1", "co-1", "dc-1", "oq-1" — a short type-prefix segment,
	// dash, then one or more kebab-case segments (02 §Object model,
	// §Identity and references: fragment refs name one of these).
	objectIDRe = regexp.MustCompile(`^[a-z][a-z0-9]*-[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// Ref is a canonical artifact reference: "<kind>/<name>", optionally pinned
// as "<kind>/<name>@<commit>" (02 §Identity and references). Either form
// gains an optional "#<object-id>" fragment suffix (R4-I-3, §Identity and
// references: "Fragment ref") naming a spec object declared in the target's
// frontmatter — the only way an edge targets an object rather than a whole
// artifact.
type Ref struct {
	Kind   Kind
	Name   string
	Commit string // empty when the ref is unpinned
	Object string // empty when the ref is not a fragment ref
}

// Pinned reports whether r carries a commit.
func (r Ref) Pinned() bool { return r.Commit != "" }

// Fragment reports whether r carries an object-id fragment.
func (r Ref) Fragment() bool { return r.Object != "" }

// String formats r back into canonical ref form. It does not validate —
// call Validate first if the caller cannot already trust r's fields.
func (r Ref) String() string {
	var b strings.Builder
	b.WriteString(string(r.Kind))
	b.WriteByte('/')
	b.WriteString(r.Name)
	if r.Pinned() {
		b.WriteByte('@')
		b.WriteString(r.Commit)
	}
	if r.Fragment() {
		b.WriteByte('#')
		b.WriteString(r.Object)
	}
	return b.String()
}

// Validate checks that r's kind is known, its name matches the shape
// required for that kind (kebab-case in general; the compound
// "<story>--<ac-id>" / "<story>--<object-id>" shape for
// attestation/waiver/reaffirmation per I-6, R4-I-4), that, if pinned, the
// commit looks like a real (short or full) git sha, and, if a fragment,
// that the object id has the expected shape (02 §Identity and references).
func (r Ref) Validate() error {
	if !r.Kind.Valid() {
		return fmt.Errorf("artifact: unknown kind %q", r.Kind)
	}
	if r.Name == "" {
		return fmt.Errorf("artifact: %s ref has an empty name", r.Kind)
	}

	switch r.Kind {
	case KindAttestation, KindWaiver, KindReaffirmation:
		if !compoundNameRe.MatchString(r.Name) {
			return fmt.Errorf("artifact: %s name %q must be <story>--<ac-id>, kebab-case on each side (I-6, R4-I-4)", r.Kind, r.Name)
		}
	default:
		if !simpleNameRe.MatchString(r.Name) {
			return fmt.Errorf("artifact: %s name %q must be kebab-case", r.Kind, r.Name)
		}
	}

	if r.Commit != "" && !commitRe.MatchString(r.Commit) {
		return fmt.Errorf("artifact: ref %s/%s has an invalid commit %q (want 7-40 lowercase hex characters)", r.Kind, r.Name, r.Commit)
	}
	if r.Object != "" && !objectIDRe.MatchString(r.Object) {
		return fmt.Errorf("artifact: ref %s/%s has an invalid fragment object id %q", r.Kind, r.Name, r.Object)
	}
	return nil
}

// ParseRef parses "kind/name", "kind/name@commit", "kind/name#object-id",
// or "kind/name@commit#object-id", and validates the result. An "@" with
// nothing after it, or a "#" with nothing after it, is a format error, not
// a silently unpinned/unfragmented ref.
func ParseRef(s string) (Ref, error) {
	base, object, hasHash := strings.Cut(s, "#")
	if hasHash && object == "" {
		return Ref{}, fmt.Errorf("artifact: ref %q has a trailing '#' with no object id", s)
	}
	if strings.Contains(object, "#") {
		return Ref{}, fmt.Errorf("artifact: ref %q has more than one '#' fragment separator", s)
	}

	kindPart, rest, ok := strings.Cut(base, "/")
	if !ok {
		return Ref{}, fmt.Errorf("artifact: ref %q is missing the '/' separating kind from name", s)
	}

	name, commit, hasAt := strings.Cut(rest, "@")
	if hasAt && commit == "" {
		return Ref{}, fmt.Errorf("artifact: ref %q has a trailing '@' with no commit", s)
	}

	r := Ref{Kind: Kind(kindPart), Name: name, Commit: commit, Object: object}
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
