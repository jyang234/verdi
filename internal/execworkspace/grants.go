package execworkspace

// Grant vocabulary and strict canonical-JSON codec for
// spec/execution-workspace §Execution-grant enforcement (ledger SI-12,
// controller decision AD-7). OD-7 ratifies THAT CI/CSE execution grants
// come from one shared strict vocabulary; SI-12 ratifies WHAT that
// vocabulary contains — exactly six kinds, closed, unknown kinds fail
// closed. This file mirrors sidecar.go's canonical-bytes decode gate: a
// GrantSet is accepted only from the exact canonical JSON bytes this
// package would itself write for the decoded document.

import (
	"bytes"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// grantSchema is GrantSet's canonical-JSON schema tag (AD-7).
const grantSchema = "verdi.execution-grants/v1"

// GrantKind is the closed six-member execution-grant vocabulary ratified by
// ledger SI-12 (handle L-6): network, path-read scopes, path-write scopes,
// process execution, resource ceilings, and timeouts. Declared in the
// spec's own listed order; a future value added to this block picks up a
// matching grantKindNames entry or the package fails to BUILD (mirrors
// internal/reclaim's keptReasonNames compile-time-exhaustive pattern) —
// never a silently blank or generic label, and DecodeGrantSet fails closed
// on any kind string outside this set (CLAUDE.md: "unknown enum values
// fail closed").
type GrantKind int

const (
	// GrantNetwork requests network access. Its payload is empty — presence
	// of the grant IS the request, per the spec's minimal-payload choice.
	GrantNetwork GrantKind = iota
	// GrantPathRead requests read access scoped to a non-empty list of
	// paths.
	GrantPathRead
	// GrantPathWrite requests write access scoped to a non-empty list of
	// paths.
	GrantPathWrite
	// GrantProcessExecution requests permission to execute a non-empty
	// allowlist of program names/paths (argv[0] values).
	GrantProcessExecution
	// GrantResourceCeilings requests a non-empty set of named integer
	// resource ceilings.
	GrantResourceCeilings
	// GrantTimeouts requests a positive deadline, in seconds.
	GrantTimeouts
	// numGrantKinds is the sentinel: always one past the last real value
	// (iota-tracked, never hand-counted).
	numGrantKinds
)

// grantKindNames is GrantKind's own compile-time exhaustiveness check. The
// right-hand side is an ellipsis-sized array literal whose inferred type
// only matches the declared [numGrantKinds]string array type when every
// GrantKind value up to numGrantKinds-1 has its own keyed entry: appending
// a new GrantKind above numGrantKinds without a matching entry here is a
// genuine build failure ("cannot use ... as [N]string value"), not a
// silently blank label at runtime (internal/reclaim's keptReasonNames
// pattern, mirrored exactly).
var grantKindNames [numGrantKinds]string = [...]string{
	GrantNetwork:          "network",
	GrantPathRead:         "path-read",
	GrantPathWrite:        "path-write",
	GrantProcessExecution: "process-execution",
	GrantResourceCeilings: "resource-ceilings",
	GrantTimeouts:         "timeouts",
}

// String renders k's closed-vocabulary label, or a self-naming "unknown"
// fallback for a value outside the closed set — never a blank or generic
// label.
func (k GrantKind) String() string {
	if k < 0 || int(k) >= len(grantKindNames) {
		return fmt.Sprintf("unknown-grant-kind(%d)", int(k))
	}
	return grantKindNames[k]
}

// grantKindByName inverts grantKindNames for strict decode. ok is false for
// any string outside the closed six — DecodeGrantSet's fail-closed gate for
// an unknown kind string.
func grantKindByName(name string) (GrantKind, bool) {
	for k, n := range grantKindNames {
		if n == name {
			return GrantKind(k), true
		}
	}
	return 0, false
}

// Grant is one decoded, domain-typed execution grant. Only the field(s)
// relevant to Kind are ever populated; Validate rejects a Grant carrying a
// field that does not belong to its own Kind, so a Grant's Kind always
// fully determines which of its fields are meaningful.
type Grant struct {
	Kind GrantKind
	// Paths is GrantPathRead/GrantPathWrite's non-empty scope list; each
	// entry must be non-empty.
	Paths []string
	// Argv0s is GrantProcessExecution's non-empty program allowlist; each
	// entry must be non-empty.
	Argv0s []string
	// Ceilings is GrantResourceCeilings' non-empty named-integer-ceiling
	// set; each key must be non-empty.
	Ceilings map[string]int
	// Seconds is GrantTimeouts' deadline; must be > 0.
	Seconds int
}

// Validate reports whether g is a well-formed grant: Kind is one of the
// closed six, only the field(s) belonging to Kind are populated, and the
// populated field(s) satisfy their own non-empty/positive rule. Every
// producer of a Grant — grantFromDoc on the decode side, a caller building
// one to pass to EncodeGrantSet — passes through this gate.
func (g Grant) Validate() error {
	if err := g.extraFieldsError(); err != nil {
		return err
	}
	switch g.Kind {
	case GrantNetwork:
		return nil
	case GrantPathRead, GrantPathWrite:
		if len(g.Paths) == 0 {
			return fmt.Errorf("execworkspace: grant %s: paths must be non-empty", g.Kind)
		}
		for i, p := range g.Paths {
			if p == "" {
				return fmt.Errorf("execworkspace: grant %s: paths[%d] is empty", g.Kind, i)
			}
		}
		return nil
	case GrantProcessExecution:
		if len(g.Argv0s) == 0 {
			return fmt.Errorf("execworkspace: grant %s: argv0s must be non-empty", g.Kind)
		}
		for i, a := range g.Argv0s {
			if a == "" {
				return fmt.Errorf("execworkspace: grant %s: argv0s[%d] is empty", g.Kind, i)
			}
		}
		return nil
	case GrantResourceCeilings:
		if len(g.Ceilings) == 0 {
			return fmt.Errorf("execworkspace: grant %s: ceilings must be non-empty", g.Kind)
		}
		for name := range g.Ceilings {
			if name == "" {
				return fmt.Errorf("execworkspace: grant %s: ceiling name is empty", g.Kind)
			}
		}
		return nil
	case GrantTimeouts:
		if g.Seconds <= 0 {
			return fmt.Errorf("execworkspace: grant %s: seconds must be > 0, got %d", g.Kind, g.Seconds)
		}
		return nil
	default:
		return fmt.Errorf("execworkspace: grant: unknown kind %s", g.Kind)
	}
}

// extraFieldsError rejects a Grant carrying a field that does not belong to
// its own Kind (e.g. a GrantNetwork grant with a non-empty Paths), keeping
// each kind's payload exactly the minimal shape the spec chose.
func (g Grant) extraFieldsError() error {
	var extra []string
	if g.Kind != GrantPathRead && g.Kind != GrantPathWrite && len(g.Paths) != 0 {
		extra = append(extra, "paths")
	}
	if g.Kind != GrantProcessExecution && len(g.Argv0s) != 0 {
		extra = append(extra, "argv0s")
	}
	if g.Kind != GrantResourceCeilings && len(g.Ceilings) != 0 {
		extra = append(extra, "ceilings")
	}
	if g.Kind != GrantTimeouts && g.Seconds != 0 {
		extra = append(extra, "seconds")
	}
	if len(extra) > 0 {
		return fmt.Errorf("execworkspace: grant %s: unexpected field(s) %v for this kind", g.Kind, extra)
	}
	return nil
}

// GrantSet is a decoded or to-be-encoded collection of grants (spec
// §Execution-grant enforcement). An empty GrantSet (no grants at all) is a
// valid minimal grant set, not an error.
type GrantSet struct {
	Grants []Grant
}

// Get returns the grant of kind k within s, if present.
func (s GrantSet) Get(k GrantKind) (Grant, bool) {
	for _, g := range s.Grants {
		if g.Kind == k {
			return g, true
		}
	}
	return Grant{}, false
}

// Validate reports whether every grant in s is individually well-formed
// (Grant.Validate) and no GrantKind repeats (spec: "duplicate kinds"
// rejected). An empty s.Grants always validates.
func (s GrantSet) Validate() error {
	seen := make(map[GrantKind]bool, len(s.Grants))
	for i, g := range s.Grants {
		if err := g.Validate(); err != nil {
			return fmt.Errorf("execworkspace: grant set: grants[%d]: %w", i, err)
		}
		if seen[g.Kind] {
			return fmt.Errorf("execworkspace: grant set: duplicate grant kind %s", g.Kind)
		}
		seen[g.Kind] = true
	}
	return nil
}

// grantDoc is one grant's on-disk JSON shape. Every field carries
// omitempty: only the field(s) meaningful for a given kind's payload are
// ever present on the wire, matching the spec's minimal-per-kind-payload
// choice. Strict decode (artifact.DecodeStrictJSON) rejects any field name
// beyond this set appearing anywhere in the document.
type grantDoc struct {
	Kind     string         `json:"kind"`
	Paths    []string       `json:"paths,omitempty"`
	Argv0s   []string       `json:"argv0s,omitempty"`
	Ceilings map[string]int `json:"ceilings,omitempty"`
	Seconds  *int           `json:"seconds,omitempty"`
}

// grantSetDoc is the top-level on-disk JSON shape: {"schema":
// "verdi.execution-grants/v1", "grants": [...]}.
type grantSetDoc struct {
	Schema string     `json:"schema"`
	Grants []grantDoc `json:"grants"`
}

// grantToDoc projects a validated Grant onto its wire shape. Seconds uses a
// pointer only here, at the wire boundary, so a GrantTimeouts grant's
// positive value is always explicitly present on the wire (a plain int
// with omitempty would vanish for the impossible-but-defensive zero case).
func grantToDoc(g Grant) grantDoc {
	doc := grantDoc{
		Kind:     g.Kind.String(),
		Paths:    g.Paths,
		Argv0s:   g.Argv0s,
		Ceilings: g.Ceilings,
	}
	if g.Kind == GrantTimeouts {
		seconds := g.Seconds
		doc.Seconds = &seconds
	}
	return doc
}

// grantFromDoc converts a decoded wire grantDoc into a domain Grant. It
// only maps the kind string to a GrantKind and copies fields across —
// Grant.Validate (called by DecodeGrantSet after every grant is converted)
// is the single place payload correctness is enforced, so this function
// never duplicates that logic.
func grantFromDoc(d grantDoc) (Grant, error) {
	kind, ok := grantKindByName(d.Kind)
	if !ok {
		return Grant{}, fmt.Errorf("execworkspace: grant set: unknown grant kind %q", d.Kind)
	}
	g := Grant{Kind: kind, Paths: d.Paths, Argv0s: d.Argv0s, Ceilings: d.Ceilings}
	if d.Seconds != nil {
		g.Seconds = *d.Seconds
	}
	return g, nil
}

// grantSetDocFor projects a validated GrantSet onto its on-disk document
// shape. Grants is built via make(..., 0, ...) rather than left nil so an
// empty GrantSet still encodes "grants":[] rather than "grants":null (the
// spec's "empty grants list is ALLOWED" case must round-trip as an empty
// array, never a null).
func grantSetDocFor(s GrantSet) grantSetDoc {
	docs := make([]grantDoc, len(s.Grants))
	for i, g := range s.Grants {
		docs[i] = grantToDoc(g)
	}
	return grantSetDoc{Schema: grantSchema, Grants: docs}
}

func encodeGrantSetDoc(doc grantSetDoc) ([]byte, error) {
	data, err := canonjson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("execworkspace: encoding grant set: %w", err)
	}
	return data, nil
}

