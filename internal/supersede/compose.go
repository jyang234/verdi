// Package supersede is the shared core for composing a superseding
// successor spec out of an accepted predecessor's exact bytes (PLAN.md §7
// I-129, ratified by the owner 2026-09-17, option (a); spec/uat-round-1
// ac-11; 02 §Kind registry: "a supersession: block is required ...
// classifying every predecessor object exactly once (VL-015)"). It is the
// ONE mechanism `verdi design start --supersedes` (cmd/verdi/
// designsupersede.go) and the board's later Revise action (W3-C,
// spec/uat-round-1 ac-11's board half) both call, so the two surfaces can
// never drift — mirroring internal/stubinstantiate's own "one shared core,
// two callers" shape (its own doc comment: "the ADJ-65 asymmetry closed at
// the mechanism, not merely at the surface").
//
// Compose is pure (no I/O, no git): it takes the predecessor's raw spec.md
// bytes and returns the successor's exact spec.md bytes plus a small
// manifest. Resolve is the one I/O-performing half: it reads a named
// predecessor out of the CURRENT checkout's active zone and proves,
// through the shared specstate projector (exactly as
// cmd/verdi/designfromstub.go's runDesignStartFromStub already does for
// stub-instantiate), that it is an accepted-pending-build feature — the
// only shape 02 §Kind registry allows a superseding revision to name.
package supersede

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
)

// ComposeInput is Compose's one argument: the predecessor's own bare name
// (spec/<PredecessorName>, the store's kebab-case directory name — 01
// §Store layout), its exact current spec.md bytes (read by the caller, in
// whatever checkout it considers authoritative — Resolve below reads them
// from the current checkout's active zone, but Compose itself never reads
// anything, so a caller with bytes from elsewhere — e.g. a specific commit
// — can drive it too), and the successor's own bare name.
type ComposeInput struct {
	PredecessorName string
	PredecessorRaw  []byte
	SuccessorName   string
}

// Composed is what a successful Compose produced: the successor's exact
// spec.md bytes, the predecessor object ids it classified `carried` (every
// acceptance_criteria/constraints/decisions/open_questions id, in
// declaration order — 02 §Object model), and the supersedes ref it wrote
// ("spec/<PredecessorName>").
type Composed struct {
	Content       []byte
	CarriedIDs    []string
	SupersedesRef string
}

// topLevelKeyRe matches a frontmatter line that opens a new top-level
// mapping key: column 0 (no leading whitespace), a lowercase identifier —
// bare, single-quoted, or double-quoted, all three of which YAML treats as
// the SAME key and artifact.DecodeSpec accepts identically — then a colon.
// The quoted spellings are recognized because a predecessor that writes
// `"status":` or `"frozen":` would otherwise sail past the drop rule below
// and hand the successor its predecessor's own acceptance stamp, and one
// that writes `"id":` would keep the predecessor's identity: legal YAML
// this package must handle, not a shape it may assume away. Every spec
// frontmatter document in this store is a flat
// top-level YAML mapping (02 §Common frontmatter, §feature-spec
// frontmatter additions) whose nested content is always MORE indented than
// its parent key — a YAML block-mapping requirement, not merely an
// observed convention, since a block scalar or sequence continuation
// dedented back to its parent key's own column would end that parent's
// value under YAML's own block rules. A line-based split on this pattern
// therefore exactly recovers each top-level key's byte span (from its own
// line through the line just before the next top-level key, or end of
// document) without parsing or re-marshalling YAML at all — the
// "prefer editing the YAML text at the line/block level" approach: every
// UNTOUCHED key's span is copied byte-for-byte from the original document,
// so its formatting (flow vs. block style, quoting, comments) survives
// exactly, which a full yaml.Node re-marshal was verified NOT to do (it
// renormalizes indent width and strips flow-mapping padding spaces even
// for nodes the edit never touches).
var topLevelKeyRe = regexp.MustCompile(`^(?:"([a-z][a-z0-9_]*)"|'([a-z][a-z0-9_]*)'|([a-z][a-z0-9_]*)):`)

