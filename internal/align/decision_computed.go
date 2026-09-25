// Decision-conflict report: computed section (03 §Decision-conflict gate,
// "Computed section — declared-edge completeness"). See doc.go's package
// comment and decision_report.go for the mode's overall shape.
package align

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/store"
)

// ErrSpecNotInTree reports that the evaluated spec is not a decodable spec
// of the tree the computed section reads (for the gate, the report's head
// commit): nothing about its edges can be computed there.
var ErrSpecNotInTree = errors.New("align: the evaluated spec is not a decodable spec of the tree")

// ComputeDecisionEdges computes the decision-conflict report's computed
// section for the spec named spec from the records of one tree — the
// working tree for `verdi align`, the report's head commit for `verdi
// gate`'s recompute (design §5, SI-262). It is the one computation both
// share: one artifact.ConflictFinding (kind: computed) per declared
// supersedes/exempts link on every decision object of spec, in decision and
// link order, followed by the issuing successor's completeness findings —
// 03's "declared-edge completeness": a declared edge "must resolve
// (SUPERSEDED with the ratified supersession, or EXEMPT with reason) before
// the spec MR is review-ready." A finding is left undispositioned
// (Disposition == "") whenever its edge is not resolved; spec-MR
// review-readiness is then exactly "every computed finding here is
// dispositioned" (decision_report.go's DecisionReviewReady). Every
// disposition and note here is computed; none is ever carried from a prior
// report (decision_report.go).
//
// Resolution rule:
//
//   - A `supersedes` edge whose ref is an object fragment of a spec is
//     evaluated by internal/objsupersede against the tree's records (design
//     §3-§5), with est answering a carried replacement's establishment
//     (objsupersede.NewHistory in production; memoized here per successor
//     and object). A resolved result (a new or a carried replacement) is
//     SUPERSEDED, its text the core's Result.Text verbatim and its note the
//     edge it resolves ("decision <id> supersedes <ref>"); an unresolved
//     result is undispositioned, its text the reason's Result.Text.
//   - The core's completeness results for the issuing successor (a fragment
//     that a superseded conflict naming spec challenges with no matching
//     edge, design §5) are their own undispositioned computed findings,
//     after the edges, with the id completenessFindingID documents: one per
//     (conflict, challenged fragment as written), however often the
//     conflict lists that fragment.
//   - Every other edge keeps its earlier computation. A dangling edge (the
//     ref does not parse, or names no document in the tree) is ALWAYS
//     unresolved, fail-closed ("silence is never a pass"). An `exempts`
//     edge resolves (ConflictExempt) once its target exists AND the link
//     carries a non-empty Note — 03: "EXEMPT with reason"; one targeting an
//     ADR is routed to that ADR's owners (RoutedOwners, 03's CODEOWNERS
//     routing, computed here so the gate's recompute covers it). A
//     `supersedes` edge to an ADR resolves (ConflictSuperseded) once that
//     ADR's own status is "superseded" — the real supersession flow has
//     landed, not merely been declared. A `supersedes` edge to a whole spec
//     (no object fragment) never computed-resolves: no record gives a whole
//     spec a "superseded by this decision" state, so it reports unresolved
//     with a message explaining why rather than treating "target exists" as
//     good enough.
//
// An edge's finding id is "edge-<decision>-<type>-<ref>", each part
// store.RefSlug'd. An operational failure reading the tree's records, or a
// spec absent from them (ErrSpecNotInTree), is an error; a target that
// cannot be read or decoded is the finding's own unresolved text.
func ComputeDecisionEdges(ctx context.Context, tr objsupersede.TreeReader, spec string, est objsupersede.Establisher) ([]artifact.ConflictFinding, error) {
	if tr == nil || est == nil {
		return nil, fmt.Errorf("align: ComputeDecisionEdges needs a tree reader and an establisher")
	}
	if spec == "" {
		return nil, fmt.Errorf("align: ComputeDecisionEdges: spec name must not be empty")
	}
	recs, err := objsupersede.ReadRecords(ctx, tr)
	if err != nil {
		return nil, fmt.Errorf("align: %w", err)
	}
	s := recs.Specs[spec]
	if s == nil {
		detail := ""
		if len(recs.Failures) > 0 {
			detail = " (records that do not decode: " + strings.Join(recs.Failures, "; ") + ")"
		}
		return nil, fmt.Errorf("%w: spec/%s%s", ErrSpecNotInTree, spec, detail)
	}
	results, err := objsupersede.Evaluate(ctx, recs, spec, &memoEstablisher{inner: est})
	if err != nil {
		return nil, fmt.Errorf("align: %w", err)
	}

	var out []artifact.ConflictFinding
	next := 0
	for _, dc := range s.FM.Decisions {
		for _, l := range dc.Links {
			if l.Type != artifact.LinkSupersedes && l.Type != artifact.LinkExempts {
				continue
			}
			id := "edge-" + store.RefSlug(dc.ID) + "-" + store.RefSlug(string(l.Type)) + "-" + store.RefSlug(l.Ref)
			if !closedSpecObjectEdge(l) {
				out = append(out, computeOneEdge(ctx, tr, id, dc, l))
				continue
			}
			if next >= len(results) || results[next].Decision != dc.ID || results[next].Edge != l.Ref {
				return nil, fmt.Errorf("align: internal error: objsupersede did not evaluate decision %s's edge to %s in order", dc.ID, l.Ref)
			}
			f, err := resultFinding(id, results[next])
			if err != nil {
				return nil, err
			}
			out = append(out, f)
			next++
		}
	}
	// A conflict may list one challenged fragment more than once (no rule
	// refuses the repeat), and the core reports each listing: one finding
	// per (conflict, fragment as written), deduplicated before its id is
	// assigned, keeps ids unique (L4 review a, I-1).
	seen := map[string]bool{}
	for _, r := range results[next:] {
		if r.Decision != "" {
			return nil, fmt.Errorf("align: internal error: objsupersede evaluated decision %s's edge to %s, which align did not route to it", r.Decision, r.Edge)
		}
		key := r.Conflict + "\x00" + r.Edge
		if seen[key] {
			continue
		}
		seen[key] = true
		f, err := resultFinding(completenessFindingID(r), r)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// closedSpecObjectEdge reports whether l is a `supersedes` edge to a spec's
// object fragment: the edges objsupersede.Evaluate evaluates, in the same
// order (ComputeDecisionEdges checks the pairing and fails closed on any
// drift).
func closedSpecObjectEdge(l artifact.Link) bool {
	ref, err := artifact.ParseRef(l.Ref)
	return l.Type == artifact.LinkSupersedes && err == nil && ref.Kind == artifact.KindSpec && ref.Fragment()
}

// completenessFindingID is a completeness finding's stable id:
// "completeness-<conflict ref>-<challenged fragment as written>", each part
// store.RefSlug'd (conflict/x challenging spec/t#ac-1 is
// completeness-conflict--x-spec--t-ac-1), or "completeness-<reason>" for a
// result naming no conflict (records that do not decode, SI-274(6)).
// ComputeDecisionEdges emits one finding per (conflict, fragment as
// written), so a conflict repeating a challenge never repeats an id.
func completenessFindingID(r objsupersede.Result) string {
	if r.Conflict == "" {
		return "completeness-" + store.RefSlug(string(r.Reason))
	}
	return "completeness-" + store.RefSlug("conflict/"+r.Conflict) + "-" + store.RefSlug(r.Edge)
}

// resultFinding renders one objsupersede result as a computed finding.
func resultFinding(id string, r objsupersede.Result) (artifact.ConflictFinding, error) {
	text, err := r.Text()
	if err != nil {
		return artifact.ConflictFinding{}, fmt.Errorf("align: %s: %w", id, err)
	}
	f := artifact.ConflictFinding{ID: id, Kind: artifact.FindingComputed, Text: text}
	if ref, err := artifact.ParseRef(r.Edge); err == nil {
		f.TargetRef = artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
	}
	if r.Outcome == objsupersede.ResolvedNew || r.Outcome == objsupersede.ResolvedCarried {
		f.Disposition = artifact.ConflictSuperseded
		f.Note = fmt.Sprintf("decision %s supersedes %s", r.Decision, r.Edge)
	}
	return f, nil
}

// memoEstablisher answers each (successor, object) establishment once per
// computation: History.Establishment re-reads the whole store at the
// successor's acceptance commit on every call (L3 re-review OB-3), and one
// spec's carried edges can ask the same question more than once.
type memoEstablisher struct {
	inner objsupersede.Establisher
	seen  map[string]objsupersede.Establishment
}

// Establishment implements objsupersede.Establisher.
func (m *memoEstablisher) Establishment(ctx context.Context, successor string, object artifact.Ref) objsupersede.Establishment {
	key := successor + "\x00" + object.String()
	if e, ok := m.seen[key]; ok {
		return e
	}
	if m.seen == nil {
		m.seen = map[string]objsupersede.Establishment{}
	}
	e := m.inner.Establishment(ctx, successor, object)
	m.seen[key] = e
	return e
}

// computeOneEdge resolves a single declared edge that is not a closed-spec
// object edge into its computed finding.
func computeOneEdge(ctx context.Context, tr objsupersede.TreeReader, id string, dc artifact.Decision, l artifact.Link) artifact.ConflictFinding {
	ref, err := artifact.ParseRef(l.Ref)
	if err != nil {
		return artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			Text: fmt.Sprintf("decision %s %s %s: dangling — ref does not parse: %v", dc.ID, l.Type, l.Ref, err),
		}
	}
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
	target, found, resolveErr := resolveDecisionTarget(ctx, tr, ref)
	if resolveErr != nil {
		return artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			Text:      fmt.Sprintf("decision %s %s %s: could not resolve target: %v", dc.ID, l.Type, l.Ref, resolveErr),
			TargetRef: unpinned,
		}
	}
	if !found {
		return artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			Text:      fmt.Sprintf("decision %s %s %s: dangling — target does not exist in the committed corpus", dc.ID, l.Type, l.Ref),
			TargetRef: unpinned,
		}
	}

	switch l.Type {
	case artifact.LinkExempts:
		if l.Note == "" {
			return artifact.ConflictFinding{
				ID: id, Kind: artifact.FindingComputed,
				Text:      fmt.Sprintf("decision %s exempts %s: unresolved — an exempts edge requires a reason (link note)", dc.ID, unpinned),
				TargetRef: unpinned,
			}
		}
		f := artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			Text:        fmt.Sprintf("decision %s exempts %s: resolved (EXEMPT)", dc.ID, unpinned),
			Disposition: artifact.ConflictExempt,
			Note:        l.Note,
			TargetRef:   unpinned,
		}
		if len(target.owners) > 0 {
			f.RoutedOwners = append([]string(nil), target.owners...)
		}
		return f
	case artifact.LinkSupersedes:
		if target.kind != artifact.KindADR {
			return artifact.ConflictFinding{
				ID: id, Kind: artifact.FindingComputed,
				// vocab:identity — computed conflict-finding grammar / ADR status enum (frozen report text)
				Text:      fmt.Sprintf("decision %s supersedes %s: unresolved — supersedes edges targeting a non-ADR decision cannot be computed-resolved (no independent status field, 02 §Kind registry); resolve via the judged section or file a conflict directly (03 §Challenging closed decisions)", dc.ID, unpinned),
				TargetRef: unpinned,
			}
		}
		if target.status != "superseded" {
			return artifact.ConflictFinding{
				ID: id, Kind: artifact.FindingComputed,
				Text:      fmt.Sprintf("decision %s supersedes %s: unresolved — target ADR status is %q, want %q (the supersession has not landed)", dc.ID, unpinned, target.status, "superseded"),
				TargetRef: unpinned,
			}
		}
		return artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			// vocab:identity — computed conflict-finding grammar / ADR status enum (frozen report text)
			Text:        fmt.Sprintf("decision %s supersedes %s: resolved (SUPERSEDED)", dc.ID, unpinned),
			Disposition: artifact.ConflictSuperseded,
			// vocab:identity — computed conflict-finding grammar / ADR status enum (frozen report text)
			Note:      "target ADR status is superseded",
			TargetRef: unpinned,
		}
	default:
		// Unreachable: the caller only passes edges of these two types.
		return artifact.ConflictFinding{ID: id, Kind: artifact.FindingComputed, Text: fmt.Sprintf("decision %s: internal error: unhandled edge type %q", dc.ID, l.Type)}
	}
}

