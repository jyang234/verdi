// Diagram-alignment computed section (spec/alignment-section, implementing
// spec/diagram-proposals ac-5). See doc.go for the package's overall shape;
// this file adds the "### Diagram alignment" subsection's computation
// alongside the existing declares:-boundaries one (computed.go).
//
// Two discovery scopes, deliberately asymmetric (spec/alignment-section
// dc-1): DiscoverAcceptedProposals walks the WHOLE corpus (a class:
// proposal diagram carries no ownership edge to any spec), while
// DiscoverIllustrativeFigures scans only the CURRENT spec's own body text
// (dc-8's containment tie). Regeneration and structural comparison are
// never reimplemented here (dc-2): every graph/mermaid computation calls
// straight into internal/diagramverify's exported functions over the same
// upstream.Runner seam Compute (computed.go) already threads through.
package align

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/diagramverify"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/upstream"
)

// diagramsDirRelPath is 01 §Directory layout's fixed home for authored
// diagram artifacts: ".verdi/diagrams/<name>.mermaid".
const diagramsDirRelPath = ".verdi/diagrams"

// DiscoveredProposal is one accepted `class: proposal` diagram found
// corpus-wide (DC-1), decoded and ready for regeneration/comparison: its
// frontmatter (Scope, DerivedFrom, Frozen) plus its byte-preserved mermaid
// source body.
type DiscoveredProposal struct {
	// Name is the diagram's ref name (its .mermaid filename, extension
	// stripped) — e.g. "loan-flow-v2" for diagrams/loan-flow-v2.mermaid.
	Name string
	// RelPath is the file's store-relative path, for error messages.
	RelPath     string
	Frontmatter *artifact.DiagramFrontmatter
	// Source is the mermaid body, exactly as authored (byte-preserved,
	// spec/diagram-proposals ac-2) — never rewritten by this package.
	Source string
}

