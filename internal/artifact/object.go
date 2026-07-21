package artifact

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Attribute is one of a feature or story spec's two required spec
// attributes, `problem:`/`outcome:` (02 §Object model). Attributes are
// distinct from objects below: exactly one each, attached to the spec
// itself, with no id and no links: of their own. Each carries the same
// anchor-resolution rule as objects — the heading named by Anchor must
// exist verbatim in the document body.
type Attribute struct {
	Text   string `yaml:"text" json:"text"`
	Anchor string `yaml:"anchor" json:"anchor"`
}

// Validate checks Text and Anchor are both present. Attribute is a
// wholly round-four field with no v0 grandfathered usage (unlike
// AcceptanceCriterion, whose pre-existing v0 fixtures never carried an
// anchor at all), so — unlike AcceptanceCriterion.Anchor — both fields are
// required together whenever an Attribute is declared at all.
func (a Attribute) Validate() error {
	if a.Text == "" {
		return fmt.Errorf("artifact: attribute has no text")
	}
	if a.Anchor == "" {
		return fmt.Errorf("artifact: attribute %q has no anchor (02 §Object model: every object and attribute carries an anchor)", a.Text)
	}
	return nil
}

// Constraint is one entry in a feature or story spec's `constraints:` block
// (02 §Object model): a rule that applies wherever relevant, deliberately
// not mappable to stories and carrying no `links:` of its own.
type Constraint struct {
	ID     string `yaml:"id"`
	Text   string `yaml:"text"`
	Anchor string `yaml:"anchor"`
}

// Validate checks ID looks like a constraint id (co-<slug>), and Text and
// Anchor are both present. Constraints: is a wholly round-four block (no
// v0 spec ever declared one), so both fields are required unconditionally,
// unlike AcceptanceCriterion's backward-compatible optional anchor.
func (c Constraint) Validate() error {
	if !strings.HasPrefix(c.ID, "co-") || !objectIDRe.MatchString(c.ID) {
		return fmt.Errorf("artifact: constraint id %q must look like co-<slug>", c.ID)
	}
	if c.Text == "" {
		return fmt.Errorf("artifact: constraint %s has no text", c.ID)
	}
	if c.Anchor == "" {
		return fmt.Errorf("artifact: constraint %s has no anchor (02 §Object model)", c.ID)
	}
	return nil
}

// Decision is one entry in a feature or story spec's `decisions:` block
// (02 §Object model): may carry its own `links:` — the same shape as
// document-level links: — for supersedes/exempts edges against ADRs or
// other decisions (03 §Decision-conflict gate).
type Decision struct {
	ID     string `yaml:"id"`
	Text   string `yaml:"text"`
	Anchor string `yaml:"anchor"`
	Links  []Link `yaml:"links,omitempty"`
}

// Validate checks ID looks like a decision id (dc-<slug>), Text and Anchor
// are both present (decisions: is a wholly round-four block, same
// unconditional-anchor posture as Constraint), and every declared link is
// individually valid.
func (d Decision) Validate() error {
	if !strings.HasPrefix(d.ID, "dc-") || !objectIDRe.MatchString(d.ID) {
		return fmt.Errorf("artifact: decision id %q must look like dc-<slug>", d.ID)
	}
	if d.Text == "" {
		return fmt.Errorf("artifact: decision %s has no text", d.ID)
	}
	if d.Anchor == "" {
		return fmt.Errorf("artifact: decision %s has no anchor (02 §Object model)", d.ID)
	}
	for i, l := range d.Links {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("artifact: decision %s links[%d]: %w", d.ID, i, err)
		}
	}
	return nil
}

// OpenQuestion is one entry in a feature or story spec's `open_questions:`
// block (02 §Object model; R4-I-16, added at ratification round four's
// phase review). Open questions carry no `links:` of their own: they are
// the *targets* of `resolves` edges (a spike's deliverable is answering
// them) and the graduation destination of the board's carried
// open-question stickies (VL-017) — a resolved open question graduates
// into a real object or prose by an ordinary edit, and the entry is
// removed in the same edit. Follows Constraint's exact pattern: a wholly
// round-four block with no v0 usage, so, like Constraint and Decision (and
// unlike AcceptanceCriterion), Anchor is required unconditionally rather
// than left decode-optional.
type OpenQuestion struct {
	ID     string `yaml:"id"`
	Text   string `yaml:"text"`
	Anchor string `yaml:"anchor"`
}

// Validate checks ID looks like an open-question id (oq-<slug>), and Text
// and Anchor are both present.
func (q OpenQuestion) Validate() error {
	if !strings.HasPrefix(q.ID, "oq-") || !objectIDRe.MatchString(q.ID) {
		return fmt.Errorf("artifact: open question id %q must look like oq-<slug>", q.ID)
	}
	if q.Text == "" {
		return fmt.Errorf("artifact: open question %s has no text", q.ID)
	}
	if q.Anchor == "" {
		return fmt.Errorf("artifact: open question %s has no anchor (02 §Object model)", q.ID)
	}
	return nil
}