// droppedKeys are the legacy per-kind lifecycle fields a superseding
// revision never carries forward: the successor is a fresh draft under the
// Git-derived, merge-signaled acceptance model (docs/superpowers/specs/
// 2026-08-01-merge-signals-spec-acceptance-design.md), so a persisted
// `status:` or `frozen:` stamp copied from the predecessor would describe
// the WRONG revision's acceptance, not this one's (which has not happened
// yet). Recorded choice (dispatch contract, part A): "legacy status:/
// frozen: fields dropped."
var droppedKeys = map[string]bool{"status": true, "frozen": true}

// Compose builds a superseding successor's exact spec.md bytes out of a
// predecessor's own raw bytes (02 §Kind registry, VL-015): every
// frontmatter field is copied byte-for-byte EXCEPT id (VL-002's
// path-bound identity — the only field that name implies), links (the
// predecessor's own whole-spec supersedes link, if any, is replaced by
// exactly one new `{type: supersedes, ref: "spec/<predecessor>"}`;
// fragment links and every other link are kept), supersession: (replaced
// or added, classifying every predecessor object `carried`), and the
// legacy status:/frozen: fields (dropped — droppedKeys above). The
// document body (everything after the closing frontmatter delimiter) is
// copied byte-for-byte, untouched. Title is kept byte-for-byte too (a
// disclosed choice: 02's own VL-002 binds only id/name to the containing
// directory — title carries no path constraint at all, so there is no
// forcing reason to regenerate it from SuccessorName, and doing so would
// silently discard the predecessor's own considered title wording).
//
// Compose self-validates before returning (CLAUDE.md: "never fake
// success"): the composed bytes must strict-decode (SplitFrontmatter +
// DecodeSpec) and declare class: feature (designscaffold.CheckClass) —
// catching both a bug in this package's own text surgery and a
// predecessor whose own class was never feature to begin with (supersession
// is feature-only, 02 §Kind registry; Resolve enforces this too, earlier
// and with a purpose-built message, but Compose is a public, independently
// callable function and must not trust a caller that skipped Resolve).
func Compose(in ComposeInput) (Composed, error) {
	fm, body, err := artifact.SplitFrontmatter(in.PredecessorRaw)
	if err != nil {
		return Composed{}, fmt.Errorf("supersede: predecessor spec/%s: %w", in.PredecessorName, err)
	}
	predSpec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return Composed{}, fmt.Errorf("supersede: predecessor spec/%s frontmatter does not strict-decode: %w", in.PredecessorName, err)
	}

	supersedesRef := "spec/" + in.PredecessorName
	carried := declarationOrderIDs(predSpec)
	newLinksBlock := renderLinksBlock(predSpec.Links, supersedesRef)
	newSupersessionBlock := renderSupersessionBlock(carried)

	lines := strings.Split(string(fm), "\n")
	keyStarts, keyNames := topLevelKeyLines(lines)

	var blocks []string
	sawLinks, sawSupersession := false, false
	for i, name := range keyNames {
		start := keyStarts[i]
		end := len(lines)
		if i+1 < len(keyStarts) {
			end = keyStarts[i+1]
		}
		span := strings.Join(lines[start:end], "\n")

		switch name {
		case "id":
			blocks = append(blocks, "id: spec/"+in.SuccessorName)
		case "links":
			blocks = append(blocks, newLinksBlock)
			sawLinks = true
		case "supersession":
			blocks = append(blocks, newSupersessionBlock)
			sawSupersession = true
		default:
			if droppedKeys[name] {
				continue
			}
			blocks = append(blocks, span)
		}
	}
	if !sawLinks {
		blocks = append(blocks, newLinksBlock)
	}
	if !sawSupersession {
		blocks = append(blocks, newSupersessionBlock)
	}

	newFM := strings.Join(blocks, "\n")
	newDoc := []byte("---\n" + newFM + "\n---\n" + string(body))

	// Self-validate (see doc comment above): a failure here is this
	// package's own internal error, never a caller-facing "your predecessor
	// is invalid" message — Resolve/the predecessor decode above already
	// screened predecessor-shaped problems.
	outFM, _, splitErr := artifact.SplitFrontmatter(newDoc)
	if splitErr != nil {
		return Composed{}, fmt.Errorf("supersede: internal error: composed successor failed self-validation: %w", splitErr)
	}
	outSpec, decodeErr := artifact.DecodeSpec(outFM)
	if decodeErr != nil {
		return Composed{}, fmt.Errorf("supersede: internal error: composed successor failed self-validation: %w", decodeErr)
	}
	if err := designscaffold.CheckClass(outSpec, artifact.ClassFeature); err != nil {
		return Composed{}, fmt.Errorf("supersede: internal error: composed successor failed self-validation: %w", err)
	}
	if err := checkComposedPostconditions(outSpec, in.SuccessorName, supersedesRef, carried); err != nil {
		return Composed{}, fmt.Errorf("supersede: internal error: composed successor failed self-validation: %w", err)
	}

	return Composed{Content: newDoc, CarriedIDs: carried, SupersedesRef: supersedesRef}, nil
}