// DiscoverAcceptedProposals walks root/.verdi/diagrams (every diagram-kind
// artifact file in the store, whichever spec's build is currently running)
// and returns every one whose decoded frontmatter is `class: proposal` and
// `status: accepted`, sorted by Name for determinism. Corpus-wide,
// unfiltered by any spec's own impacts: or links: (spec/alignment-section
// dc-1: "a class: proposal diagram carries no ownership edge to any spec in
// the ratified 02 diagram-artifact FRONTMATTER FIELDS").
//
// An absent diagrams/ directory (no diagram has ever been authored in this
// store) is not an error: it returns an explicit empty, non-nil slice
// (CLAUDE.md: "silence is never a pass" cuts the other way too — an empty
// result must be a disclosed, deliberate empty set, never a masked read
// failure). Any other read or decode failure is a real operational problem
// and is returned as an error rather than silently skipping the offending
// file — a class: proposal diagram that fails to decode is exactly the
// kind of corpus corruption this story's own discovery must not paper over.
func DiscoverAcceptedProposals(root string) ([]DiscoveredProposal, error) {
	dir := filepath.Join(root, ".verdi", "diagrams")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []DiscoveredProposal{}, nil
		}
		return nil, fmt.Errorf("align: discovering accepted proposal diagrams: reading %s: %w", dir, err)
	}

	out := make([]DiscoveredProposal, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mermaid") {
			continue
		}
		relPath := diagramsDirRelPath + "/" + e.Name()
		path := filepath.Join(dir, e.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("align: reading %s: %w", relPath, err)
		}
		fm, body, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("align: %s: %w", relPath, err)
		}
		diagram, err := artifact.DecodeDiagram(fm)
		if err != nil {
			return nil, fmt.Errorf("align: %s: %w", relPath, err)
		}
		if diagram.Class != artifact.DiagramClassProposal || diagram.Status != "accepted" {
			continue
		}
		out = append(out, DiscoveredProposal{
			Name:        strings.TrimSuffix(e.Name(), ".mermaid"),
			RelPath:     relPath,
			Frontmatter: diagram,
			Source:      string(body),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// IllustrativeFigure is one illustrative body figure discovered in the
// current spec's own body — a fenced ```mermaid code block (dc-8's
// body-figure convention; spec/alignment-section ac-1). Deliberately
// carries no verification tier: illustrative discovery is a wholly
// distinct concept from a class: proposal artifact's own full/partial
// coverage tier (spec/alignment-section dc-1) — a body figure is always
// rendered "unverifiable", by construction, never full or partial.
type IllustrativeFigure struct {
	// Name is a stable, ordinal label ("figure 1", "figure 2", ...) in the
	// order the figures appear in the spec body — the current spec's own
	// body carries no other identity for a bare fenced block.
	Name string
}

// illustrativeFenceOpenRe matches a fenced code block's opening line whose
// info string is exactly "mermaid" (spec/alignment-section's own problem
// statement: "every illustrative fenced-mermaid body figure"). Leading
// whitespace is tolerated (a figure may sit inside an indented list item);
// trailing whitespace on the fence line is tolerated too.
var illustrativeFenceOpenRe = regexp.MustCompile("^```mermaid\\s*$")
var illustrativeFenceCloseRe = regexp.MustCompile("^```\\s*$")

// DiscoverIllustrativeFigures scans specBody — the CURRENT spec's own
// markdown body, and ONLY that spec's body (spec/alignment-section dc-1:
// illustrative discovery "is correctly scoped to the CURRENT spec's own
// body only") — for fenced ```mermaid code blocks, returning one
// IllustrativeFigure per occurrence in body order. This function performs
// no filesystem walk and reads no other document: ac-1's no-cross-leakage
// requirement is enforced by construction, since the caller passes only the
// spec under build's own body text (see readCurrentSpecBody).
//
// Always returns a non-nil, possibly-empty slice (CLAUDE.md: "silence is
// never a pass" — an empty result is explicit, not nil-as-absence).
func DiscoverIllustrativeFigures(specBody string) []IllustrativeFigure {
	out := make([]IllustrativeFigure, 0)
	inFence := false
	n := 0
	for _, raw := range strings.Split(specBody, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case !inFence && illustrativeFenceOpenRe.MatchString(trimmed):
			inFence = true
			n++
			out = append(out, IllustrativeFigure{Name: fmt.Sprintf("figure %d", n)})
		case inFence && illustrativeFenceCloseRe.MatchString(trimmed):
			inFence = false
		}
	}
	return out
}

// readCurrentSpecBody reads the build-head spec's own body markdown
// straight from the working tree (root/.verdi/specs/active/<name>/spec.md
// — the same active-only assumption cmd/verdi/align.go's runAlignForSpec
// already makes when deriving deviation-report.md's own path, since align's
// build-branch mode only ever runs against the currently-checked-out
// active spec). SpecFrontmatter itself carries no Body field (only the
// decoded frontmatter, storyresolve.ResolveBuildSpec's own return type), so
// this re-reads the file rather than threading a new field through
// Input/ComputedInput — mirroring decision_computed.go's own precedent of
// independently re-reading a file already available on disk rather than
// widening an existing signature for one new caller.
//
// A missing file at that exact path returns ("", nil) rather than an
// error: in production the spec was only ever handed to Compute after
// storyresolve already read it from this same path (LoadActiveSpec), so
// this can only be absent in a test double that hand-builds a
// SpecFrontmatter with no backing file — a legitimate "nothing to
// discover" reading (DiscoverIllustrativeFigures("") returns the explicit
// empty slice, never silently dropping a figure that genuinely exists),
// not a masked corpus failure. Any OTHER read/decode error (permission
// denied, a malformed frontmatter delimiter) is a real operational problem
// and is returned as such.
func readCurrentSpecBody(root string, spec *artifact.SpecFrontmatter) (string, error) {
	ref, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return "", fmt.Errorf("align: resolved spec has an invalid id %q: %w", spec.ID, err)
	}
	path := filepath.Join(root, ".verdi", "specs", "active", ref.Name, "spec.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("align: reading %s: %w", path, err)
	}
	_, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return "", fmt.Errorf("align: %s: %w", path, err)
	}
	return string(body), nil
}

// DiagramAlignmentEntry is one accepted proposal's alignment outcome,
// computed alongside (never instead of) its own Finding — supporting,
// undispositioned-context data for the "### Diagram alignment" subsection,
// rendered exactly the way ServiceBoundaryDiff already is (dc-3). The two
// are deliberately redundant views of the same Compare/StaleBase result,
// mirroring how ServiceBoundaryDiff and the declared-boundary Findings both
// derive from the same regenerated contracts elsewhere in this package.
type DiagramAlignmentEntry struct {
	Name          string
	Coverage      diagramverify.Coverage
	ExcludedCount int // ambiguous identities excluded from comparison (dc-3's "N elements excluded")
	Divergent     bool
	// Deltas is one human-legible line per kept-but-gone element, its
	// candidate witness folded in when resolved (dc-3: "the schema carries
	// no separate witness field").
	Deltas []string
	// StaleBase is true when the proposal's derived_from base has moved
	// since acceptance (independent of Divergent — diagramverify's own
	// stale.go: "a proposal can be stale-base and still have every element
	// exists, or vice versa").
	StaleBase bool
}