// Stub is one entry in a feature spec's acceptance-time `stubs:` scoping
// record (02 §Kind registry: "the acceptance-time scoping record, one entry
// per intended story"): `{ slug: <title-slug>, acceptance_criteria:
// [<ac-id>...] }`. A stub may instead be a **spike stub** (round 5.4,
// mirroring the story level's own spike discriminator): `{ slug, spike:
// true, resolves: [<oq-id>...] }` — one list, flag-discriminated (DC-4),
// rather than a parallel `spike_stubs:` block.
type Stub struct {
	Slug               string   `yaml:"slug"`
	Spike              bool     `yaml:"spike,omitempty"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria,omitempty"`
	Resolves           []string `yaml:"resolves,omitempty"`
}

// Validate checks Slug is a non-empty kebab-case title slug and enforces
// the DC-4 grammar, fail closed: `resolves` requires `spike: true`; a spike
// stub declares `resolves` (non-empty) and no `acceptance_criteria`; a
// plain stub declares `acceptance_criteria` (non-empty) and no `resolves`
// (the two blocks are mutually exclusive by construction of the switch
// below, but Resolves is checked unconditionally first so a non-spike stub
// carrying a stray `resolves:` fails closed rather than being silently
// ignored).
func (s Stub) Validate() error {
	if !simpleNameRe.MatchString(s.Slug) {
		return fmt.Errorf("artifact: stub slug %q must be kebab-case", s.Slug)
	}
	if len(s.Resolves) > 0 && !s.Spike {
		return fmt.Errorf("artifact: stub %s: resolves requires spike: true (02 §Kind registry, DC-4)", s.Slug)
	}
	if s.Spike {
		if len(s.Resolves) == 0 {
			// vocab:identity — strict-decode/schema diagnostic speaking class/field ids
			return fmt.Errorf("artifact: spike stub %s declares no resolves (the open questions it will answer)", s.Slug)
		}
		if len(s.AcceptanceCriteria) != 0 {
			// vocab:identity — strict-decode/schema diagnostic speaking class/field ids
			return fmt.Errorf("artifact: spike stub %s must not declare acceptance_criteria (02 §Kind registry, DC-4)", s.Slug)
		}
		for _, id := range s.Resolves {
			if !oqIDRe.MatchString(id) {
				// vocab:identity — strict-decode/schema diagnostic speaking class/field ids
				return fmt.Errorf("artifact: spike stub %s: resolves entry %q is not a valid oq-<slug> id", s.Slug, id)
			}
		}
		return nil
	}
	if len(s.AcceptanceCriteria) == 0 {
		return fmt.Errorf("artifact: stub %s declares no acceptance criteria", s.Slug)
	}
	for _, id := range s.AcceptanceCriteria {
		if !acIDRe.MatchString(id) {
			return fmt.Errorf("artifact: stub %s: acceptance_criteria entry %q is not a valid ac-<slug> id", s.Slug, id)
		}
	}
	return nil
}

// ObjectContentHash computes the (kind, id, text) content hash that is a
// frontmatter-declared object's cross-revision identity (02 §Object model,
// the I-37 identity): "the same id with an unchanged hash across revisions
// is carried; the same id with a changed hash is amended or
// amended_advisory" — the identity the supersession manifest classifies
// objects by. kind is the object's frontmatter block name
// ("acceptance_criteria" | "constraints" | "decisions") — object attributes
// (problem/outcome) are excluded, per 02 §Object model's "attributes are
// distinct from objects below" (no id, so no content-hash identity of their
// own). Computed over the canonical JSON form (02 §Generated artifacts and
// digests) so the hash is stable regardless of map/struct field order. The
// hash tail itself is canonjson.Digest (spec/shared-homes ac-2/dc-2: this
// wrapper's exported API and doc survive the collapse; only its body did).
func ObjectContentHash(kind ObjectKind, id, text string) (string, error) {
	payload := map[string]interface{}{"kind": string(kind), "id": id, "text": text}
	digest, err := canonjson.Digest(payload)
	if err != nil {
		return "", fmt.Errorf("artifact: object content hash: %w", err)
	}
	return digest, nil
}

// ObjectKind discriminates the three frontmatter-declared object block
// types for ObjectContentHash's (kind, id, text) identity tuple
// (02 §Object model).
type ObjectKind string

const (
	ObjectKindAcceptanceCriterion ObjectKind = "acceptance_criteria"
	ObjectKindConstraint          ObjectKind = "constraints"
	ObjectKindDecision            ObjectKind = "decisions"
	ObjectKindOpenQuestion        ObjectKind = "open_questions"
)

