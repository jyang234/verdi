package artifact

import (
	"fmt"
	"regexp"
)

// LinkType is a typed edge per 02 §Link taxonomy. Backlinks are computed by
// inverting this table at index/dex-build time (out of scope here).
type LinkType string

const (
	LinkImplements  LinkType = "implements"
	LinkResolves    LinkType = "resolves" // R4: spike story -> open-question fragment
	LinkSupersedes  LinkType = "supersedes"
	LinkExempts     LinkType = "exempts" // R4: spec-scoped decision excused from an ADR/decision above it
	LinkVerifies    LinkType = "verifies"
	LinkDerivedFrom LinkType = "derived-from"
	LinkAnnotates   LinkType = "annotates"
	LinkDependsOn   LinkType = "depends-on"
	LinkStory       LinkType = "story"
	LinkImpacts     LinkType = "impacts"
	LinkChallenges  LinkType = "challenges"
)

var validLinkTypes = map[LinkType]bool{
	LinkImplements:  true,
	LinkResolves:    true,
	LinkSupersedes:  true,
	LinkExempts:     true,
	LinkVerifies:    true,
	LinkDerivedFrom: true,
	LinkAnnotates:   true,
	LinkDependsOn:   true,
	LinkStory:       true,
	LinkImpacts:     true,
	LinkChallenges:  true,
}

// Valid reports whether t is one of the eleven known link types.
func (t LinkType) Valid() bool { return validLinkTypes[t] }

// closedEdgeVocab is the "closed spec-object edge vocabulary" (02 §Link
// taxonomy, R4 concept §1): the only five link types a decision object's own
// `links:`, or a story/spike's top-level `links:`, may use when the target
// is an object fragment (§Identity and references). A fragment-targeting
// link of any other type fails closed.
var closedEdgeVocab = map[LinkType]bool{
	LinkImplements: true,
	LinkResolves:   true,
	LinkSupersedes: true,
	LinkExempts:    true,
	LinkDependsOn:  true,
}

// storyRefRe matches a scheme-prefixed tracker reference, e.g.
// "jira:LOAN-1482" (02 §Link taxonomy: "story ... scheme-prefixed ref").
var storyRefRe = regexp.MustCompile(`^[a-z][a-z0-9]*:[A-Za-z0-9][A-Za-z0-9-]*$`)

// externalRefRe matches a provisional index-minted external ref,
// "svc/<service>/<artifact>[/<name>]" (02 §Identity: "External refs
// (provisional)"). These are read-only, minted by the index from
// discovery rather than authored under .verdi/, but 02 is explicit that
// "they are valid link targets" — so this package's link validation
// accepts them alongside ordinary kind/name refs, even though the index
// that would resolve them (VL-003) doesn't exist until phase 3/4.
var externalRefRe = regexp.MustCompile(`^svc/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)?$`)

// Link is a typed edge in an artifact's frontmatter `links:` block
// (02 §Common frontmatter). Refs inside links are unpinned — only context
// manifests, evidence records, and board pins carry pinned refs.
type Link struct {
	Type LinkType `yaml:"type" json:"type"`
	Ref  string   `yaml:"ref" json:"ref"`
	Note string   `yaml:"note,omitempty" json:"note,omitempty"`
}

// Validate checks the link type is known and Ref has the right shape for
// that type: story links are scheme:key tracker refs; every other type is
// either an unpinned kind/name artifact ref or a provisional svc/... external
// ref (02 §Identity: "valid link targets"). A ref carrying an object
// fragment (§Identity and references) is additionally checked against the
// closed five-value spec-object edge vocabulary (02 §Link taxonomy):
// implements/resolves/supersedes/exempts/depends-on are the only types
// allowed to target a fragment — every other type fails closed.
func (l Link) Validate() error {
	if !l.Type.Valid() {
		return fmt.Errorf("artifact: unknown link type %q", l.Type)
	}
	if l.Ref == "" {
		return fmt.Errorf("artifact: link of type %q has an empty ref", l.Type)
	}
	if l.Type == LinkStory {
		if !storyRefRe.MatchString(l.Ref) {
			return fmt.Errorf("artifact: story link ref %q must be scheme:key form (e.g. jira:LOAN-1482)", l.Ref)
		}
		return nil
	}
	if externalRefRe.MatchString(l.Ref) {
		return nil
	}
	ref, err := ParseRef(l.Ref)
	if err != nil {
		return fmt.Errorf("artifact: link of type %q: %w", l.Type, err)
	}
	if ref.Fragment() && !closedEdgeVocab[l.Type] {
		return fmt.Errorf("artifact: link of type %q targets object fragment %q, but only implements/resolves/supersedes/exempts/depends-on may target a fragment (02 §Link taxonomy: closed spec-object edge vocabulary)", l.Type, l.Ref)
	}
	return nil
}