// ComputeDiagramAlignment discovers every accepted proposal corpus-wide and
// every illustrative figure in spec's own body, regenerates and diffs each
// proposal via internal/diagramverify's shared functions (never
// reimplemented, dc-2), and returns one artifact.Finding per proposal (kind:
// computed, id "diagram-<name>") alongside the same information in
// DiagramAlignmentEntry / IllustrativeFigure shape for the "### Diagram
// alignment" subsection's own rendering (render.go). Findings are sorted by
// ID for deterministic output, matching Compute's own boundary findings.
func ComputeDiagramAlignment(ctx context.Context, root string, runner upstream.Runner, spec *artifact.SpecFrontmatter, covers string) ([]artifact.Finding, []DiagramAlignmentEntry, []IllustrativeFigure, error) {
	proposals, err := DiscoverAcceptedProposals(root)
	if err != nil {
		return nil, nil, nil, err
	}

	specBody, err := readCurrentSpecBody(root, spec)
	if err != nil {
		return nil, nil, nil, err
	}
	illustrative := DiscoverIllustrativeFigures(specBody)

	findings := make([]artifact.Finding, 0, len(proposals))
	entries := make([]DiagramAlignmentEntry, 0, len(proposals))
	for _, p := range proposals {
		finding, entry, err := computeOneProposal(ctx, root, runner, covers, p)
		if err != nil {
			return nil, nil, nil, err
		}
		findings = append(findings, finding)
		entries = append(entries, entry)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	return findings, entries, illustrative, nil
}

// computeOneProposal regenerates truth at p's declared scope and runs the
// three-way structural comparison (diagramverify.CompareWithWitness) over
// both its node-identity and edge-identity spaces, plus (for a derived
// proposal) StaleBase's independent base-digest check — every call going
// straight through diagramverify's exported functions and the SAME
// upstream.Runner Compute's own boundary-contract regeneration already
// threads through (spec/alignment-section ac-2/dc-2).
func computeOneProposal(ctx context.Context, root string, runner upstream.Runner, covers string, p DiscoveredProposal) (artifact.Finding, DiagramAlignmentEntry, error) {
	id := "diagram-" + p.Name

	g, err := diagramverify.RegenerateTruth(ctx, runner, root, covers, p.Frontmatter.Scope)
	if err != nil {
		return artifact.Finding{}, DiagramAlignmentEntry{}, fmt.Errorf("align: regenerating truth for %s: %w", p.RelPath, err)
	}
	truthFQNs := diagramverify.TruthFQNs(g)
	propExt := diagramverify.Parse(p.Source, truthFQNs)

	var baseNodeIDs, baseEdgeIDs []string
	if p.Frontmatter.DerivedFrom != nil {
		baseSource, err := resolveBaseSource(ctx, root, p.Frontmatter.DerivedFrom)
		if err != nil {
			return artifact.Finding{}, DiagramAlignmentEntry{}, err
		}
		baseExt := diagramverify.Parse(baseSource, truthFQNs)
		baseNodeIDs = baseExt.ComparableNodeIdentities()
		baseEdgeIDs = baseExt.ComparableEdgeIdentities()
	}

	truthNodes := diagramverify.TruthShortNames(g)
	truthEdges := diagramverify.TruthEdgeIdentities(g)

	// No path restriction on the pickaxe search (unlike a boundary-contract
	// witness, which is scoped to one service's own directory): a proposal
	// is corpus-wide and unowned by any single service (dc-1), so the
	// witness search covers the whole repository — a candidate, never a
	// verified cause (diagramverify dc-4), and honestly so here too.
	nodeResults, err := diagramverify.CompareWithWitness(ctx, root, propExt.ComparableNodeIdentities(), baseNodeIDs, truthNodes)
	if err != nil {
		return artifact.Finding{}, DiagramAlignmentEntry{}, fmt.Errorf("align: comparing nodes for %s: %w", p.RelPath, err)
	}
	edgeResults, err := diagramverify.CompareWithWitness(ctx, root, propExt.ComparableEdgeIdentities(), baseEdgeIDs, truthEdges)
	if err != nil {
		return artifact.Finding{}, DiagramAlignmentEntry{}, fmt.Errorf("align: comparing edges for %s: %w", p.RelPath, err)
	}

	var deltas []string
	for _, r := range nodeResults {
		if r.Classification == diagramverify.KeptButGone {
			deltas = append(deltas, formatDelta("node", r))
		}
	}
	for _, r := range edgeResults {
		if r.Classification == diagramverify.KeptButGone {
			deltas = append(deltas, formatDelta("edge", r))
		}
	}
	sort.Strings(deltas)

	stale := false
	if p.Frontmatter.DerivedFrom != nil {
		stale, _, err = diagramverify.StaleBase(ctx, runner, root, covers, p.Frontmatter.Scope, p.Frontmatter.DerivedFrom.Digest)
		if err != nil {
			return artifact.Finding{}, DiagramAlignmentEntry{}, fmt.Errorf("align: checking stale-base for %s: %w", p.RelPath, err)
		}
	}

	entry := DiagramAlignmentEntry{
		Name:          p.Name,
		Coverage:      propExt.Coverage,
		ExcludedCount: countAmbiguous(propExt),
		Divergent:     len(deltas) > 0,
		Deltas:        deltas,
		StaleBase:     stale,
	}

	return artifact.Finding{ID: id, Kind: artifact.FindingComputed, Text: diagramFindingText(entry)}, entry, nil
}

// resolveBaseSource reads a derived proposal's declared base diagram's own
// mermaid body: at df.Ref's pinned commit via git show when the ref is
// pinned (mirroring computed.go's own baselineDiffFor precedent of reading
// a historical committed file), or straight from the working tree when
// unpinned. Only the mermaid BODY is needed here (Compare's base-identity
// input); the base's own frontmatter is not otherwise consulted by this
// story.
func resolveBaseSource(ctx context.Context, root string, df *artifact.DiagramDerivedFrom) (string, error) {
	ref, err := artifact.ParseRef(df.Ref)
	if err != nil {
		return "", fmt.Errorf("align: derived_from.ref %q: %w", df.Ref, err)
	}
	relPath := diagramsDirRelPath + "/" + ref.Name + ".mermaid"

	var data []byte
	if ref.Pinned() {
		data, err = gitx.Show(ctx, root, ref.Commit, relPath)
	} else {
		data, err = os.ReadFile(filepath.Join(root, relPath))
	}
	if err != nil {
		return "", fmt.Errorf("align: reading derived_from base %s: %w", df.Ref, err)
	}
	_, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return "", fmt.Errorf("align: derived_from base %s: %w", df.Ref, err)
	}
	return string(body), nil
}

