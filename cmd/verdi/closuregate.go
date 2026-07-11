// The CLOSURE gate (V1-P4, 03 §Gates' "closure gate"): "the story may
// close only when eligible is true, and no unresolved spec-stale or
// pending-supersession flag is present on its edges." Distinct from the
// merge gate above (gate.go): 03 is explicit these two flags "block
// closure, not merge — builds keep moving," so they are NOT folded into
// runGate's condition list. `verdi close` (the verb that would dispatch a
// closure-MR run of these conditions) stays out of this phase's scope —
// see gate.go's doc comment for why this function is deliberately built
// self-contained and unwired rather than invented onto a CLI surface this
// phase does not own.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/evidence"
	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/store"
)

// runClosureGate evaluates 03 §Gates' closure gate for spec at head:
// eligible (the story-level fold, authoritative evidence only), no
// unresolved spec-stale flag, and no unresolved pending-supersession flag.
// f may be nil (no forge configured / unreachable, or no network in tests).
// When the story implements a feature whose open supersession MRs cannot be
// enumerated because f is nil, the pending-supersession condition is
// reported disclosed-unproven — a printed [NOTICE], never a silent pass
// (constitution 2/10: silence is never a pass) — rather than being read as
// "no pending MRs exist". Only a story that implements no feature at all
// (nothing to prove) passes that condition outright with a nil forge.
func runClosureGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, f forge.Forge, defaultBranchRef string, manifest *store.Manifest, head string, stdout io.Writer) (bool, error) {
	cond1, err := checkClosureEligible(ctx, root, spec, head)
	if err != nil {
		return false, err
	}
	cond2, err := checkSpecStaleCondition(root, spec, manifest)
	if err != nil {
		return false, err
	}
	cond3, err := checkPendingSupersessionCondition(ctx, f, defaultBranchRef, spec)
	if err != nil {
		return false, err
	}

	allOK := true
	for _, c := range []gateCondition{cond1, cond2, cond3} {
		switch {
		case c.Disclosed:
			// Three-valued honesty (constitution 2/10): the input was
			// unavailable, so this is neither a pass nor a fail — a printed
			// notice that leaves the gate verdict to the other conditions
			// (mirrors VL-017's disclosure mechanism).
			fmt.Fprintf(stdout, "[NOTICE] closure: %s\n", c.Name)
			fmt.Fprintf(stdout, "       %s\n", c.Reason)
		case c.OK:
			fmt.Fprintf(stdout, "[PASS] closure: %s\n", c.Name)
		default:
			allOK = false
			fmt.Fprintf(stdout, "[FAIL] closure: %s\n", c.Name)
			fmt.Fprintf(stdout, "       %s\n", c.Reason)
		}
	}
	return allOK, nil
}

// checkClosureEligible is the closure gate's "eligible is true" condition:
// the same story-level fold checkNoACViolated (gate.go) uses, checked for
// full eligibility (every AC evidenced or waived) rather than merely
// "not violated".
func checkClosureEligible(ctx context.Context, root string, spec *artifact.SpecFrontmatter, head string) (gateCondition, error) {
	name := "1. story eligible (every AC evidenced or waived, authoritative evidence)"

	derivedRoot := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, head)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: loading evidence records: %w", err)
	}
	slug := store.RefSlug(spec.Story)
	result, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: false, StoreRoot: root, StorySlug: slug})
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: folding evidence: %w", err)
	}
	if result.Eligible {
		return gateCondition{Name: name, OK: true}, nil
	}
	return gateCondition{Name: name, Reason: "story is not eligible (not every AC is evidenced or waived)"}, nil
}

