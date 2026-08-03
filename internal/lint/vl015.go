package lint

import (
	"fmt"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// vl015 enforces "supersession manifest completeness and fidelity: every
// object in the predecessor spec is classified exactly once across the
// superseding revision's supersession: block
// (carried/amended/amended_advisory/removed, plus added); every carried
// object's (kind, id, text) content is byte-identical to its predecessor
// (§Object model) — fail closed on drift" (02 §Lint rules; R4-I-4). The
// predecessor's own object manifest is read from real git history, never
// the live working-tree document, at one of two commits depending on the
// predecessor's own lifecycle shape:
//
//   - a predecessor carrying a legacy frozen: stamp is read at its
//     frozen.commit (gitx.Show) — a frozen file is immutable (VL-010) so
//     the two should already agree, but VL-015 itself proves it rather than
//     assuming it;
//   - a predecessor with NO frozen stamp — the ratified merge-signaled
//     lifecycle's statusless shape, which intentionally has none — is read
//     at its Git-derived accepted baseline's first-parent landing commit,
//     resolved through the shared internal/specstate.Projector
//     (engine.go's SpecStateResolver) exactly like vl004.go's compatibility
//     disclosure: a PROVEN baseline (AcceptedPendingBuild, Superseded, or
//     Closed, with a complete Path/Blob/LandingCommit identity) is
//     required, and any other resolution (proposed, unproven, or an
//     operational failure) fails VL-015 closed rather than silently
//     falling back to frozen-only or working-tree bytes.
//
// Both paths converge on the same read+decode+validate step once a commit
// is selected (readPredecessorManifestAt) — the completeness/byte-identity
// check below has no idea which path supplied predSpec.
type vl015 struct{}

func (vl015) ID() string { return "VL-015" }

func (r vl015) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil || d.Spec.Supersession == nil {
			continue
		}
		// badge-computes dc-3's "spec-stale adjacent rules" spec-level
		// bucket: supersession-manifest fidelity is a whole-revision
		// property (every predecessor object classified exactly once), not
		// any single card's own defect, even where a message names one
		// object id among many it checks.
		findings = append(findings, locusAll(r.checkOne(in, d), SpecLocus())...)
	}
	return findings
}

// checkOne runs VL-015 for one superseding revision d.
func (vl015) checkOne(in *RunInput, d *Document) []Finding {
	// I-47 (PLAN.md §7): the manifest is a statement about ONE named
	// predecessor's object set, so the revision must name exactly one
	// whole-spec, spec-kind predecessor. Zero leaves the manifest
	// un-anchored; two or more make it ambiguous — VL-015 used to validate
	// against whichever supersedes link happened to come FIRST, silently
	// leaving every other named predecessor an unchecked supersession claim
	// (which internal/specstate then turned into an unearned terminal
	// Superseded verdict). Reported and returned here rather than guessed
	// past: with no determinate predecessor there is no object manifest to
	// check completeness or byte-identity against.
	//
	// internal/artifact enforces the same invariant at decode time
	// (validateSupersessionPredecessor) — that is the primary, fail-closed
	// signal, and it is what internal/specstate relies on. lint carries its
	// own copy of the CHECK (the design note in doc.go: every semantic rule
	// is re-implemented against the raw decoded struct rather than by
	// calling a kind's Validate()) but shares the DEFINITION of "whole-spec
	// predecessor" through artifact.WholeSpecSupersedesRefs.
	predRefs := artifact.WholeSpecSupersedesRefs(d.Base.Links)
	if len(predRefs) != 1 {
		return []Finding{{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf(
			"supersession: block is present but the revision names %d whole-spec predecessors via links: {type: supersedes} — exactly one is required, since the manifest classifies exactly one predecessor's objects (02 §Lint rules VL-015, §Kind registry, §Link taxonomy; I-47). A fragment supersedes edge (spec/x#object-id) is a decision-level override and does not count",
			len(predRefs))}}
	}
	predRef := predRefs[0].String()

	predDocs, ok := in.Snapshot.ByRef[predRef]
	if !ok || len(predDocs) == 0 || predDocs[0].Spec == nil {
		// VL-003 already flags a dangling supersedes ref; nothing more this
		// rule can check without a resolved predecessor.
		return nil
	}
	pred := predDocs[0]

	var (
		predSpec *artifact.SpecFrontmatter
		errFind  *Finding
	)
	if pred.Base.Frozen != nil {
		predSpec, errFind = readPredecessorManifestAt(in, d, predRef, pred.RelPath, predecessorManifestSource{
			commit: pred.Base.Frozen.Commit,
			label:  "its frozen commit",
		})
	} else {
		predSpec, errFind = resolveStatuslessPredecessorManifest(in, d, predRef, pred)
	}
	if errFind != nil {
		return []Finding{*errFind}
	}

	predObjects := specObjects(predSpec)
	succObjects := specObjects(d.Spec)
	sup := d.Spec.Supersession

	classified := map[string]int{}
	for _, id := range sup.Carried {
		classified[id]++
	}
	for _, n := range sup.Amended {
		classified[n.ID]++
	}
	for _, n := range sup.AmendedAdvisory {
		classified[n.ID]++
	}
	for _, n := range sup.Removed {
		classified[n.ID]++
	}

	var findings []Finding
	for id := range predObjects {
		switch classified[id] {
		case 0:
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor object %s is not classified anywhere in supersession: (carried/amended/amended_advisory/removed)", id)})
		case 1:
			// exactly once: fine
		default:
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor object %s is classified more than once across supersession: buckets", id)})
		}
	}
	for id := range classified {
		if _, ok := predObjects[id]; !ok {
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("supersession: names %s, which is not an object %s declares", id, predRef)})
		}
	}

	for _, id := range sup.Carried {
		predObj, ok := predObjects[id]
		if !ok {
			continue // already flagged above
		}
		succObj, ok := succObjects[id]
		if !ok {
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("carried object %s is classified carried but is not declared on this revision at all", id)})
			continue
		}
		if succObj.Kind != predObj.Kind || succObj.Text != predObj.Text {
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("carried object %s content drifted from its predecessor — byte-identical required (02 §Object model): predecessor=%q successor=%q", id, predObj.Text, succObj.Text)})
		}
	}

	return findings
}