// countAmbiguous counts ext's ambiguous nodes — dc-3's "N elements excluded
// from comparison": the one concrete count Extraction itself discloses
// (an out-of-grammar CONSTRUCT downgrades Coverage too, but is not a parsed
// element this package can count without reimplementing grammar.go's own
// line classification, which dc-2 forbids).
func countAmbiguous(ext *diagramverify.Extraction) int {
	n := 0
	for _, node := range ext.Nodes {
		if node.Ambiguous {
			n++
		}
	}
	return n
}

// formatDelta renders one kept-but-gone comparison result as a single
// human-legible line, folding its candidate witness in when resolved
// (dc-3: "the schema carries no separate witness field ... folds every
// divergence delta and its candidate witness into Finding.Text").
func formatDelta(kind string, r diagramverify.Result) string {
	if r.Witness != nil {
		return fmt.Sprintf("%s %q: contradicted — truth no longer has it (candidate witness %s)", kind, r.Identity, *r.Witness)
	}
	return fmt.Sprintf("%s %q: contradicted — truth no longer has it (no candidate witness resolved)", kind, r.Identity)
}

// coverageText renders entry's coverage tier as dc-3's disclosed clause —
// "full coverage" or a partial-coverage clause naming the excluded count
// when known, so a partial-coverage proposal's clean diff never reads
// identically to a fully-verified one (spec/alignment-section dc-3, parent
// dc-3).
func coverageText(entry DiagramAlignmentEntry) string {
	if entry.Coverage == diagramverify.CoverageFull {
		return "full coverage"
	}
	if entry.ExcludedCount > 0 {
		noun := "element"
		if entry.ExcludedCount != 1 {
			noun = "elements"
		}
		return fmt.Sprintf("partial coverage — %d %s excluded from comparison", entry.ExcludedCount, noun)
	}
	return "partial coverage — source contains constructs outside the declared grammar"
}

// diagramFindingText renders one proposal's Finding.Text: realized/divergent
// plus the coverage-tier clause, always both present (dc-3: "every Finding's
// text ALSO states the proposal's full/partial coverage tier alongside
// realized/divergent"), with every delta and a stale-base note folded in.
func diagramFindingText(entry DiagramAlignmentEntry) string {
	var b strings.Builder
	if entry.Divergent {
		fmt.Fprintf(&b, "divergent (%s): %s", coverageText(entry), strings.Join(entry.Deltas, "; "))
	} else {
		fmt.Fprintf(&b, "realized (%s)", coverageText(entry))
	}
	if entry.StaleBase {
		b.WriteString(" — base has moved since acceptance (stale-base)")
	}
	return b.String()
}