// checkSpecStaleCondition is the closure gate's spec-stale condition
// (03 §The amendment ladder's rung-arbitrage counter-pressure): blocks
// while SpecStale is Flagged. The story's deviation report (frozen or
// living — closure reads whichever is on disk, mirroring gate condition
// 3's own read) supplies Findings; an absent report has no
// accepted-deviation dispositions to flag at all, so it is read as
// trivially unflagged, not as an error (a story with no build activity yet
// cannot be spec-stale).
func checkSpecStaleCondition(root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest) (gateCondition, error) {
	name := "2. no unresolved spec-stale flag"

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: internal error: resolved spec has an invalid id: %w", err)
	}
	path := filepath.Join(root, ".verdi", "specs", "active", specRef.Name, "deviation-report.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return gateCondition{Name: name, OK: true}, nil
		}
		return gateCondition{}, fmt.Errorf("closure gate: reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %s: %w", path, err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %s failed to decode: %w", path, err)
	}

	storyACIDs := make(map[string]bool, len(spec.AcceptanceCriteria))
	for _, ac := range spec.AcceptanceCriteria {
		storyACIDs[ac.ID] = true
	}
	threshold := 0
	if manifest != nil && manifest.Audit != nil {
		threshold = manifest.Audit.DeviationsStaleThreshold
	}

	result := evidence.SpecStale(evidence.SpecStaleInput{Findings: decoded.Findings, StoryACIDs: storyACIDs, Threshold: threshold})
	if !result.Flagged {
		return gateCondition{Name: name, OK: true}, nil
	}
	return gateCondition{Name: name, Reason: fmt.Sprintf("spec-stale: own-text finding(s) %v, accepted-deviation count %d (threshold %d)", result.OwnTextFindingIDs, result.AcceptedDeviationCount, threshold)}, nil
}

// checkPendingSupersessionCondition is the closure gate's
// pending-supersession condition (03 §The amendment ladder: "the fold's
// input set includes open supersession MRs"): for every feature the story
// implements, probes for an open MR carrying a candidate v2 spec
// (R4-I-14's naming convention, <name>-v2, mirroring V1-P3's own
// evidence.LoadPendingSupersessionCandidates caller contract) and folds
// the story's touched object ids against it.
func checkPendingSupersessionCondition(ctx context.Context, f forge.Forge, defaultBranchRef string, spec *artifact.SpecFrontmatter) (gateCondition, error) {
	name := "3. no unresolved pending-supersession flag"

	byFeature := evidence.ImplementsByFeature(spec.Links)
	if len(byFeature) == 0 {
		// The story implements no feature — there is no open-supersession
		// input to fold at all, so the condition is genuinely satisfied.
		return gateCondition{Name: name, OK: true}, nil
	}
	if f == nil {
		// The story implements a feature, but no forge is configured or
		// reachable, so open supersession MRs cannot be enumerated. Disclose
		// the check unproven rather than reading the missing input as
		// "no pending MRs" (constitution 2/10: silence is never a pass).
		return gateCondition{
			Name:      name,
			Disclosed: true,
			Reason:    "disclosed-unproven: no forge configured/reachable, so open supersession MRs cannot be enumerated (not read as 'no pending MRs' — constitution 2/10)",
		}, nil
	}

	featureNames := make([]string, 0, len(byFeature))
	for n := range byFeature {
		featureNames = append(featureNames, n)
	}
	sort.Strings(featureNames)

	var touched, mrIDs []string
	for _, featureName := range featureNames {
		candidatePath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", featureName+"-v2", "spec.md"))
		candidates, err := evidence.LoadPendingSupersessionCandidates(ctx, f, defaultBranchRef, "spec/"+featureName, candidatePath)
		if err != nil {
			return gateCondition{}, fmt.Errorf("closure gate: loading pending-supersession candidates for %s: %w", featureName, err)
		}
		result := evidence.PendingSupersession(evidence.PendingSupersessionInput{ObjectIDs: byFeature[featureName], Candidates: candidates})
		if result.Flagged {
			touched = append(touched, result.Touched...)
			mrIDs = append(mrIDs, result.MRIDs...)
		}
	}
	if len(touched) == 0 {
		return gateCondition{Name: name, OK: true}, nil
	}
	sort.Strings(touched)
	sort.Strings(mrIDs)
	return gateCondition{Name: name, Reason: fmt.Sprintf("open supersession MR(s) %v touch object(s) %v", mrIDs, touched)}, nil
}