// predecessorManifestSource names WHICH commit checkOne selected as the
// predecessor's manifest read point, and how to describe it in a Finding
// message — purely prose: readPredecessorManifestAt's own read+split+decode
// steps are identical regardless of which path chose commit.
type predecessorManifestSource struct {
	// commit is the git-resolvable commit (or ref) to read pred's own
	// content at.
	commit string
	// label names commit's provenance in every finding message this
	// produces, e.g. "its frozen commit" or "its Git-derived accepted
	// baseline's landing commit" — always read as "predecessor %s at
	// <label> <commit>: ...".
	label string
}

// readPredecessorManifestAt is VL-015's single shared read+decode+validate
// step: both the legacy frozen.commit path and the merge-signaled
// Git-derived-baseline path pick their own commit and then converge here
// (single responsibility; the commit-selection logic differs, the
// validation of what that commit holds does not — no copy-pasted
// gitx.Show/SplitFrontmatter/DecodeStrict block per path).
func readPredecessorManifestAt(in *RunInput, d *Document, predRef, predRelPath string, src predecessorManifestSource) (*artifact.SpecFrontmatter, *Finding) {
	content, err := gitx.Show(in.Ctx, in.Root, src.commit, predRelPath)
	if err != nil {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("reading predecessor %s at %s %s: %v", predRef, src.label, src.commit, err)}
	}
	fm, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s frontmatter at %s does not split: %v", predRef, src.label, err)}
	}
	var predSpec artifact.SpecFrontmatter
	if err := artifact.DecodeStrict(fm, &predSpec); err != nil {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s frontmatter at %s does not decode: %v", predRef, src.label, err)}
	}
	return &predSpec, nil
}

