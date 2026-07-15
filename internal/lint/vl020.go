package lint

import (
	"fmt"
	"path"

	"github.com/jyang234/verdi/internal/artifact"
)

// vl020 enforces spec/obligation-gate ac-1/ac-2 (spec/evidence-obligations
// ac-2/dc-2): the obligation-shaped sibling of VL-006. Where VL-006 refuses
// an AC that declares NO evidence kind at all, VL-020 refuses a STORY AC
// that declares a kind with no matching first-class obligation artifact —
// "a spec may not be accepted saying what KIND of evidence it wants without
// an obligation stating what that evidence must specifically show"
// (spec/obligation-gate outcome). For every (ac, kind) pair a story AC
// declares, an obligation must exist at
// .verdi/obligations/<spec-name>/<ac-id>--<kind>.md — the spec-name keying
// spec/obligation-artifact settled (DC-2 there) and the same convention
// internal/workbench/obligationauthor.go's actionObligationGraduate already
// writes to (its `dir := filepath.Join(s.root, ".verdi", "obligations",
// name)`, where name is the wall's own spec directory name) and VL-011/
// VL-019 already read. Every missing pair is its own Finding, naming both
// the ac and the kind (D6-18: never a silent absence — mirroring VL-019's
// own "always name the offending target").
//
// Existence only: this rule checks that a document classified kind
// "obligation" sits at the implied path, exactly the shape
// spec/obligation-gate's ac-1 asks for ("an obligation artifact must
// exist ... with the obligation present, it passes"). It does not
// re-validate that file's own content (id/for_kind/path self-agreement is
// artifact.ObligationFrontmatter.Validate's job; path-implies-id is
// VL-011's; the verifies target's class and AC membership is VL-019's) —
// each rule owns exactly the semantic slice 02 assigns it (vl019.go's own
// doc note), and a malformed-but-present obligation already surfaces via
// VL-001/011/019 without this rule piling on a redundant finding.
//
// Scoping (dc-3): only STORY-class specs are checked — a feature (or
// component) AC never requires an obligation, mirroring VL-019's own class
// resolution (badVerifiesTarget refuses an obligation whose verifies target
// is not a STORY) and evidence-obligations DC-3 ("obligations attach to
// STORY ACs only ... note that this feature's OWN ACs, being feature ACs,
// therefore carry no obligations").
//
// Timing (co-2 / evidence-obligations co-2): "a draft story with an
// un-obligated kind is not refused for that reason; the refusal is reserved
// for the accept / activation path" — d.Status == "draft" is tolerated.
//
// Disclosed reading (CLAUDE.md provenance discipline: recorded here rather
// than silently resolved): this is NOT literally vl006.go's own runtime
// condition. Read closely, vl006.go's Check carries no status/draft branch
// at all — its evidence-kind-declared check (and, for new-class specs, the
// requiredness check) fires unconditionally on every non-grandfathered,
// cleanly-decoded spec regardless of status; vl006_test.go's own fixtures
// are ALL status: draft and still fire. The two specs' prose cites VL-006 as
// precedent for "this is an activation-completeness lint" (02 §Lint rules
// literally labels VL-006 "(activation lint)"), not as a literal claim that
// vl006.go itself gates on status. Since spec/obligation-gate's own co-2
// states the draft-tolerance requirement directly and unconditionally ("a
// draft is never refused... the refusal is reserved for the accept /
// activation path"), this rule honors that authoritative text over a
// stricter (draft-inclusive) literal mirror of vl006.go's code.
//
// See obligationGateBaseline below for the second, larger disclosed
// judgment call this rule required: without it, VL-020 would refuse every
// story spec already in this repository's own store (none of which predate
// this rule are exempt any other way), redding lint-store/spec-align on
// this story's own merge.
type vl020 struct{}

func (vl020) ID() string { return "VL-020" }

func (vl020) Check(in *RunInput) []Finding {
	existingObligations := make(map[string]bool)
	for _, d := range in.Snapshot.Docs {
		if d.Kind == "obligation" {
			existingObligations[d.RelPath] = true
		}
	}

	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil {
			continue
		}
		if d.Spec.Class != artifact.ClassStory {
			continue // dc-3: feature (and component) ACs never require an obligation
		}
		if d.Status == "draft" {
			continue // co-2: authoring is never blocked; the gate is at activation
		}

		specName := path.Base(specDirOf(d))
		if obligationGateBaseline[specName] {
			continue // disclosed pre-existing-corpus exemption — see the map's doc comment
		}

		for _, ac := range d.Spec.AcceptanceCriteria {
			for _, kind := range ac.Evidence {
				wantPath := fmt.Sprintf(".verdi/obligations/%s/%s--%s.md", specName, ac.ID, kind)
				if existingObligations[wantPath] {
					continue
				}
				findings = append(findings, Finding{Rule: "VL-020", Path: d.RelPath, Message: fmt.Sprintf("acceptance criterion %s declares evidence kind %s with no obligation at %s", ac.ID, kind, wantPath)})
			}
		}
	}
	return findings
}