// decisionTarget is a declared edge's resolved target: its artifact kind
// and, for an ADR, its status and owners.
type decisionTarget struct {
	kind   artifact.Kind
	status string
	owners []string
}

// resolveDecisionTarget reads ref's target document from tr — the working
// tree for align, the head commit for the gate's recompute. found is false
// for a dangling ref (no such file); a non-nil error is a real failure (a
// file exists but cannot be read or fails to decode) — never conflated
// with "dangling", per CLAUDE.md's "silence is never a pass".
func resolveDecisionTarget(ctx context.Context, tr objsupersede.TreeReader, ref artifact.Ref) (decisionTarget, bool, error) {
	switch ref.Kind {
	case artifact.KindADR:
		p := filepath.ToSlash(store.ADRPath("", ref.Name))
		data, found, err := readTreeFile(ctx, tr, p)
		if err != nil || !found {
			return decisionTarget{}, false, err
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return decisionTarget{}, false, fmt.Errorf("%s: %w", p, err)
		}
		adr, err := artifact.DecodeADR(fm)
		if err != nil {
			return decisionTarget{}, false, fmt.Errorf("%s: %w", p, err)
		}
		return decisionTarget{kind: artifact.KindADR, status: string(adr.Status), owners: adr.Owners}, true, nil

	case artifact.KindSpec:
		spec, err := readTreeSpec(ctx, tr, ref.Name)
		if err != nil || spec == nil {
			return decisionTarget{}, false, err
		}
		if ref.Fragment() && !specDeclaresObject(spec, ref.Object) {
			return decisionTarget{}, false, nil
		}
		return decisionTarget{kind: artifact.KindSpec}, true, nil

	default:
		// Any other kind (diagram, attestation, waiver, conflict,
		// reaffirmation) is not a legal decision-edge target per 03
		// §Decision-conflict gate's three tiers (ADRs, spec-scoped
		// decisions, story decisions) — treated as dangling rather than a
		// silently-accepted resolve.
		return decisionTarget{}, false, nil
	}
}