// checkComposedPostconditions asserts, on the DECODED successor, every
// property the text surgery above is supposed to have established — the
// backstop that makes a bug in that surgery fail CLOSED, naming the
// offending field, instead of shipping a successor that silently inherits
// what it must not.
//
// A decode-and-CheckClass self-validation alone is NOT enough, and the
// quoted-key defect this function was written for is the witness: a
// predecessor spelling its own lifecycle fields `"status":`/`"frozen":`
// (ordinary, legal YAML that DecodeSpec accepts) had those spans copied
// verbatim, and the composed successor still decoded cleanly as a
// well-formed spec of the right class — so the caller exited 0 on a
// successor carrying the predecessor's own identity and acceptance stamp.
// Every property below is therefore asserted on the decode RESULT, never
// on the rendered text: what the next reader of these bytes sees is what
// is checked.
//
// The properties are exactly the controller's R3-4 ruling (docs/superpowers/
// reports/2026-09-17-uat-round-1-wave3-ledger.md): only identity fields,
// the supersedes link, and the supersession block may differ from the
// predecessor; the legacy lifecycle stamps are dropped; a predecessor
// already carrying a supersedes link/supersession block has both replaced.
func checkComposedPostconditions(out *artifact.SpecFrontmatter, successorName, supersedesRef string, carried []string) error {
	if wantID := "spec/" + successorName; out.ID != wantID {
		return fmt.Errorf("id is %q, want %q", out.ID, wantID)
	}
	if out.Status != "" {
		return fmt.Errorf("status: is present (%q); a successor must carry no inherited lifecycle stamp", out.Status)
	}
	if out.Frozen != nil {
		return fmt.Errorf("frozen: is present (commit %q); a successor must carry no inherited lifecycle stamp", out.Frozen.Commit)
	}

	refs := artifact.WholeSpecSupersedesRefs(out.Links)
	if len(refs) != 1 {
		return fmt.Errorf("links: declare %d whole-spec supersedes refs, want exactly one naming %s (I-47)", len(refs), supersedesRef)
	}
	if got := refs[0].String(); got != supersedesRef {
		return fmt.Errorf("links: declare the whole-spec supersedes ref %s, want %s", got, supersedesRef)
	}

	if out.Supersession == nil {
		return fmt.Errorf("supersession: block is absent, want every predecessor object classified carried (VL-015)")
	}
	classified := make(map[string]bool, len(out.Supersession.Carried))
	for _, id := range out.Supersession.Carried {
		classified[id] = true
	}
	for _, id := range carried {
		if !classified[id] {
			return fmt.Errorf("supersession: leaves the predecessor object %s unclassified (VL-015 classifies every predecessor object exactly once)", id)
		}
	}
	if got, want := len(out.Supersession.Carried), len(carried); got != want {
		return fmt.Errorf("supersession: carried lists %d ids, want the predecessor's own %d", got, want)
	}
	for _, bucket := range []struct {
		name string
		n    int
	}{
		{"amended", len(out.Supersession.Amended)},
		{"amended_advisory", len(out.Supersession.AmendedAdvisory)},
		{"removed", len(out.Supersession.Removed)},
		{"added", len(out.Supersession.Added)},
	} {
		if bucket.n != 0 {
			return fmt.Errorf("supersession: %s lists %d entries, want none at scaffold time (the author reclassifies by hand afterward)", bucket.name, bucket.n)
		}
	}
	return nil
}