// obligationGateBaseline is a ONE-TIME, DISCLOSED exemption — an invention,
// not spec-mandated, recorded here per CLAUDE.md's provenance discipline
// ("never resolve a spec ambiguity silently ... choose the smallest
// reversible option") rather than silently picked or silently skipped.
//
// The tension: spec/obligation-gate's outcome says obligations gate "NEW
// specs going forward"; the existing corpus predates obligations. Unlike
// VL-006's own isNewClassSpec discriminator — which can tell new-vintage
// from grandfathered v0 content apart because round-four surface fields are
// SCHEMA additions a v0 spec.md structurally cannot carry — an obligation is
// an EXTRINSIC file, not a spec.md field. Nothing on a story spec's own
// frontmatter distinguishes "authored before obligations existed" from
// "authored after, but the obligation was simply never written"; the two
// are byte-for-byte indistinguishable. At this rule's introduction, EVERY
// story spec in this store is already accepted-pending-build, superseded,
// or closed (none is status: draft) and ZERO .verdi/obligations/ files
// exist anywhere (the directory itself is absent) — so an unconditional
// story-scoped gate would refuse 37 pre-existing (ac, kind) pairs across
// the 10 specs below in one commit, redding make lint-store and
// internal/specalign's own no-error-severity-findings check for the whole
// repository — exactly the outcome this rule's own brief says must not
// happen.
//
// Two other resolutions were considered and rejected: (a) retroactively
// authoring ~37 obligation artifacts — rejected as a large, easily-
// fabricated-content undertaking far outside this story's own "a new VL
// lint rule" scope, and a poor way to produce obligations whose entire
// point is to state IN GOOD FAITH what evidence must specifically show; (b)
// a wholly dormant enforcement flag (mirroring Options.GrandfatherArchive's
// own off-by-default precedent) — rejected because a rule whose title is
// "makes evidence obligations mandatory" must not default to enforcing
// nothing at all.
//
// This finite, named, auditable list — the same shape a real lint rollout
// uses (a "baseline"/"nolint" migration list) — is the smallest reversible
// alternative: it exempts only the specs enumerated below, by name, as they
// stood when this rule was introduced. It does NOT extend to any spec
// authored after this list is frozen — a brand-new story spec's
// un-obligated kind is refused from day one, exactly as ac-1/ac-2 require,
// which is exactly what this rule's own table-driven tests prove (they use
// synthetic spec names that are never in this map). Shrinking, then
// deleting, this map as the pre-existing corpus's obligations get
// backfilled (e.g. via obligation-wall's wave-3 wall-authoring loop) is the
// intended follow-up. Flagged for review: if endorsed, this judgment call
// belongs in PLAN.md's invention ledger rather than standing only as this
// comment.
//
// (disclosure-enumeration-spike, also pre-existing and story-class, needs
// no entry: it is a spike with zero declared acceptance_criteria, so the
// per-AC loop above never visits it regardless.)
//
// The last two entries (borrower-update-api, borrower-update-mobile) are a
// SEPARATE provenance: not this repository's own real store, but
// examples/showcase's round-four "v2 fixture corpus" (internal/lint's
// v2clean_test.go, internal/artifact's own v2fixture_test.go) — golden
// fixture files several packages chain into git-real repos and cite by
// exact, precomputed commit SHA (goldenShaA/B, goldenHeads[2], etc., per
// v2clean_test.go's own doc comment). Editing those files in place — even
// just adding a sibling obligation file to the same fixturegit layer —
// changes that layer's tree and therefore its commit SHA, invalidating
// every hardcoded pin across those packages; that ripple is far larger than
// this story's own scope. Exempting the two story specs by name here, the
// same as the real corpus above, is the smaller, fully self-contained fix:
// both predate evidence-obligations by multiple rounds and carry the exact
// same "authored before the concept existed" justification.
var obligationGateBaseline = map[string]bool{
	"disclosure-seam-v2":         true, // 2 (ac,kind) pairs, accepted-pending-build
	"disclosure-seam":            true, // 1 pair, superseded
	"disclosures-panel":          true, // 3 pairs, accepted-pending-build
	"close-verb":                 true, // 6 pairs, closed (archive)
	"feature-supersession-state": true, // 3 pairs, closed (archive)
	"remote-and-ci":              true, // 6 pairs, closed (archive)
	"runtime-evidence":           true, // 4 pairs, closed (archive)

	"borrower-update-api":    true, // 2 pairs; examples/showcase v2 fixture corpus (SHA-pinned, see above)
	"borrower-update-mobile": true, // 3 pairs; examples/showcase v2 fixture corpus (SHA-pinned, see above)
}
