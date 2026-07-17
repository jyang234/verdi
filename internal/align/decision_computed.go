// Decision-conflict report: computed section (03 §Decision-conflict gate,
// "Computed section — declared-edge completeness"). See doc.go's package
// comment and decision_report.go for the mode's overall shape.
package align

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// ComputeDecisionEdges computes one artifact.ConflictFinding (kind:
// computed) per declared supersedes/exempts link on every decision object
// in spec.Decisions — 03's "declared-edge completeness": a declared edge
// "must resolve (SUPERSEDED with the ratified supersession, or EXEMPT with
// reason) before the spec MR is review-ready." A finding is left
// undispositioned (Disposition == "") whenever its edge is not yet
// resolved — spec-MR review-readiness is then exactly "every computed
// finding here is dispositioned" (decision_report.go's DecisionReviewReady),
// mirroring the build-branch report's own computed/disposition shape
// (computed.go).
//
// Resolution rule (a documented, disclosed judgment call — 03 names the
// two outcomes but not the mechanics; see the phase report):
//
//   - A dangling edge (the target ref does not parse, or names no
//     document that exists in the committed corpus) is ALWAYS unresolved,
//     fail-closed, regardless of edge type (PLAN.md/CLAUDE.md: "silence is
//     never a pass") — the exit criterion's "a dangling declared edge
//     fails closed".
//   - An `exempts` edge resolves (ConflictExempt) once its target exists
//     AND the link carries a non-empty Note — 03: "EXEMPT with reason";
//     the reason itself, not a change on the target's side, is the
//     resolution signal, since an exemption "does not invalidate the
//     default."
//   - A `supersedes` edge resolves (ConflictSuperseded) once its target
//     exists AND is an ADR whose own Status is "superseded" — i.e. the
//     real supersession flow (§Challenging closed decisions' two-Code-
//     Owner quorum ritual) has actually landed, not merely been declared.
//     A supersedes edge targeting a non-ADR (a spec-scoped decision
//     fragment) is a disclosed, scoped-out gap: Decision (object.go)
//     carries no independent status field a computed check could read —
//     no 02 kind carries a "this decision is superseded" state — so such
//     an edge can never resolve by computed means in this phase; it
//     always reports unresolved with a message explaining why, rather
//     than silently treating "target exists" as good enough (which would
//     let a merely-declared, never-ratified supersession pass the gate).
func ComputeDecisionEdges(root string, spec *artifact.SpecFrontmatter) ([]artifact.ConflictFinding, error) {
	if spec == nil {
		return nil, fmt.Errorf("align: ComputeDecisionEdges: spec is required")
	}
	if root == "" {
		return nil, fmt.Errorf("align: ComputeDecisionEdges: root must not be empty")
	}

	var out []artifact.ConflictFinding
	for _, dc := range spec.Decisions {
		for _, l := range dc.Links {
			if l.Type != artifact.LinkSupersedes && l.Type != artifact.LinkExempts {
				continue
			}
			out = append(out, computeOneEdge(root, dc, l))
		}
	}
	return out, nil
}

// computeOneEdge resolves a single declared edge into its computed finding.
func computeOneEdge(root string, dc artifact.Decision, l artifact.Link) artifact.ConflictFinding {
	id := "edge-" + store.RefSlug(dc.ID) + "-" + store.RefSlug(string(l.Type)) + "-" + store.RefSlug(l.Ref)

	ref, err := artifact.ParseRef(l.Ref)
	if err != nil {
		return artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			Text: fmt.Sprintf("decision %s %s %s: dangling — ref does not parse: %v", dc.ID, l.Type, l.Ref, err),
		}
	}
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
	target, found, resolveErr := resolveDecisionTarget(root, ref)
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
		return artifact.ConflictFinding{
			ID: id, Kind: artifact.FindingComputed,
			Text:        fmt.Sprintf("decision %s exempts %s: resolved (EXEMPT)", dc.ID, unpinned),
			Disposition: artifact.ConflictExempt,
			Note:        l.Note,
			TargetRef:   unpinned,
		}
	case artifact.LinkSupersedes:
		if target.kind != artifact.KindADR {
			return artifact.ConflictFinding{
				ID: id, Kind: artifact.FindingComputed,
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
			Text:        fmt.Sprintf("decision %s supersedes %s: resolved (SUPERSEDED)", dc.ID, unpinned),
			Disposition: artifact.ConflictSuperseded,
			Note:        "target ADR status is superseded",
			TargetRef:   unpinned,
		}
	default:
		// Unreachable: the caller only appends edges of these two types.
		return artifact.ConflictFinding{ID: id, Kind: artifact.FindingComputed, Text: fmt.Sprintf("decision %s: internal error: unhandled edge type %q", dc.ID, l.Type)}
	}
}

// decisionTarget is a declared edge's resolved target: its artifact kind
// and (for an ADR) its status.
type decisionTarget struct {
	kind   artifact.Kind
	status string
}

// resolveDecisionTarget reads ref's target document straight from the
// working tree (the design branch's own committed zone — align always
// operates on the checked-out tree, matching computed.go's own precedent
// of reading local files rather than a snapshot). found is false for a
// dangling ref (no such file); a non-nil error is a real operational
// failure (a file exists but fails to decode) — never conflated with
// "dangling", per CLAUDE.md's "silence is never a pass".
func resolveDecisionTarget(root string, ref artifact.Ref) (decisionTarget, bool, error) {
	switch ref.Kind {
	case artifact.KindADR:
		path := filepath.Join(root, ".verdi", "adr", ref.Name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return decisionTarget{}, false, nil
			}
			return decisionTarget{}, false, fmt.Errorf("reading %s: %w", path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return decisionTarget{}, false, fmt.Errorf("%s: %w", path, err)
		}
		adr, err := artifact.DecodeADR(fm)
		if err != nil {
			return decisionTarget{}, false, fmt.Errorf("%s: %w", path, err)
		}
		return decisionTarget{kind: artifact.KindADR, status: string(adr.Status)}, true, nil

	case artifact.KindSpec:
		spec, path, err := readSpecByName(root, ref.Name)
		if err != nil {
			return decisionTarget{}, false, err
		}
		if spec == nil {
			return decisionTarget{}, false, nil
		}
		if ref.Fragment() {
			if !specDeclaresObject(spec, ref.Object) {
				return decisionTarget{}, false, nil
			}
		}
		_ = path
		return decisionTarget{kind: artifact.KindSpec, status: string(spec.Status)}, true, nil

	default:
		// Any other kind (diagram, attestation, waiver, conflict,
		// reaffirmation) is not a legal decision-edge target per 03
		// §Decision-conflict gate's three tiers (ADRs, spec-scoped
		// decisions, story decisions) — treated as dangling rather than a
		// silently-accepted resolve.
		return decisionTarget{}, false, nil
	}
}

// readSpecByName looks for name under both specs/active/ and
// specs/archive/ (a spec target may legitimately live in either — an
// accepted/closed spec's decisions remain valid supersedes/exempts
// targets), returning (nil, "", nil) when neither exists (dangling).
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