// readTreeSpec reads spec/<name> from tr's active zone, else its archive
// zone (a spec target may legitimately live in either — an accepted or
// closed spec's decisions remain valid targets), returning nil when neither
// exists (dangling).
func readTreeSpec(ctx context.Context, tr objsupersede.TreeReader, name string) (*artifact.SpecFrontmatter, error) {
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		p := store.SpecRelPath(zone, name)
		data, found, err := readTreeFile(ctx, tr, p)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		return spec, nil
	}
	return nil, nil
}

// readSpecByName looks for name under both specs/active/ and
// specs/archive/ of the working tree at root (a spec may legitimately live
// in either), returning (nil, "", nil) when neither exists: the judged
// sweeps' reader (decision_judge.go, diagram_judge.go); the computed
// section reads through readTreeSpec.
func readSpecByName(root, name string) (*artifact.SpecFrontmatter, string, error) {
	for _, statusDir := range []string{"active", "archive"} {
		path := store.SpecPath(root, statusDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("reading %s: %w", path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", path, err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", path, err)
		}
		return spec, path, nil
	}
	return nil, "", nil
}

// readTreeFile reads the repo-relative path p from tr. found is false when
// the tree holds nothing at p; an entry at p that is not a regular file, or
// a path through a link, is an error, never followed.
func readTreeFile(ctx context.Context, tr objsupersede.TreeReader, p string) ([]byte, bool, error) {
	files, err := tr.Files(ctx, p)
	if err != nil {
		return nil, false, err
	}
	if len(files) == 0 {
		return nil, false, nil
	}
	if len(files) != 1 || files[0].Path != p || !files[0].Regular {
		return nil, false, fmt.Errorf("%s is not a regular file", p)
	}
	data, err := tr.ReadFile(ctx, p)
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", p, err)
	}
	return data, true, nil
}

// specDeclaresObject reports whether spec declares a frontmatter object
// (acceptance criterion, constraint, decision, or open question) with the
// given id — the fragment-ref resolution rule VL-003 already applies
// (lint/vl003.go's declaredObjectIDs), reimplemented here in miniature
// rather than imported: internal/lint depends on nothing this phase's
// touch surface should couple internal/align to, and the check itself is a
// handful of lines.
func specDeclaresObject(spec *artifact.SpecFrontmatter, id string) bool {
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == id {
			return true
		}
	}
	for _, c := range spec.Constraints {
		if c.ID == id {
			return true
		}
	}
	for _, dc := range spec.Decisions {
		if dc.ID == id {
			return true
		}
	}
	for _, q := range spec.OpenQuestions {
		if q.ID == id {
			return true
		}
	}
	return false
}