// HeadingAnchors extracts every ATX ("# ", "## ", ...) heading in body and
// returns the set of GitHub-flavored-markdown-style anchor slugs those
// headings resolve to — the exact-match resolution target for every
// object's and attribute's `anchor:` field (02 §Object model: "Anchor
// resolution is exact-match ... the VL-014 where-anchor check, restated
// here as the general rule for every object and attribute").
func HeadingAnchors(body []byte) map[string]bool {
	anchors := make(map[string]bool)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		i := 0
		for i < len(trimmed) && trimmed[i] == '#' {
			i++
		}
		if i == 0 || i > 6 {
			continue
		}
		rest := trimmed[i:]
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			continue // e.g. "#foo" is not a heading
		}
		text := strings.TrimSpace(rest)
		if text == "" {
			continue
		}
		anchors[SlugifyHeading(text)] = true
	}
	return anchors
}

// SlugifyHeading computes a heading's anchor slug: lowercase, spaces and
// hyphens preserved as hyphens, every other rune dropped — the common
// GitHub-flavored-markdown heading-anchor algorithm (it does not implement
// GFM's duplicate-heading "-1" disambiguation suffix, which the corpus and
// self-hosted specs never need — no two headings in the same document
// share text). Exported so internal/lint can reuse it directly rather than
// keeping its own copy (CLAUDE.md: shared code lives in one internal/
// package).
func SlugifyHeading(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// ResolveAnchor reports whether anchor (a "#slug" or bare "slug" reference)
// resolves to a heading in anchors. The anchor side is slugified through the
// identical SlugifyHeading transform HeadingAnchors already applies to every
// heading's own text before the two are compared (spec/ritual-traps ac-1,
// X-1): without this, a frontmatter anchor: value written in the heading's
// own original case (e.g. "AC-1" against a "## AC-1" heading) silently
// failed to resolve unless the author already knew, from unwritten
// convention, to write every anchor pre-lowercased. Symmetric slugification
// only ever resolves MORE anchors than before, never fewer — an anchor that
// already resolved keeps resolving.
func ResolveAnchor(anchors map[string]bool, anchor string) bool {
	return anchors[SlugifyHeading(strings.TrimPrefix(anchor, "#"))]
}

// ResolveObjectAnchors checks every present attribute and object anchor in
// fm against body's actual headings and returns the first mismatch, named
// with which attribute/object it belongs to. Resolution is slug-symmetric,
// not exact-match: ResolveAnchor puts both the anchor and every heading
// through the same SlugifyHeading transform (case-folded, punctuation
// dropped, spaces hyphenated) and compares the resulting slugs, so
// `anchor: "AC 1"` resolves against a `## AC-1` heading (spec/ritual-traps
// ac-1, the ratification vehicle for this reading of 02 §Object model; it
// superseded the earlier exact-match rule under which only an anchor already
// written as the heading's own slug resolved). An empty anchor is skipped
// rather than treated as a mismatch: v0 grandfathered feature specs, and any
// decode path that validates frontmatter without a document body, never
// populate these fields at all (see Attribute's and AcceptanceCriterion's own
// requiredness notes) — this method is the separate, body-aware resolution
// step callers run once a body is available.
func (fm SpecFrontmatter) ResolveObjectAnchors(body []byte) error {
	anchors := HeadingAnchors(body)
	check := func(label, anchor string) error {
		if anchor == "" {
			return nil
		}
		if !ResolveAnchor(anchors, anchor) {
			// Truthful post-ac-1 guidance: comparison is slug-vs-slug, so
			// surface the anchor's own computed slug and say no heading's slug
			// matches it — not the stale "make it match exactly" (spec/
			// ritual-traps ac-1). e.g. `anchor: "AC 1"` already resolves
			// against `## AC-1`, so a genuine failure is a real slug mismatch.
			slug := SlugifyHeading(strings.TrimPrefix(anchor, "#"))
			return fmt.Errorf("artifact: %s anchor %q does not resolve to a heading in the document body: its slug %q matches no heading's slug (resolution is slug-symmetric, spec/ritual-traps ac-1)", label, anchor, slug)
		}
		return nil
	}

	if fm.Problem != nil {
		if err := check("problem attribute", fm.Problem.Anchor); err != nil {
			return err
		}
	}
	if fm.Outcome != nil {
		if err := check("outcome attribute", fm.Outcome.Anchor); err != nil {
			return err
		}
	}
	for _, ac := range fm.AcceptanceCriteria {
		if err := check(fmt.Sprintf("acceptance criterion %s", ac.ID), ac.Anchor); err != nil {
			return err
		}
	}
	for _, c := range fm.Constraints {
		if err := check(fmt.Sprintf("constraint %s", c.ID), c.Anchor); err != nil {
			return err
		}
	}
	for _, d := range fm.Decisions {
		if err := check(fmt.Sprintf("decision %s", d.ID), d.Anchor); err != nil {
			return err
		}
	}
	for _, q := range fm.OpenQuestions {
		if err := check(fmt.Sprintf("open question %s", q.ID), q.Anchor); err != nil {
			return err
		}
	}
	return nil
}
