// The context-sensitive edge-type table (05 §Workbench, yarn row: "only
// the edge types legal for the (source kind, target kind) pair, each
// with a one-line consequence label ... and a confirmation step on
// gate-bearing types"). One Go table is the single source of truth: the
// server enforces it on every edge write, and the same table is embedded
// into the page for the picker — the menu can never offer what the
// server would refuse.
package workbench

import (
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
)

// targetKindOf classifies a yarn target for the legality table: an
// internal object card's kind, or the artifact kind of a reference card
// ("adr", "spec-fragment", ...).
func targetKindOf(declaredKinds map[string]string, endpoint string) string {
	if k, ok := declaredKinds[endpoint]; ok {
		return k
	}
	r, err := artifact.ParseRef(endpoint)
	if err != nil {
		return "unknown"
	}
	if r.Kind == artifact.KindSpec && r.Object != "" {
		return "spec-fragment"
	}
	return string(r.Kind)
}

// legalEdgeTypes returns the typed edges legal for a (source kind,
// target kind) pair, in menu order. The scratch tier's untyped relates
// thread is always available besides these (05 §Workbench "The scratch
// tier") and is not listed here.
//
// 02 §Object model binds the writable pairs the board can author today:
// a decision object's own links: carry supersedes/exempts edges "against
// ADRs or other decisions". Document-level story edges (implements/
// resolves) are declared at the document level, not on an object card —
// drawing those from the board is not a V1-P6 surface (they still
// PROJECT as yarn; see buildProjection).
func legalEdgeTypes(sourceKind, targetKind string) []string {
	if sourceKind != string(boardlayout.ZoneDecision) {
		return nil
	}
	switch targetKind {
	case "adr", string(boardlayout.ZoneDecision), "spec-fragment":
		// spec-fragment: a feature-decision fragment on another spec
		// (02 §Link taxonomy: exempts targets "ADR or feature-decision
		// fragment"; supersedes covers the decision replacement chain).
		return []string{string(artifact.LinkSupersedes), string(artifact.LinkExempts)}
	}
	return nil
}

// gateBearing reports whether an edge type requires the explicit
// confirmation step (05 §Workbench: "a menu misclick must not summon an
// org-wide supersession flow").
func gateBearing(edgeType string) bool {
	return edgeType == string(artifact.LinkSupersedes) || edgeType == string(artifact.LinkExempts)
}

// consequenceLabels is the one-line consequence per pickable edge type —
// calm warnings, not modal nagging (05 §Workbench's own example wording
// for supersedes), phrased as what the choice DOES to the spec document
// in words a PM reads without a glossary (owner UAT round 6, item 1).
var consequenceLabels = map[string]string{
	string(artifact.LinkSupersedes): "amends the target for everyone once accepted; requires quorum from its owners",
	string(artifact.LinkExempts):    "the target's rule keeps applying to everyone else — this spec alone is excused; its owners are notified",
	string(artifact.LinkImplements): "records that this story delivers that acceptance criterion; its owners see the claim",
	string(artifact.LinkResolves):   "records that this spike answers that open question",
	string(artifact.LinkDependsOn):  "notes the target as required background reading; gates nothing",
	"relates":                       "a scratch thread for your own thinking — never written into the spec document",
}

// removalConsequenceLabels is the same voice for taking a gate-bearing
// edge OUT of the spec document — removal mirrors creation, including
// the confirmation ritual (owner UAT round 6, item 3).
var removalConsequenceLabels = map[string]string{
	string(artifact.LinkSupersedes): "withdraws this spec's amendment claim on the target; the target stands as it was",
	string(artifact.LinkExempts):    "ends this spec's exemption — the target's rule applies to this spec again",
}