var (
	dateRe   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	sha256Re = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// Frozen is the point-in-time stamp carried by frozen artifacts
// (01 §Temporal classes): `frozen: { at: date, commit: sha }`.
type Frozen struct {
	At     string `yaml:"at" json:"at"`
	Commit string `yaml:"commit" json:"commit"`
}

// Validate checks At is a YYYY-MM-DD date and Commit looks like a real sha.
func (f Frozen) Validate() error {
	if !dateRe.MatchString(f.At) {
		return fmt.Errorf("artifact: frozen.at %q is not a YYYY-MM-DD date", f.At)
	}
	if !commitRe.MatchString(f.Commit) {
		return fmt.Errorf("artifact: frozen.commit %q is not a valid sha (7-40 lowercase hex characters)", f.Commit)
	}
	return nil
}

// Provenance records how a generated artifact was produced
// (02 §Generated artifacts and digests). Computed content carries Digest
// (recomputable from Inputs); judged content carries Integrity (tamper
// evident, not reproducible); an artifact with both kinds of section
// carries both fields.
type Provenance struct {
	Generator string   `yaml:"generator" json:"generator"`
	Version   string   `yaml:"version" json:"version"`
	Inputs    []string `yaml:"inputs" json:"inputs"`
	Digest    string   `yaml:"digest,omitempty" json:"digest,omitempty"`
	Integrity string   `yaml:"integrity,omitempty" json:"integrity,omitempty"`
}

// Validate checks Generator/Version/Inputs are present, at least one of
// Digest/Integrity is present, and every present hash and input has the
// right shape.
func (p Provenance) Validate() error {
	if p.Generator == "" {
		return fmt.Errorf("artifact: provenance.generator is required")
	}
	if p.Version == "" {
		return fmt.Errorf("artifact: provenance.version is required")
	}
	if len(p.Inputs) == 0 {
		return fmt.Errorf("artifact: provenance.inputs must list at least one input")
	}
	if p.Digest == "" && p.Integrity == "" {
		return fmt.Errorf("artifact: provenance must carry digest, integrity, or both (02 §Generated artifacts and digests)")
	}
	if p.Digest != "" && !sha256Re.MatchString(p.Digest) {
		return fmt.Errorf("artifact: provenance.digest %q is not sha256:<64 hex> form", p.Digest)
	}
	if p.Integrity != "" && !sha256Re.MatchString(p.Integrity) {
		return fmt.Errorf("artifact: provenance.integrity %q is not sha256:<64 hex> form", p.Integrity)
	}
	for _, in := range p.Inputs {
		if err := validateProvenanceInput(in); err != nil {
			return fmt.Errorf("artifact: provenance.inputs: %w", err)
		}
	}
	return nil
}

// validateProvenanceInput accepts either a pinned artifact ref
// (kind/name@commit) or a "path@commit" form (02 §Common frontmatter:
// "inputs: [<pinned-ref | path@commit>]").
func validateProvenanceInput(s string) error {
	if _, err := ParsePinnedRef(s); err == nil {
		return nil
	}
	path, commit, ok := cutLastAt(s)
	if !ok || path == "" || !commitRe.MatchString(commit) {
		return fmt.Errorf("input %q is neither a pinned ref (kind/name@commit) nor path@commit", s)
	}
	return nil
}

// cutLastAt splits s on its last '@', since paths may themselves contain
// '@' but a trailing "@<commit>" is always the pin.
func cutLastAt(s string) (before, after string, ok bool) {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}

// Base holds the frontmatter fields common to every artifact kind
// (02 §Common frontmatter), minus `status`, which each kind embeds with
// its own enum type (attestation has none at all — "existence is the
// record").
//
// Schema is *not* listed in 02 §Common frontmatter's table, but all five
// of that spec's own sibling component-spec documents (01, 02, 03, 04, 05)
// carry a top-level `schema: verdi.xxx/vN` key in their frontmatter (e.g.
// 02 itself: `schema: verdi.artifact/v1`) — discovered self-hosting these
// six specs in phase 4 (PLAN.md Phase 4's "eat the dog food" step).
// Accepted here as an optional common field so strict decode does not
// reject 02's own documents; flagged in the phase 4 report as an
// invention-ledger candidate for 02 §Common frontmatter to document
// formally. No format is enforced on the value beyond being a string.
type Base struct {
	ID         string      `yaml:"id"`
	Kind       Kind        `yaml:"kind"`
	Title      string      `yaml:"title"`
	Owners     []string    `yaml:"owners"`
	Schema     string      `yaml:"schema,omitempty"`
	Links      []Link      `yaml:"links,omitempty"`
	Frozen     *Frozen     `yaml:"frozen,omitempty"`
	Provenance *Provenance `yaml:"provenance,omitempty"`
}

// validateBase checks the fields common to every kind: id parses as a ref
// of the expected kind and agrees with the Kind field, title and owners
// are non-empty, every link is individually valid, and Frozen/Provenance
// (if present) are individually valid.
func (b Base) validateBase(wantKind Kind) error {
	if b.Kind != wantKind {
		return fmt.Errorf("artifact: kind field %q does not match expected kind %q", b.Kind, wantKind)
	}
	ref, err := ParseRef(b.ID)
	if err != nil {
		return fmt.Errorf("artifact: id: %w", err)
	}
	if ref.Kind != wantKind {
		return fmt.Errorf("artifact: id %q has kind %q, want %q", b.ID, ref.Kind, wantKind)
	}
	if ref.Pinned() {
		return fmt.Errorf("artifact: id %q must not be pinned", b.ID)
	}
	if b.Title == "" {
		return fmt.Errorf("artifact: title is required")
	}
	if len(b.Owners) == 0 {
		return fmt.Errorf("artifact: owners must list at least one owner")
	}
	for _, o := range b.Owners {
		if o == "" {
			return fmt.Errorf("artifact: owners must not contain an empty entry")
		}
	}
	for i, l := range b.Links {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("artifact: links[%d]: %w", i, err)
		}
	}
	if b.Frozen != nil {
		if err := b.Frozen.Validate(); err != nil {
			return fmt.Errorf("artifact: frozen: %w", err)
		}
	}
	if b.Provenance != nil {
		if err := b.Provenance.Validate(); err != nil {
			return fmt.Errorf("artifact: provenance: %w", err)
		}
	}
	return nil
}