// topLevelKeyLines scans lines (a split, delimiter-free frontmatter body)
// for topLevelKeyRe matches, returning each match's line index and key
// name (UNQUOTED, whichever of the three spellings the line used), in
// document order.
func topLevelKeyLines(lines []string) (starts []int, names []string) {
	for i, l := range lines {
		m := topLevelKeyRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		// Exactly one of the three alternatives can have matched.
		name := ""
		for _, group := range m[1:] {
			if group != "" {
				name = group
				break
			}
		}
		starts = append(starts, i)
		names = append(names, name)
	}
	return starts, names
}

// declarationOrderIDs returns every id spec declares across its four
// object blocks, in 02 §Object model's own fixed block order —
// acceptance_criteria, constraints, decisions, open_questions — each
// block's own internal order preserved. This is the `carried:` list's
// order (dispatch contract, part A): a fresh scaffold classifies every
// predecessor object carried (VL-015's completeness requirement, trivially
// satisfied at generation time since nothing has been reclassified yet);
// the author reclassifies by hand-editing supersession: afterward.
func declarationOrderIDs(spec *artifact.SpecFrontmatter) []string {
	ids := make([]string, 0, len(spec.AcceptanceCriteria)+len(spec.Constraints)+len(spec.Decisions)+len(spec.OpenQuestions))
	for _, ac := range spec.AcceptanceCriteria {
		ids = append(ids, ac.ID)
	}
	for _, c := range spec.Constraints {
		ids = append(ids, c.ID)
	}
	for _, d := range spec.Decisions {
		ids = append(ids, d.ID)
	}
	for _, q := range spec.OpenQuestions {
		ids = append(ids, q.ID)
	}
	return ids
}

// renderLinksBlock renders a fresh `links:` block: every one of
// predecessor's own links EXCEPT its own whole-spec supersedes link(s), if
// any (I-47: a superseding revision names exactly one whole-spec
// predecessor, so the predecessor's own inherited claim about ITS OWN
// predecessor must not leak through — a v2's `supersedes: spec/v1` must not
// survive into v3's frontmatter alongside the new `supersedes: spec/v2`),
// plus exactly one new `{type: supersedes, ref: supersedesRef}` — matching
// internal/designscaffold's story.md template's own established rendering
// convention byte-for-byte ("  - { type: <type>, ref: \"<ref>\" }") so a
// freshly-scaffolded links: block reads exactly like every other scaffold
// this codebase already produces.
func renderLinksBlock(predLinks []artifact.Link, supersedesRef string) string {
	wholeSpecSupersedes := make(map[artifact.Link]bool)
	for _, ref := range artifact.WholeSpecSupersedesRefs(predLinks) {
		for _, l := range predLinks {
			if l.Type == artifact.LinkSupersedes && l.Ref == ref.String() {
				wholeSpecSupersedes[l] = true
			}
		}
	}

	var b strings.Builder
	b.WriteString("links:")
	for _, l := range predLinks {
		if wholeSpecSupersedes[l] {
			continue
		}
		b.WriteString("\n  - { type: ")
		b.WriteString(string(l.Type))
		b.WriteString(", ref: ")
		b.WriteString(fmt.Sprintf("%q", l.Ref))
		if l.Note != "" {
			b.WriteString(", note: ")
			b.WriteString(fmt.Sprintf("%q", l.Note))
		}
		b.WriteString(" }")
	}
	b.WriteString("\n  - { type: supersedes, ref: ")
	b.WriteString(fmt.Sprintf("%q", supersedesRef))
	b.WriteString(" }")
	return b.String()
}

// renderSupersessionBlock renders a fresh `supersession:` block classifying
// every id in carried as `carried`, with amended/amended_advisory/removed/
// added present and empty (dispatch contract, part A) — matching 02 §Kind
// registry's own worked example's formatting (flow-style id/empty lists).
func renderSupersessionBlock(carried []string) string {
	var b strings.Builder
	b.WriteString("supersession:\n  carried: [")
	b.WriteString(strings.Join(carried, ", "))
	b.WriteString("]\n  amended: []\n  amended_advisory: []\n  removed: []\n  added: []")
	return b.String()
}