// resolveStatuslessPredecessorManifest is VL-015's merge-signaled path: when
// the predecessor carries no frozen: stamp (the ratified merge-signaled
// lifecycle's statusless shape — docs/superpowers/specs/2026-08-01-merge-
// signals-spec-acceptance-design.md — has none by design, not by omission),
// its object manifest is read from the Git-derived accepted baseline the
// shared specstate projector proves, never from any legacy stamp (there is
// none) and never from working-tree bytes. pred's own current bytes are
// consumed ONLY as Candidate.Content — the projector's exact-byte proof
// that pred.RelPath on the default branch really is pred's own reviewed
// revision — never as the manifest source itself, which always comes from
// git history at the proven landing commit. Resolved through in.Projector
// (engine.go's SpecStateResolver), the SAME consumption pattern vl004.go's
// legacy-draft compatibility disclosure uses — no second reachability or
// default-branch resolver is built here.
//
// A baseline is accepted ONLY when ALL of: Resolve returns no error;
// result.State is one of AcceptedPendingBuild, Superseded, or Closed (the
// three states specstate.Result's own doc comment identifies as carrying a
// fully proven baseline — Superseded matters here specifically because once
// a successor lands, the predecessor itself projects Superseded, and
// lint-store on main must stay green reading exactly that predecessor's own
// manifest); and result.Baseline is non-nil with Path, Blob, and
// LandingCommit all non-empty (engine.go's SpecStateResolver doc comment:
// real git always self-proves its own landing once a terminal state is
// reached, so an incomplete Baseline alongside a proven State is a shape
// only a test fake can force — checked and failed closed anyway, never
// assumed impossible); and that Baseline.Path is the predecessor's OWN
// path, since a baseline for any other path is no witness for the bytes
// this rule then reads. Anything else — a proposed (new or diverged)
// predecessor, an unproven result, or an operational Resolve/ReadFile
// failure — fails VL-015 closed with a finding naming the observed
// state/relation and carrying every specstate.Disclosures entry verbatim
// (joined "; "), so the failure stays honest about the missing witness
// (three-valued honesty) instead of a bare "no baseline."
func resolveStatuslessPredecessorManifest(in *RunInput, d *Document, predRef string, pred *Document) (*artifact.SpecFrontmatter, *Finding) {
	content, err := os.ReadFile(pred.Path)
	if err != nil {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s has no frozen stamp, and reading its own bytes to resolve a Git-derived accepted baseline failed: %v", predRef, err)}
	}

	result, err := in.Projector.Resolve(in.Ctx, in.Root, specstate.Candidate{Path: pred.RelPath, Content: content})
	if err != nil {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s has no frozen stamp; resolving its Git-derived accepted baseline: %v", predRef, err)}
	}

	switch result.State {
	case specstate.AcceptedPendingBuild, specstate.Superseded, specstate.Closed:
		// These three carry a fully proven baseline (see doc comment
		// above) — proceed to the completeness check below.
	default:
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: statuslessBaselineMessage(predRef, "not a proven accepted baseline (accepted-pending-build/superseded/closed required)", result)}
	}

	if result.Baseline == nil || result.Baseline.Path == "" || result.Baseline.Blob == "" || result.Baseline.LandingCommit == "" {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: statuslessBaselineMessage(predRef, "carries an incomplete accepted-baseline identity (path/blob/landing commit required)", result)}
	}

	// The baseline is only a witness FOR THIS PREDECESSOR if it is a
	// baseline OF this predecessor's own path: the manifest is read at
	// pred.RelPath, so a baseline proving some other spec's landing would
	// silently authorize reading pred.RelPath at a commit nothing proved it
	// landed in. Resolve is asked about Candidate{Path: pred.RelPath} and
	// so always answers about that path — which is exactly why this is
	// checked rather than assumed: an identity the rule depends on is
	// proven here, not left to the resolver's good behavior.
	if result.Baseline.Path != pred.RelPath {
		return nil, &Finding{Rule: "VL-015", Path: d.RelPath, Message: statuslessBaselineMessage(predRef, fmt.Sprintf(
			"resolved an accepted baseline for a DIFFERENT spec path (baseline path %s, predecessor path %s) — the manifest must be read at the predecessor's own path",
			result.Baseline.Path, pred.RelPath), result)}
	}

	// pred.RelPath, not result.Baseline.Path: they are now PROVEN equal by
	// the assertion above, and pred.RelPath reads as what it is — the
	// document whose manifest this rule is checking.
	return readPredecessorManifestAt(in, d, predRef, pred.RelPath, predecessorManifestSource{
		commit: result.Baseline.LandingCommit,
		label:  "its Git-derived accepted baseline's landing commit",
	})
}

// statuslessBaselineMessage names, for a frozen-stamp-less predecessor,
// what was observed (its Git-derived state and relation) and why that
// falls short of resolveStatuslessPredecessorManifest's acceptance
// condition (why), then appends every specstate.Disclosures entry verbatim
// (joined "; ") when non-empty — the failure names the missing witness
// specstate itself already produced, rather than re-deriving a vaguer one.
//
// A zero-value Relation renders as "unknown" rather than as an empty
// "(relation )": specstate always sets one, so this is a fake-reachable
// shape only, but a finding a human reads must never be ragged about what
// it did and did not observe.
func statuslessBaselineMessage(predRef, why string, result specstate.Result) string {
	relation := string(result.Relation)
	if relation == "" {
		relation = "unknown"
	}
	msg := fmt.Sprintf("predecessor %s has no frozen stamp and its Git-derived state is %s (relation %s), %s", predRef, result.State, relation, why)
	if len(result.Disclosures) > 0 {
		msg += ": " + strings.Join(result.Disclosures, "; ")
	}
	return msg
}

// objectEntry is one frontmatter-declared object's cross-revision identity
// tuple minus id (02 §Object model, the I-37 identity): its block kind and
// text.
type objectEntry struct {
	Kind artifact.ObjectKind
	Text string
}

// specObjects indexes every frontmatter-declared object on spec (across
// all four object blocks) by id.
func specObjects(spec *artifact.SpecFrontmatter) map[string]objectEntry {
	m := make(map[string]objectEntry, len(spec.AcceptanceCriteria)+len(spec.Constraints)+len(spec.Decisions)+len(spec.OpenQuestions))
	for _, ac := range spec.AcceptanceCriteria {
		m[ac.ID] = objectEntry{artifact.ObjectKindAcceptanceCriterion, ac.Text}
	}
	for _, c := range spec.Constraints {
		m[c.ID] = objectEntry{artifact.ObjectKindConstraint, c.Text}
	}
	for _, dc := range spec.Decisions {
		m[dc.ID] = objectEntry{artifact.ObjectKindDecision, dc.Text}
	}
	for _, q := range spec.OpenQuestions {
		m[q.ID] = objectEntry{artifact.ObjectKindOpenQuestion, q.Text}
	}
	return m
}
