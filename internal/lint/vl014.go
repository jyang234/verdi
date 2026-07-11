package lint

import (
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

// vl014 enforces "disposition completeness, bidirectional, grandfathered
// (R4-I-9(a)): fires only on specs carrying a dispositions: block (pre-R4
// artifacts) — every sticky id in a committed board.json appears there as
// incorporated ... contradicted ... or open-question, and every entry
// names a real board sticky. New specs use VL-017 instead" (02 §Lint
// rules, as amended, R4-I-9). The grandfather scope guard below is the
// rescope itself: a new-style spec's board state and spec are free to
// disagree (VL-017 owns readiness for those going forward) — only a spec
// that still carries a dispositions: block (a pre-R4 artifact, or one
// deliberately grandfathered) is checked here at all, in two parts:
//
//   - per-entry validity — incorporated requires a where anchor that
//     resolves to a heading in the spec's own body, contradicted requires
//     a note — checked for every dispositions:-carrying spec, independent
//     of whether a board.json sibling exists (these are properties of the
//     entry itself, not a board/spec join);
//   - bidirectional board reconciliation (a board sticky with no
//     disposition, a disposition naming no real sticky) — checked only for
//     a spec directory that has a committed board.json: 02's own wording
//     ("every sticky id in a committed board.json") has no subject to
//     check when no board.json exists at all.
type vl014 struct{}

func (vl014) ID() string { return "VL-014" }

func (r vl014) Check(in *RunInput) []Finding {
	var findings []Finding

	boardBySpecDir := make(map[string]*Board, len(in.Snapshot.Boards))
	for _, b := range in.Snapshot.Boards {
		boardBySpecDir[b.SpecDir] = b
		if b.DecodeErr != nil {
			findings = append(findings, Finding{Rule: "VL-014", Path: b.RelPath, Message: fmt.Sprintf("board.json does not decode: %v", b.DecodeErr)})
		}
	}

	for _, spec := range in.Snapshot.Docs {
		if spec.Grandfathered || spec.DecodeErr != nil || spec.Spec == nil {
			continue
		}
		if len(spec.Spec.Dispositions) == 0 {
			// R4-I-9: VL-014 is retained but scoped to specs still carrying a
			// dispositions: block; new specs use VL-017 instead, even when a
			// stale/leftover board.json sibling disagrees with this spec.
			continue
		}
		findings = append(findings, r.checkEntries(spec)...)

		if b, ok := boardBySpecDir[specDirOf(spec)]; ok && b.DecodeErr == nil {
			findings = append(findings, r.reconcile(spec, b)...)
		}
	}

	return findings
}

// checkEntries validates every disposition entry's own shape, independent
// of any board.json.
func (vl014) checkEntries(spec *Document) []Finding {
	var findings []Finding
	anchors := headingAnchors(spec.Body)

	for _, d := range spec.Spec.Dispositions {
		switch d.Disposition {
		case artifact.DispositionIncorporated:
			if d.Where == "" {
				findings = append(findings, Finding{Rule: "VL-014", Path: spec.RelPath, Message: fmt.Sprintf("sticky %s is incorporated but has no where anchor", d.Sticky)})
			} else if !resolveAnchor(anchors, d.Where) {
				findings = append(findings, Finding{Rule: "VL-014", Path: spec.RelPath, Message: fmt.Sprintf("sticky %s's where anchor %q does not resolve to a heading in this spec", d.Sticky, d.Where)})
			}
		case artifact.DispositionContradicted:
			if d.Note == "" {
				findings = append(findings, Finding{Rule: "VL-014", Path: spec.RelPath, Message: fmt.Sprintf("sticky %s is contradicted but has no note", d.Sticky)})
			}
		case artifact.DispositionOpenQuestion:
			// no per-value required field
		default:
			findings = append(findings, Finding{Rule: "VL-014", Path: spec.RelPath, Message: fmt.Sprintf("sticky %s has unknown disposition value %q", d.Sticky, d.Disposition)})
		}
	}
	return findings
}

// reconcile checks bidirectional completeness between a spec's
// dispositions: block and its sibling board.json's stickies.
func (vl014) reconcile(spec *Document, b *Board) []Finding {
	var findings []Finding

	stickies := make(map[string]bool, len(b.Board.Stickies))
	for _, s := range b.Board.Stickies {
		stickies[s.ID] = true
	}
	dispositioned := make(map[string]bool, len(spec.Spec.Dispositions))

	for _, d := range spec.Spec.Dispositions {
		dispositioned[d.Sticky] = true
		if !stickies[d.Sticky] {
			findings = append(findings, Finding{Rule: "VL-014", Path: spec.RelPath, Message: fmt.Sprintf("dispositions[] names sticky %q, which is not a real sticky in %s", d.Sticky, b.RelPath)})
		}
	}

	for id := range stickies {
		if !dispositioned[id] {
			findings = append(findings, Finding{Rule: "VL-014", Path: spec.RelPath, Message: fmt.Sprintf("board sticky %q in %s has no dispositions[] entry", id, b.RelPath)})
		}
	}

	return findings
}
