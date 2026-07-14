package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// vl021 enforces spec/proposal-artifact ac-5: a class: proposal diagram's
// derived_from must resolve to something real. id/path agreement for the
// diagrams/ kind is already generic and class-blind (VL-002's
// singleFileKindDir["diagram"] entry, vl002.go, unconditional on class),
// and class/status enum agreement is already enforced at strict-decode
// time (a DecodeErr, surfaced by VL-001's baseline decode-cleanliness
// check every kind gets) — so neither needs new coverage here (spec/
// proposal-artifact ac-5's own text). This rule's entire scope is the one
// genuinely new, corpus-aware check the class needs: derived_from.ref
// must resolve to a real diagram artifact in the corpus, and
// derived_from.digest must have the sha256:<64-hex> shape
// (artifact.ValidDigest). Every refusal names the offending field
// (D6-18: never a silent absence).
//
// internal/artifact/diagram.go's DiagramDerivedFrom.Validate deliberately
// does NOT check either of these two things at decode time (see its own
// doc comment): a dangling ref or a malformed digest must still decode
// cleanly so this rule — not decode — is the one that catches them, which
// is exactly what this rule's own fixture test proves (vl021_test.go).
type vl021 struct{}

func (vl021) ID() string { return "VL-021" }

func (vl021) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Diagram == nil {
			continue
		}
		if d.Diagram.Class != artifact.DiagramClassProposal || d.Diagram.DerivedFrom == nil {
			continue
		}
		df := d.Diagram.DerivedFrom

		if !resolvesToDiagram(in.Snapshot, df.Ref) {
			findings = append(findings, Finding{Rule: "VL-021", Path: d.RelPath, Message: fmt.Sprintf("derived_from.ref %q does not resolve to a real diagram artifact in the corpus", df.Ref)})
		}
		if !artifact.ValidDigest(df.Digest) {
			findings = append(findings, Finding{Rule: "VL-021", Path: d.RelPath, Message: fmt.Sprintf("derived_from.digest %q is not sha256:<64-hex>", df.Digest)})
		}
	}
	return findings
}

// resolvesToDiagram reports whether ref (already known to parse as SOME
// ref — DiagramDerivedFrom.Validate checked that much at decode time)
// names a real diagram-kind artifact in the corpus, ignoring any pin it
// may carry: a derived proposal's base ref is typically pinned to its
// fork commit (02 §Diagram proposals' own example), but existence is
// checked against the target's own unpinned id — the only form
// Base.ID / Snapshot.ByRef ever indexes (validateBase forbids a pinned
// id).
func resolvesToDiagram(snap *Snapshot, ref string) bool {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Kind != artifact.KindDiagram {
		return false
	}
	unqualified := (artifact.Ref{Kind: r.Kind, Name: r.Name}).String()
	docs, ok := snap.ByRef[unqualified]
	if !ok {
		return false
	}
	for _, d := range docs {
		if d.Kind == "diagram" {
			return true
		}
	}
	return false
}
