package lint

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// memDoc is one in-memory store document for a rule-level test: the kind
// its location implies, its store-relative path, and its full bytes
// (frontmatter plus body). memSnapshot decodes it through the same
// decodeDocumentBytes seam the on-disk walk uses, so a rule sees exactly
// what it would see from a committed file.
type memDoc struct {
	kind, relPath, text string
}

// memSnapshot decodes docs into a Snapshot (Docs plus ByRef), failing the
// test when any fixture does not strict-decode — a fixture that VL-001
// would refuse never reaches the rule under test, so it would prove
// nothing.
func memSnapshot(t *testing.T, docs ...memDoc) *Snapshot {
	t.Helper()
	snap := &Snapshot{Root: t.TempDir(), ByRef: map[string][]*Document{}}
	for _, md := range docs {
		d := &Document{Kind: md.kind, RelPath: md.relPath}
		decodeDocumentBytes(d, []byte(md.text))
		if d.DecodeErr != nil {
			t.Fatalf("fixture %s does not strict-decode: %v\n%s", md.relPath, d.DecodeErr, md.text)
		}
		snap.Docs = append(snap.Docs, d)
		snap.ByRef[d.Base.ID] = append(snap.ByRef[d.Base.ID], d)
	}
	return snap
}

// runRule runs one rule over snap with no git, CI, or model context.
func runRule(r Rule, snap *Snapshot) []Finding {
	return r.Check(&RunInput{Ctx: context.Background(), Root: snap.Root, Snapshot: snap})
}

// cssSHA is a well-formed commit for pinned refs. No test here resolves
// it: a pinned link ref is refused on shape, before any git read.
const cssSHA = "0123456789abcdef0123456789abcdef01234567"

// cssObjectsYAML declares one object of every kind a fragment can name,
// plus a stub, so each target spec can answer every clause-(c) case.
const cssObjectsYAML = `acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "t", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "t", anchor: "#dc-1" }
open_questions:
  - { id: oq-1, text: "t", anchor: "#oq-1" }
stubs:
  - { slug: retry-path, acceptance_criteria: [ac-1] }
`

// cssSpec builds a spec document in zone ("active" or "archive"). status
// "" omits the field (a statusless spec). extra is raw frontmatter lines
// appended after the common fields (links:, decisions:, ...).
func cssSpec(zone, name, class, status, extra string) memDoc {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: spec/%s\nkind: spec\nclass: %s\ntitle: %q\nowners: [platform-team]\n", name, class, name)
	if status != "" {
		fmt.Fprintf(&b, "status: %s\n", status)
	}
	b.WriteString(extra)
	b.WriteString("---\n# body\n")
	return memDoc{kind: "spec", relPath: fmt.Sprintf(".verdi/specs/%s/%s/spec.md", zone, name), text: b.String()}
}

// cssConflict builds a conflict whose links are one challenges edge per
// ref. resolvedBy "" omits the field.
func cssConflict(name, status, resolvedBy string, challenges ...string) memDoc {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: conflict/%s\nkind: conflict\ntitle: %q\nowners: [platform-team]\nstatus: %s\n", name, name, status)
	if resolvedBy != "" {
		fmt.Fprintf(&b, "resolved_by: %q\n", resolvedBy)
	}
	b.WriteString("links:\n")
	for _, c := range challenges {
		fmt.Fprintf(&b, "  - { type: challenges, ref: %q }\n", c)
	}
	b.WriteString("---\n# body\n")
	return memDoc{kind: "conflict", relPath: fmt.Sprintf(".verdi/conflicts/%s.md", name), text: b.String()}
}

// cssADR builds an accepted ADR carrying the given raw links: block.
func cssADR(name, links string) memDoc {
	text := fmt.Sprintf("---\nid: adr/%s\nkind: adr\ntitle: %q\nowners: [platform-team]\nstatus: accepted\n%s---\n# body\n", name, name, links)
	return memDoc{kind: "adr", relPath: fmt.Sprintf(".verdi/adr/%s.md", name), text: text}
}

// linksYAML renders a top-level links: block, one {type, ref} per pair.
func linksYAML(pairs ...string) string {
	var b strings.Builder
	b.WriteString("links:\n")
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(&b, "  - { type: %s, ref: %q }\n", pairs[i], pairs[i+1])
	}
	return b.String()
}

// decisionYAML renders a decisions: block with one decision, dc-9, whose
// own links are the given {type, ref} pairs.
func decisionYAML(pairs ...string) string {
	var b strings.Builder
	b.WriteString("decisions:\n  - id: dc-9\n    text: \"replaces the object\"\n    anchor: \"#dc-9\"\n    links:\n")
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(&b, "      - { type: %s, ref: %q }\n", pairs[i], pairs[i+1])
	}
	return b.String()
}

// cssBase is the store every closed-spec object supersession case (VL-003's
// amendment and VL-026) runs against: a spec closed by zone (archive,
// statusless); an active-zone spec with an explicit legacy
// `status: closed`, which is NOT closed for VL-026 under SI-277 (the
// archive zone alone decides it); and a live spec that is not closed —
// each declaring an acceptance criterion, a constraint, a decision, an
// open question, and a stub — plus one ADR. None of them carries a link,
// so the base alone lints clean under VL-003 and VL-026.
func cssBase() []memDoc {
	return []memDoc{
		cssSpec("archive", "css-closed-archive", "feature", "", cssObjectsYAML),
		cssSpec("active", "css-closed-status", "story", "closed", cssObjectsYAML),
		cssSpec("active", "css-live", "feature", "accepted-pending-build", cssObjectsYAML),
		cssADR("0001-css", ""),
	}
}