// EncodeGrantSet renders set as canonical JSON bytes: {"schema":
// "verdi.execution-grants/v1", "grants": [...]}, sorted keys, trailing
// newline. It validates set first (GrantSet.Validate) and fails closed on
// any invalid grant or unknown GrantKind value — a malformed or
// out-of-vocabulary GrantSet is never serialized.
func EncodeGrantSet(set GrantSet) ([]byte, error) {
	if err := set.Validate(); err != nil {
		return nil, fmt.Errorf("execworkspace: encoding grant set: %w", err)
	}
	return encodeGrantSetDoc(grantSetDocFor(set))
}

// DecodeGrantSet strict-decodes grant-set bytes into a GrantSet (spec
// §Execution-grant enforcement; AD-7). It fails closed, in order, on: any
// unknown field anywhere or trailing data (artifact.DecodeStrictJSON); any
// departure from the canonical bytes this package would itself write for
// the decoded document (sidecar.go's canonical-bytes gate, mirrored
// exactly — see DecodeSidecar's doc comment for the full rationale); a
// schema value other than grantSchema; an unknown grant kind string; a
// duplicate kind; and an invalid per-kind payload (empty paths/argv0s
// list, an empty path/argv0 entry, empty or empty-keyed ceilings, or
// seconds <= 0) via Grant.Validate/GrantSet.Validate. An empty "grants": []
// list decodes to a valid, empty GrantSet — never an error.
func DecodeGrantSet(data []byte) (GrantSet, error) {
	var doc grantSetDoc
	if err := artifact.DecodeStrictJSON(data, &doc); err != nil {
		return GrantSet{}, fmt.Errorf("execworkspace: decoding grant set: %w", err)
	}
	canonical, err := encodeGrantSetDoc(doc)
	if err != nil {
		return GrantSet{}, fmt.Errorf("execworkspace: decoding grant set: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return GrantSet{}, fmt.Errorf(
			"execworkspace: grant set: bytes are not the canonical encoding of the document they decode to (want %q, got %q)",
			canonical, data,
		)
	}
	if doc.Schema != grantSchema {
		return GrantSet{}, fmt.Errorf("execworkspace: grant set: schema %q, want %q", doc.Schema, grantSchema)
	}
	grants := make([]Grant, 0, len(doc.Grants))
	for i, d := range doc.Grants {
		g, err := grantFromDoc(d)
		if err != nil {
			return GrantSet{}, fmt.Errorf("execworkspace: grant set: grants[%d]: %w", i, err)
		}
		grants = append(grants, g)
	}
	set := GrantSet{Grants: grants}
	if err := set.Validate(); err != nil {
		return GrantSet{}, fmt.Errorf("execworkspace: grant set: %w", err)
	}
	return set, nil
}
