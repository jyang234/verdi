// Rung-4 blast-radius-priced quorum (03 §Lifecycle: the feature-first
// cascade, §Ceremony pricing; 03 §The amendment ladder rung 4): "Quorum is
// blast-radius-priced: the cascade fold below computes the set of affected
// in-flight or closed stories; zero affected → single-owner acceptance;
// otherwise the full two-Code-Owner quorum." R4-I-6 keeps this computed
// INSIDE an existing verb rather than inventing a new one — `verdi accept`
// is the natural home: rung-4's "supersession MR" is exactly the
// superseding feature spec's acceptance MR (03 §Lifecycle step 1:
// "Merging the feature spec's MR to main *is* acceptance"), so this
// disclosure fires exactly when `verdi accept` flips a feature spec that
// itself carries a `supersession:` block (i.e. it is a rung-4 v2 revision,
// not an ordinary first acceptance).
//
// verdi never counts or enforces approval numbers (03: "the mechanics of
// counting approvals stay repo/CODEOWNERS configuration either way") — this
// file computes and PRINTS the quorum label only; nothing here blocks
// acceptance or checks a real approval count.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
)

// QuorumSingleOwner and QuorumTwoCodeOwner are the two blast-radius-priced
// quorum labels (03 §The amendment ladder rung 4).
const (
	QuorumSingleOwner  = "single-owner"
	QuorumTwoCodeOwner = "two-code-owner"
)

// BlastRadiusResult is computeBlastRadius's output: the affected in-flight
// or closed stories and the resulting quorum label.
type BlastRadiusResult struct {
	// PredecessorRef is the feature ref v2 supersedes (e.g. "spec/loan-mgmt"),
	// or "" if v2 carries no whole-spec supersedes link at all (not a rung-4
	// acceptance).
	PredecessorRef string
	Affected       []evidence.CascadeResult
	Quorum         string
}

// computeBlastRadius scans specs/active/ for every story spec whose
// implements edges target the feature v2 supersedes, folds each against
// v2's Supersession block (evidence.FoldCascade), and labels the quorum:
// zero affected in-flight-or-closed stories → single-owner; otherwise
// two-code-owner. Returns a zero-value (PredecessorRef == "") result, not
// an error, when v2 carries no whole-spec supersedes link — the ordinary
// "first acceptance, not a rung-4 revision" case.
func computeBlastRadius(root string, v2 *artifact.SpecFrontmatter) (BlastRadiusResult, error) {
	predecessorRef := wholeSpecSupersedesTarget(v2)
	if predecessorRef == "" || v2.Supersession == nil {
		return BlastRadiusResult{}, nil
	}
	predRef, err := artifact.ParseRef(predecessorRef)
	if err != nil {
		return BlastRadiusResult{}, fmt.Errorf("blast radius: predecessor ref %q: %w", predecessorRef, err)
	}

	dir := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return BlastRadiusResult{PredecessorRef: predecessorRef, Quorum: QuorumSingleOwner}, nil
		}
		return BlastRadiusResult{}, fmt.Errorf("blast radius: listing %s: %w", dir, err)
	}

	var stories []evidence.CascadeStory
	statusByRef := map[string]artifact.Status{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		spec, err := loadActiveSpecTolerant(root, e.Name())
		if err != nil || spec == nil {
			continue
		}
		if spec.Class != artifact.ClassStory {
			continue
		}
		byFeature := evidence.ImplementsByFeature(spec.Links)
		objectIDs, ok := byFeature[predRef.Name]
		if !ok {
			continue
		}
		stories = append(stories, evidence.CascadeStory{SpecRef: spec.ID, ObjectIDs: objectIDs})
		statusByRef[spec.ID] = spec.Status
	}
	sort.Slice(stories, func(i, j int) bool { return stories[i].SpecRef < stories[j].SpecRef })

	results, err := evidence.FoldCascade(*v2.Supersession, stories)
	if err != nil {
		return BlastRadiusResult{}, fmt.Errorf("blast radius: folding cascade: %w", err)
	}

	var affected []evidence.CascadeResult
	for _, r := range results {
		if r.Verdict == evidence.CascadeUnaffected {
			continue
		}
		// "affected in-flight or closed stories" (03): a still-draft story
		// has no frozen mapping into the predecessor yet, so it is not
		// counted toward the blast radius — it will simply be authored
		// against the new feature revision.
		status := statusByRef[r.SpecRef]
		if status == "accepted-pending-build" || status == "closed" {
			affected = append(affected, r)
		}
	}

	quorum := QuorumSingleOwner
	if len(affected) > 0 {
		quorum = QuorumTwoCodeOwner
	}
	return BlastRadiusResult{PredecessorRef: predecessorRef, Affected: affected, Quorum: quorum}, nil
}

// wholeSpecSupersedesTarget returns the ref named by v2's top-level
// `supersedes` link, if any — a WHOLE-spec supersession edge (no object
// fragment; R4-I-14: "a superseding spec is a NEW ref ... the supersedes
// link carries the chain"), as opposed to a decision-level or story-level
// supersedes edge targeting an object fragment (03 §Decision-conflict
// gate's rung-2 machinery, a different mechanism entirely).
func wholeSpecSupersedesTarget(v2 *artifact.SpecFrontmatter) string {
	for _, l := range v2.Links {
		if l.Type != artifact.LinkSupersedes {
			continue
		}
		ref, err := artifact.ParseRef(l.Ref)
		if err != nil || ref.Fragment() {
			continue
		}
		return l.Ref
	}
	return ""
}
